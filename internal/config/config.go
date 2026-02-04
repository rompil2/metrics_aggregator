// Package config provides configuration management for the metrics aggregator server.
// path: internal/config
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rompil2/metrics_aggregator/internal/logger"
)

var log = logger.Get()

const (
	defaultHost            = "localhost"
	defaultPort            = 8080
	defaultPollInterval    = 2
	defaultReportInterval  = 10
	defaultStoreInterval   = 300
	defaultFileStoragePath = "./storage.txt"
	defaultRestore         = false
	emptyString            = ""
)

// SocketConfig represents a network address configuration with host and port.
type SocketConfig struct {
	Host string
	Port uint
}

func (s *SocketConfig) String() string {
	return net.JoinHostPort(s.Host, strconv.FormatUint(uint64(s.Port), 10))
}

func (s *SocketConfig) Set(flagVal string) error {
	host, portStr, err := net.SplitHostPort(flagVal)
	if err != nil {
		return fmt.Errorf("invalid address format: %w", err)
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return errors.New("port should be a valid number")
	}
	if port > 65535 {
		return errors.New("port should not be greater than 65535")
	}

	s.Host = host
	s.Port = uint(port)
	return nil
}

// HashConfig holds the secret key used for HMAC-based request/response integrity verification.
type HashConfig struct {
	Key string
}

func (h *HashConfig) String() string {
	return h.Key
}

func (h *HashConfig) Set(flagVal string) error {
	h.Key = flagVal
	return nil
}

// Audit represents an audit sink configuration (either file path or URL).
type Audit struct {
	auditSink string
}

func (a *Audit) String() string {
	return a.auditSink
}

func (a *Audit) Set(flagVal string) error {
	a.auditSink = flagVal
	return nil
}

// AuditConfig groups audit logging destinations for file and HTTP endpoints.
type AuditConfig struct {
	AuditFile Audit
	AuditURL  Audit
}

// StoreConfig defines persistent storage settings for metrics data,
// including file path, restore behavior, and database connection string.
type StoreConfig struct {
	FileStoragePath string
	DBConnStr       string
	StoreInterval   time.Duration
	Restore         bool
}

// ServerConfig combines all configuration parameters required to run the metrics server,
// including network, storage, security (hashing), and auditing options.
type ServerConfig struct {
	AuditConfig
	HashConfig
	SocketConfig
	StoreConfig
	PrivateKeyPath string
}

// ServerConfigJSON represents the JSON format for server config.
type ServerConfigJSON struct {
	Address       string `json:"address"`
	Restore       bool   `json:"restore"`
	StoreInterval string `json:"store_interval"`
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
}

// AgentConfig contains all settings needed for the metrics collection agent,
// including server address, polling/reporting intervals, rate limiting, and hashing key.
type AgentConfig struct {
	HashConfig
	SocketConfig
	PollInterval   time.Duration
	ReportInterval time.Duration
	RateLimit      uint
	PublicKeyPath  string
}

// AgentConfigJSON represents the JSON format for agent config.
type AgentConfigJSON struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	CryptoKey      string `json:"crypto_key"`
}

// LoadServerConfigFromFile loads server config from a JSON file.
func LoadServerConfigFromFile(filename string) (*ServerConfigJSON, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config ServerConfigJSON
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadAgentConfigFromFile loads agent config from a JSON file.
func LoadAgentConfigFromFile(filename string) (*AgentConfigJSON, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config AgentConfigJSON
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadServerConfig parses command-line flags and environment variables to build a ServerConfig.
// It supports both CLI flags (e.g., -a, -k, -f) and corresponding environment variables (e.g., ADDRESS, KEY).
// The priority order is: env > flags > config file.
func LoadServerConfig(args []string) ServerConfig {
	flagSet := flag.NewFlagSet("server", flag.ContinueOnError)

	configFile := ""
	socket := SocketConfig{
		Host: defaultHost,
		Port: defaultPort,
	}
	hashKey := HashConfig{
		Key: emptyString,
	}
	auditFile := Audit{}
	auditURL := Audit{}

	// Default values from config file (lowest priority)
	var loadedConfig *ServerConfigJSON

	// Check env var for config file first
	envConfigFile := os.Getenv("CONFIG")
	if envConfigFile != "" {
		cfg, err := LoadServerConfigFromFile(envConfigFile)
		if err != nil {
			log.Warn().Str("config_file", envConfigFile).Err(err).Msg("Failed to load config from file")
		} else {
			loadedConfig = cfg
		}
	}

	// Then check -c/-config flag
	flagSet.StringVar(&configFile, "c", "", "Path to config file")
	flagSet.StringVar(&configFile, "config", "", "Path to config file")
	flagSet.Var(&socket, "a", "-a=<host>:<port>")
	flagSet.Var(&hashKey, "k", "-k=<key_for_hash>")
	storeInterval := flagSet.Uint("i", defaultStoreInterval, "storing interval in seconds")
	fileStoragePath := flagSet.String("f", defaultFileStoragePath, "path to a file to store data")
	restore := flagSet.Bool("r", defaultRestore, "should restore data")
	database := flagSet.String("d", "", "A DB connection string")
	flagSet.Var(&auditFile, "audit-file", "--audit-file=<path to an audit log file>")
	flagSet.Var(&auditURL, "audit-url", "--audit-url=<URL to an audit log service>")
	privateKeyPath := flagSet.String("crypto-key", "", "Path to the private key for decryption")

	// Parse flags (these have medium priority)
	if err := flagSet.Parse(args); err != nil {
		log.Error().Err(err).Msg("Error parsing flags")
	}

	// Now load config file from flag if provided (overrides defaults but not env/flags)
	if configFile != "" && loadedConfig == nil {
		cfg, err := LoadServerConfigFromFile(configFile)
		if err != nil {
			log.Warn().Str("config_file", configFile).Err(err).Msg("Failed to load config from file")
		} else {
			loadedConfig = cfg
		}
	}

	// Apply defaults from config file first
	if loadedConfig != nil {
		if socket.Host == defaultHost && socket.Port == defaultPort {
			if loadedConfig.Address != "" {
				socket.Set(loadedConfig.Address)
			}
		}
		if *storeInterval == defaultStoreInterval {
			if loadedConfig.StoreInterval != "" {
				if dur, err := time.ParseDuration(loadedConfig.StoreInterval); err == nil {
					*storeInterval = uint(dur.Seconds())
				}
			}
		}
		if *fileStoragePath == defaultFileStoragePath {
			if loadedConfig.StoreFile != "" {
				*fileStoragePath = loadedConfig.StoreFile
			}
		}
		//nolint:gosimple
		if *restore == defaultRestore {
			*restore = loadedConfig.Restore
		}
		if *database == "" {
			if loadedConfig.DatabaseDSN != "" {
				*database = loadedConfig.DatabaseDSN
			}
		}
		if *privateKeyPath == "" {
			if loadedConfig.CryptoKey != "" {
				*privateKeyPath = loadedConfig.CryptoKey
			}
		}
	}

	// Now apply environment variables (highest priority)
	if val, ok := os.LookupEnv("ADDRESS"); ok {
		socket.Set(val)
		log.Info().Str("ADDRESS", val).Send()
	}

	if val, ok := os.LookupEnv("STORE_INTERVAL"); ok {
		if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
			*storeInterval = uint(parsed)
			log.Info().Str("STORE_INTERVAL", val).Send()
		}
	}

	if val, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		*fileStoragePath = val
		log.Info().Str("FILE_STORAGE_PATH", val).Send()
	}

	if val, ok := os.LookupEnv("RESTORE"); ok {
		*restore = strings.ToLower(val) == "true"
		log.Info().Bool("RESTORE", *restore).Send()
	}

	if val, ok := os.LookupEnv("DATABASE_DSN"); ok {
		*database = val
		log.Info().Str("DATABASE_DSN", *database).Send()
	}
	if val, ok := os.LookupEnv("KEY"); ok {
		hashKey.Set(val)
		log.Info().Str("KEY", val).Send()
	}
	if val, ok := os.LookupEnv("AUDIT_FILE"); ok {
		auditFile.Set(val)
		log.Info().Str("AUDIT_FILE", val).Send()
	}
	if val, ok := os.LookupEnv("AUDIT_URL"); ok {
		auditURL.Set(val)
		log.Info().Str("AUDIT_URL", val).Send()
	}
	if val, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		*privateKeyPath = val
	}

	return ServerConfig{
		SocketConfig: socket,
		StoreConfig: StoreConfig{
			StoreInterval:   time.Duration(*storeInterval) * time.Second,
			FileStoragePath: *fileStoragePath,
			Restore:         *restore,
			DBConnStr:       *database,
		},
		HashConfig: hashKey,
		AuditConfig: AuditConfig{
			AuditFile: auditFile,
			AuditURL:  auditURL,
		},
		PrivateKeyPath: *privateKeyPath,
	}
}

// LoadAgentConfig parses command-line arguments and environment variables to construct an AgentConfig.
// It supports flags like -a (address), -k (hash key), -p (poll interval), -r (report interval),
// and corresponding environment variables (ADDRESS, KEY, POLL_INTERVAL, etc.).
// The priority order is: env > flags > config file.
func LoadAgentConfig(args []string) AgentConfig {
	flagSet := flag.NewFlagSet("agent", flag.ContinueOnError)

	configFile := ""
	socket := SocketConfig{
		Host: defaultHost,
		Port: defaultPort,
	}
	hashKey := HashConfig{
		Key: emptyString,
	}

	// Default values from config file (lowest priority)
	var loadedConfig *AgentConfigJSON

	// Check env var for config file first
	envConfigFile := os.Getenv("CONFIG")
	if envConfigFile != "" {
		cfg, err := LoadAgentConfigFromFile(envConfigFile)
		if err != nil {
			log.Warn().Str("config_file", envConfigFile).Err(err).Msg("Failed to load config from file")
		} else {
			loadedConfig = cfg
		}
	}

	// Then check -c/-config flag
	flagSet.StringVar(&configFile, "c", "", "Path to config file")
	flagSet.StringVar(&configFile, "config", "", "Path to config file")
	flagSet.Var(&socket, "a", "-a=<host>:<port>")
	flagSet.Var(&hashKey, "k", "-k=<key_for_hash>")

	pollInterval := flagSet.Uint("p", defaultPollInterval, "polling interval in seconds")
	reportInterval := flagSet.Uint("r", defaultReportInterval, "report interval in seconds")
	rateLimit := flagSet.Uint("l", 1, "rate limit for agent")
	publicKeyPath := flagSet.String("crypto-key", "", "Path to the public key for encryption")

	// Parse flags (medium priority)
	if err := flagSet.Parse(args); err != nil {
		log.Error().Err(err).Msg("Error parsing flags")
	}

	// Now load config file from flag if provided
	if configFile != "" && loadedConfig == nil {
		cfg, err := LoadAgentConfigFromFile(configFile)
		if err != nil {
			log.Warn().Str("config_file", configFile).Err(err).Msg("Failed to load config from file")
		} else {
			loadedConfig = cfg
		}
	}

	// Apply defaults from config file first
	if loadedConfig != nil {
		if socket.Host == defaultHost && socket.Port == defaultPort {
			if loadedConfig.Address != "" {
				socket.Set(loadedConfig.Address)
			}
		}
		if *reportInterval == defaultReportInterval {
			if loadedConfig.ReportInterval != "" {
				if dur, err := time.ParseDuration(loadedConfig.ReportInterval); err == nil {
					*reportInterval = uint(dur.Seconds())
				}
			}
		}
		if *pollInterval == defaultPollInterval {
			if loadedConfig.PollInterval != "" {
				if dur, err := time.ParseDuration(loadedConfig.PollInterval); err == nil {
					*pollInterval = uint(dur.Seconds())
				}
			}
		}
		if *publicKeyPath == "" {
			if loadedConfig.CryptoKey != "" {
				*publicKeyPath = loadedConfig.CryptoKey
			}
		}
	}

	// Now apply environment variables (highest priority)
	if val, ok := os.LookupEnv("ADDRESS"); ok {
		socket.Set(val)
		log.Info().Str("ADDRESS", val).Send()
	}

	if val, ok := os.LookupEnv("POLL_INTERVAL"); ok {
		if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
			*pollInterval = uint(parsed)
		}
	}

	if val, ok := os.LookupEnv("REPORT_INTERVAL"); ok {
		if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
			*reportInterval = uint(parsed)
		}
	}
	if val, ok := os.LookupEnv("KEY"); ok {
		hashKey.Set(val)
		log.Info().Str("KEY", val).Send()
	}

	if val, ok := os.LookupEnv("RATE_LIMIT"); ok {
		if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
			*rateLimit = uint(parsed)
		}
	}

	if val, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		*publicKeyPath = val
	}

	return AgentConfig{
		SocketConfig:   socket,
		PollInterval:   time.Duration(*pollInterval) * time.Second,
		ReportInterval: time.Duration(*reportInterval) * time.Second,
		HashConfig:     hashKey,
		RateLimit:      *rateLimit,
		PublicKeyPath:  *publicKeyPath,
	}
}
