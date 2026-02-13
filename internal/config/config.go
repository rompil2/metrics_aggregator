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
	defaultHost                 = "localhost"
	defaultPort                 = 8080
	defaultPollInterval    uint = 2
	defaultReportInterval  uint = 10
	defaultStoreInterval   uint = 300
	defaultFileStoragePath      = "./storage.txt"
	defaultRestore              = false
	defaultRateLimit            = 1
	emptyString                 = ""
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
	Restore         *bool
}

// ServerConfig combines all configuration parameters required to run the metrics server,
// including network, storage, security (hashing), and auditing options.
type ServerConfig struct {
	AuditConfig
	HashConfig
	SocketConfig
	StoreConfig
	PrivateKeyPath string
	TrustedSubnet  string
	GRPCAddr       string
}

// ServerConfigJSON represents the JSON format for server config.
type ServerConfigJSON struct {
	Address       string `json:"address"`
	Restore       bool   `json:"restore"`
	StoreInterval string `json:"store_interval"`
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
	TrustedSubnet string `json:"trusted_subnet"`
	GRPCAddr      string `json:"grpc_address"`
}
type agentConfigValues struct {
	SocketConfig
	HashConfig
	PollInterval   uint
	ReportInterval uint
	RateLimit      uint
	PublicKeyPath  string
	GRPCAddr       string
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
	GRPCAddr       string
}

// AgentConfigJSON represents the JSON format for agent config.
type AgentConfigJSON struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	CryptoKey      string `json:"crypto_key"`
	GRPCAddr       string `json:"grpc_address"`
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

// ConfigLoader manages loading configuration from multiple sources.
type ConfigLoader struct {
	args []string
}

// NewConfigLoader creates a new ConfigLoader instance with the given command-line arguments.
func NewConfigLoader(args []string) *ConfigLoader {
	return &ConfigLoader{args: args}
}

// LoadServerConfig loads and merges server configuration from defaults, file, flags, and environment variables.
func (cl *ConfigLoader) LoadServerConfig() ServerConfig {
	defaults := cl.getDefaultServerConfig()
	fromFile := cl.loadServerConfigFromFile()
	fromFlags := cl.parseServerFlags()
	fromEnv := cl.getServerEnvConfig()

	merged := cl.mergeServerConfigs(defaults, fromFile, fromFlags, fromEnv)

	return ServerConfig{
		SocketConfig: merged.SocketConfig,
		StoreConfig: StoreConfig{
			StoreInterval:   merged.StoreInterval,
			FileStoragePath: merged.FileStoragePath,
			Restore:         merged.Restore,
			DBConnStr:       merged.DBConnStr,
		},
		HashConfig:     merged.HashConfig,
		AuditConfig:    merged.AuditConfig,
		PrivateKeyPath: merged.PrivateKeyPath,
		GRPCAddr:       merged.GRPCAddr,
	}
}

// getDefaultServerConfig returns the default server configuration values.
func (cl *ConfigLoader) getDefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		SocketConfig: SocketConfig{
			Host: defaultHost,
			Port: defaultPort,
		},
		HashConfig: HashConfig{Key: emptyString},
		StoreConfig: StoreConfig{
			StoreInterval:   time.Duration(defaultStoreInterval) * time.Second,
			FileStoragePath: defaultFileStoragePath,
			Restore:         nil,
		},
		AuditConfig: AuditConfig{
			AuditFile: Audit{},
			AuditURL:  Audit{},
		},
		PrivateKeyPath: emptyString,
		GRPCAddr:       emptyString,
	}
}

// loadServerConfigFromFile loads server configuration from a JSON file specified by environment variable or flag.
func (cl *ConfigLoader) loadServerConfigFromFile() *ServerConfig {
	envConfigFile := os.Getenv("CONFIG")
	if envConfigFile != emptyString {
		cfg, err := LoadServerConfigFromFile(envConfigFile)
		if err != nil {
			log.Warn().Str("config_file", envConfigFile).Err(err).Msg("Failed to load config from file")
			return nil
		}
		return cl.serverConfigJSONToValues(cfg)
	}

	// Also check -c/-config flag for config file path only
	tempFlagSet := flag.NewFlagSet(emptyString, flag.ContinueOnError)
	configFile := tempFlagSet.String("c", emptyString, "Path to config file")
	tempFlagSet.StringVar(configFile, "config", emptyString, "Path to config file")
	tempFlagSet.Parse(cl.args)

	if *configFile != emptyString {
		cfg, err := LoadServerConfigFromFile(*configFile)
		if err != nil {
			log.Warn().Str("config_file", *configFile).Err(err).Msg("Failed to load config from file")
			return nil
		}
		return cl.serverConfigJSONToValues(cfg)
	}

	return nil
}

// serverConfigJSONToValues converts a ServerConfigJSON to ServerConfig.
func (cl *ConfigLoader) serverConfigJSONToValues(cfg *ServerConfigJSON) *ServerConfig {
	if cfg == nil {
		return nil
	}

	storeInterval := defaultStoreInterval
	if cfg.StoreInterval != emptyString {
		if dur, err := strconv.ParseUint(cfg.StoreInterval, 10, 32); err == nil {
			storeInterval = uint(dur)
		}
	}

	return &ServerConfig{
		SocketConfig: SocketConfig{
			Host: defaultHost,
			Port: defaultPort,
		},
		StoreConfig: StoreConfig{
			StoreInterval:   time.Duration(storeInterval) * time.Second,
			FileStoragePath: cfg.StoreFile,
			Restore:         &(cfg.Restore),
			DBConnStr:       cfg.DatabaseDSN,
		},
		AuditConfig: AuditConfig{
			AuditFile: Audit{auditSink: emptyString},
			AuditURL:  Audit{auditSink: emptyString},
		},
		PrivateKeyPath: cfg.CryptoKey,
		TrustedSubnet:  cfg.TrustedSubnet,
		GRPCAddr:       cfg.GRPCAddr,
	}
}

// parseServerFlags parses command-line flags for server configuration.
func (cl *ConfigLoader) parseServerFlags() *ServerConfig {
	flagSet := flag.NewFlagSet("server", flag.ContinueOnError)

	configFile := emptyString
	socket := SocketConfig{
		Host: defaultHost,
		Port: defaultPort,
	}
	hashKey := HashConfig{Key: emptyString}
	auditFile := Audit{}
	auditURL := Audit{}

	storeInterval := flagSet.Uint("i", defaultStoreInterval, "storing interval in seconds")
	fileStoragePath := flagSet.String("f", defaultFileStoragePath, "path to a file to store data")
	restore := flagSet.Bool("r", defaultRestore, "should restore data")

	database := flagSet.String("d", emptyString, "A DB connection string")
	privateKeyPath := flagSet.String("crypto-key", emptyString, "Path to the private key for decryption")
	trustedSubnet := flagSet.String("t", "", "trusted subnet in CIDR format")

	flagSet.StringVar(&configFile, "c", emptyString, "Path to config file")
	flagSet.StringVar(&configFile, "config", emptyString, "Path to config file")
	flagSet.Var(&socket, "a", "-a=<host>:<port>")
	flagSet.Var(&hashKey, "k", "-k=<key_for_hash>")
	flagSet.Var(&auditFile, "audit-file", "--audit-file=<path to an audit log file>")
	flagSet.Var(&auditURL, "audit-url", "--audit-url=<URL to an audit log service>")
	grpcAddr := flagSet.String("grpc-addr", emptyString, "gRPC server address")

	if err := flagSet.Parse(cl.args); err != nil {
		log.Error().Err(err).Msg("Error parsing flags")
	}
	if !*restore {
		restore = nil
	}
	return &ServerConfig{
		SocketConfig: socket,
		HashConfig:   hashKey,
		StoreConfig: StoreConfig{
			StoreInterval:   time.Duration(*storeInterval) * time.Second,
			FileStoragePath: *fileStoragePath,
			Restore:         restore,
			DBConnStr:       *database,
		},
		AuditConfig:    AuditConfig{AuditFile: auditFile, AuditURL: auditURL},
		PrivateKeyPath: *privateKeyPath,
		TrustedSubnet:  *trustedSubnet,
		GRPCAddr:       *grpcAddr,
	}
}

// getServerEnvConfig loads server configuration from environment variables.
func (cl *ConfigLoader) getServerEnvConfig() *ServerConfig {
	defConfig := cl.getDefaultServerConfig()

	if val, ok := os.LookupEnv("ADDRESS"); ok {
		defConfig.SocketConfig.Set(val)
		log.Info().Str("ADDRESS", val).Send()
	}

	if val, ok := os.LookupEnv("STORE_INTERVAL"); ok {
		if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
			defConfig.StoreInterval = time.Duration(uint(parsed)) * time.Second
			log.Info().Str("STORE_INTERVAL", val).Send()
		}
	}

	if val, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		defConfig.FileStoragePath = val
		log.Info().Str("FILE_STORAGE_PATH", val).Send()
	}

	if val, ok := os.LookupEnv("RESTORE"); ok {
		defConfig.Restore = new(bool)
		*(defConfig.Restore) = strings.ToLower(val) == "true"
		log.Info().Bool("RESTORE", *(defConfig.Restore)).Send()
	}

	if val, ok := os.LookupEnv("DATABASE_DSN"); ok {
		defConfig.DBConnStr = val
		log.Info().Str("DATABASE_DSN", defConfig.DBConnStr).Send()
	}

	if val, ok := os.LookupEnv("KEY"); ok {
		defConfig.HashConfig.Set(val)
		log.Info().Str("KEY", val).Send()
	}

	if val, ok := os.LookupEnv("AUDIT_FILE"); ok {
		defConfig.AuditFile.Set(val)
		log.Info().Str("AUDIT_FILE", val).Send()
	}

	if val, ok := os.LookupEnv("AUDIT_URL"); ok {
		defConfig.AuditURL.Set(val)
		log.Info().Str("AUDIT_URL", val).Send()
	}

	if val, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		defConfig.PrivateKeyPath = val
	}

	if val, ok := os.LookupEnv("TRUSTED_SUBNET"); ok {
		defConfig.TrustedSubnet = val
		log.Info().Str("TRUSTED_SUBNET", val).Send()
	}

	if val, ok := os.LookupEnv("GRPC_ADDR"); ok {
		defConfig.GRPCAddr = val
		log.Info().Str("GRPC_ADDR", val).Send()
	}

	return defConfig

}

// mergeServerConfigs merges server configuration from defaults, file, flags, and environment variables with priority.
func (cl *ConfigLoader) mergeServerConfigs(defaults, fromFile, fromFlags, fromEnv *ServerConfig) *ServerConfig {
	merged := *defaults

	// Apply from file if not default
	if fromFile != nil {
		if merged.SocketConfig.Host == defaultHost && merged.SocketConfig.Port == defaultPort {
			merged.SocketConfig = fromFile.SocketConfig
		}
		if merged.StoreConfig.StoreInterval.Seconds() == float64(defaultStoreInterval) {
			merged.StoreConfig.StoreInterval = fromFile.StoreConfig.StoreInterval
		}
		if merged.StoreConfig.FileStoragePath == defaultFileStoragePath {
			merged.StoreConfig.FileStoragePath = fromFile.StoreConfig.FileStoragePath
		}
		if merged.StoreConfig.Restore != nil {
			merged.StoreConfig.Restore = fromFile.StoreConfig.Restore
		}
		if merged.StoreConfig.DBConnStr == emptyString {
			merged.StoreConfig.DBConnStr = fromFile.StoreConfig.DBConnStr
		}
		if merged.PrivateKeyPath == emptyString {
			merged.PrivateKeyPath = fromFile.PrivateKeyPath
		}
		if merged.GRPCAddr == emptyString {
			merged.GRPCAddr = fromFile.GRPCAddr
		}

	}

	// Apply from flags if not default
	if fromFlags != nil {

		if fromFlags.SocketConfig.Host != defaultHost || fromFlags.SocketConfig.Port != defaultPort {
			merged.SocketConfig = fromFlags.SocketConfig
		}
		merged.HashConfig = fromFlags.HashConfig
		if fromFlags.StoreConfig.StoreInterval.Seconds() != float64(defaultStoreInterval) {
			merged.StoreConfig.StoreInterval = fromFlags.StoreConfig.StoreInterval
		}
		if fromFlags.StoreConfig.FileStoragePath != defaultFileStoragePath {
			merged.StoreConfig.FileStoragePath = fromFlags.StoreConfig.FileStoragePath
		}
		if fromFlags.StoreConfig.Restore != nil {
			merged.StoreConfig.Restore = fromFlags.StoreConfig.Restore
		}
		if fromFlags.StoreConfig.DBConnStr != emptyString {
			merged.StoreConfig.DBConnStr = fromFlags.StoreConfig.DBConnStr
		}
		merged.AuditConfig = fromFlags.AuditConfig
		if fromFlags.PrivateKeyPath != emptyString {
			merged.PrivateKeyPath = fromFlags.PrivateKeyPath
		}
		if fromFile.GRPCAddr != emptyString {
			merged.GRPCAddr = fromFlags.GRPCAddr
		}
	}

	// Apply from env (highest priority)
	if fromEnv != nil {
		if fromEnv.SocketConfig.Host != defaultHost || fromEnv.SocketConfig.Port != defaultPort {
			merged.SocketConfig = fromEnv.SocketConfig
		}
		if fromEnv.HashConfig.Key != emptyString {
			merged.HashConfig = fromEnv.HashConfig
		}
		if fromEnv.StoreConfig.StoreInterval.Seconds() != float64(defaultStoreInterval) {
			merged.StoreConfig.StoreInterval = fromEnv.StoreConfig.StoreInterval
		}
		if fromEnv.StoreConfig.FileStoragePath != defaultFileStoragePath {
			merged.StoreConfig.FileStoragePath = fromEnv.StoreConfig.FileStoragePath
		}
		if fromEnv.StoreConfig.Restore != nil {
			merged.StoreConfig.Restore = fromEnv.StoreConfig.Restore
		}
		if fromEnv.StoreConfig.DBConnStr != emptyString {
			merged.StoreConfig.DBConnStr = fromEnv.StoreConfig.DBConnStr
		}
		if fromEnv.AuditConfig.AuditFile.String() != emptyString {
			merged.AuditConfig.AuditFile = fromEnv.AuditConfig.AuditFile
		}
		if fromEnv.AuditConfig.AuditURL.String() != emptyString {
			merged.AuditConfig.AuditURL = fromEnv.AuditConfig.AuditURL
		}
		if fromEnv.PrivateKeyPath != emptyString {
			merged.PrivateKeyPath = fromEnv.PrivateKeyPath
		}
		if fromEnv.GRPCAddr != emptyString {
			merged.GRPCAddr = fromEnv.GRPCAddr
		}
	}

	return &merged
}

// LoadServerConfig parses command-line flags and environment variables to build a ServerConfig.
// It supports both CLI flags (e.g., -a, -k, -f) and corresponding environment variables (e.g., ADDRESS, KEY).
// The priority order is: env > flags > config file.
func LoadServerConfig(args []string) ServerConfig {
	loader := NewConfigLoader(args)
	return loader.LoadServerConfig()
}

// LoadAgentConfig parses command-line arguments and environment variables to construct an AgentConfig.
// It supports flags like -a (address), -k (hash key), -p (poll interval), -r (report interval),
// and corresponding environment variables (ADDRESS, KEY, POLL_INTERVAL, etc.).
// The priority order is: env > flags > config file.
func LoadAgentConfig(args []string) AgentConfig {
	loader := NewConfigLoader(args)
	return loader.LoadAgentConfig()
}

// LoadAgentConfig loads and merges agent configuration from defaults, file, flags, and environment variables.
func (cl *ConfigLoader) LoadAgentConfig() AgentConfig {
	defaults := cl.getDefaultAgentConfig()
	fromFile := cl.loadAgentConfigFromFile()
	fromFlags := cl.parseAgentFlags()
	fromEnv := cl.getAgentEnvConfig()

	merged := cl.mergeAgentConfigs(defaults, fromFile, fromFlags, fromEnv)

	return AgentConfig{
		SocketConfig:   merged.SocketConfig,
		PollInterval:   time.Duration(merged.PollInterval) * time.Second,
		ReportInterval: time.Duration(merged.ReportInterval) * time.Second,
		HashConfig:     merged.HashConfig,
		RateLimit:      merged.RateLimit,
		PublicKeyPath:  merged.PublicKeyPath,
		GRPCAddr:       merged.GRPCAddr,
	}
}

// getDefaultAgentConfig returns the default agent configuration values.
func (cl *ConfigLoader) getDefaultAgentConfig() *agentConfigValues {
	return &agentConfigValues{
		SocketConfig: SocketConfig{
			Host: defaultHost,
			Port: defaultPort,
		},
		HashConfig:     HashConfig{Key: emptyString},
		PollInterval:   defaultPollInterval,
		ReportInterval: defaultReportInterval,
		RateLimit:      defaultRateLimit,
		PublicKeyPath:  emptyString,
		GRPCAddr:       emptyString,
	}
}

// loadAgentConfigFromFile loads agent configuration from a JSON file specified by environment variable or flag.
func (cl *ConfigLoader) loadAgentConfigFromFile() *agentConfigValues {
	envConfigFile := os.Getenv("CONFIG")
	if envConfigFile != emptyString {
		cfg, err := LoadAgentConfigFromFile(envConfigFile)
		if err != nil {
			log.Warn().Str("config_file", envConfigFile).Err(err).Msg("Failed to load config from file")
			return nil
		}
		return cl.agentConfigJSONToValues(cfg)
	}

	// Also check -c/-config flag for config file path only
	tempFlagSet := flag.NewFlagSet(emptyString, flag.ContinueOnError)
	configFile := tempFlagSet.String("c", emptyString, "Path to config file")
	tempFlagSet.StringVar(configFile, "config", emptyString, "Path to config file")
	tempFlagSet.Parse(cl.args)

	if *configFile != emptyString {
		cfg, err := LoadAgentConfigFromFile(*configFile)
		if err != nil {
			log.Warn().Str("config_file", *configFile).Err(err).Msg("Failed to load config from file")
			return nil
		}
		return cl.agentConfigJSONToValues(cfg)
	}

	return nil
}

// agentConfigJSONToValues converts an AgentConfigJSON to agentConfigValues.
func (cl *ConfigLoader) agentConfigJSONToValues(cfg *AgentConfigJSON) *agentConfigValues {
	if cfg == nil {
		return nil
	}

	reportInterval := defaultReportInterval
	if cfg.ReportInterval != emptyString {
		if dur, err := strconv.ParseUint(cfg.ReportInterval, 10, 32); err == nil {
			reportInterval = uint(dur)
		}
	}

	pollInterval := defaultPollInterval
	if cfg.PollInterval != emptyString {
		if dur, err := strconv.ParseUint(cfg.PollInterval, 10, 32); err == nil {
			pollInterval = uint(dur)
		}
	}

	return &agentConfigValues{
		SocketConfig: SocketConfig{
			Host: defaultHost,
			Port: defaultPort,
		},
		PollInterval:   pollInterval,
		ReportInterval: reportInterval,
		PublicKeyPath:  cfg.CryptoKey,
		GRPCAddr:       cfg.GRPCAddr,
	}
}

// parseAgentFlags parses command-line flags for agent configuration.
func (cl *ConfigLoader) parseAgentFlags() *agentConfigValues {
	flagSet := flag.NewFlagSet("agent", flag.ContinueOnError)

	configFile := emptyString
	socket := SocketConfig{
		Host: defaultHost,
		Port: defaultPort,
	}
	hashKey := HashConfig{Key: emptyString}

	pollInterval := flagSet.Uint("p", defaultPollInterval, "polling interval in seconds")
	reportInterval := flagSet.Uint("r", defaultReportInterval, "report interval in seconds")
	rateLimit := flagSet.Uint("l", defaultRateLimit, "rate limit for agent")
	publicKeyPath := flagSet.String("crypto-key", emptyString, "Path to the public key for encryption")

	flagSet.StringVar(&configFile, "c", emptyString, "Path to config file")
	flagSet.StringVar(&configFile, "config", emptyString, "Path to config file")
	flagSet.Var(&socket, "a", "-a=<host>:<port>")
	flagSet.Var(&hashKey, "k", "-k=<key_for_hash>")
	grpcAddr := flagSet.String("grpc-addr", emptyString, "gRPC server address")

	if err := flagSet.Parse(cl.args); err != nil {
		log.Error().Err(err).Msg("Error parsing flags")
	}

	return &agentConfigValues{
		SocketConfig:   socket,
		HashConfig:     hashKey,
		PollInterval:   *pollInterval,
		ReportInterval: *reportInterval,
		RateLimit:      *rateLimit,
		PublicKeyPath:  *publicKeyPath,
		GRPCAddr:       *grpcAddr,
	}
}

// getAgentEnvConfig loads agent configuration from environment variables.
func (cl *ConfigLoader) getAgentEnvConfig() *agentConfigValues {
	defConfig := cl.getDefaultAgentConfig()

	if val, ok := os.LookupEnv("ADDRESS"); ok {
		defConfig.SocketConfig.Set(val)
		log.Info().Str("ADDRESS", val).Send()
	}

	if val, ok := os.LookupEnv("POLL_INTERVAL"); ok {
		if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
			defConfig.PollInterval = uint(parsed)
		}
	}

	if val, ok := os.LookupEnv("REPORT_INTERVAL"); ok {
		if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
			defConfig.ReportInterval = uint(parsed)
		}
	}

	if val, ok := os.LookupEnv("KEY"); ok {
		defConfig.HashConfig.Set(val)
		log.Info().Str("KEY", val).Send()
	}

	if val, ok := os.LookupEnv("RATE_LIMIT"); ok {
		if parsed, err := strconv.ParseUint(val, 10, 32); err == nil {
			defConfig.RateLimit = uint(parsed)
		}
	}

	if val, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		defConfig.PublicKeyPath = val
	}

	if val, ok := os.LookupEnv("GRPC_ADDR"); ok {
		defConfig.GRPCAddr = val
	}

	return defConfig
}

// mergeAgentConfigs merges agent configuration from defaults, file, flags, and environment variables with priority.
func (cl *ConfigLoader) mergeAgentConfigs(defaults, fromFile, fromFlags, fromEnv *agentConfigValues) *agentConfigValues {
	merged := *defaults

	// Apply from file if not default
	if fromFile != nil {
		if merged.SocketConfig.Host == defaultHost && merged.SocketConfig.Port == defaultPort {
			merged.SocketConfig = fromFile.SocketConfig
		}
		if merged.ReportInterval == defaultReportInterval {
			merged.ReportInterval = fromFile.ReportInterval
		}
		if merged.PollInterval == defaultPollInterval {
			merged.PollInterval = fromFile.PollInterval
		}
		if merged.PublicKeyPath == emptyString {
			merged.PublicKeyPath = fromFile.PublicKeyPath
		}
		if merged.GRPCAddr == emptyString {
			merged.GRPCAddr = fromFile.GRPCAddr
		}
	}

	// Apply from flags if not default
	if fromFlags != nil {
		if fromFlags.SocketConfig.Host != defaultHost || fromFlags.SocketConfig.Port != defaultPort {
			merged.SocketConfig = fromFlags.SocketConfig
		}
		merged.HashConfig = fromFlags.HashConfig
		if fromFlags.PollInterval != defaultPollInterval {
			merged.PollInterval = fromFlags.PollInterval
		}
		if fromFlags.ReportInterval != defaultReportInterval {
			merged.ReportInterval = fromFlags.ReportInterval
		}
		if fromFlags.RateLimit != defaultRateLimit {
			merged.RateLimit = fromFlags.RateLimit
		}
		if fromFile.GRPCAddr != emptyString {
			merged.GRPCAddr = fromFlags.GRPCAddr
		}

		merged.PublicKeyPath = fromFlags.PublicKeyPath
	}

	// Apply from env (highest priority)
	if fromEnv != nil {
		if fromEnv.SocketConfig.Host != defaultHost || fromEnv.SocketConfig.Port != defaultPort {
			merged.SocketConfig = fromEnv.SocketConfig
		}
		if fromEnv.HashConfig.Key != emptyString {
			merged.HashConfig = fromEnv.HashConfig
		}

		if fromEnv.PollInterval != defaultPollInterval {
			merged.PollInterval = fromEnv.PollInterval
		}
		if fromEnv.ReportInterval != defaultReportInterval {
			merged.ReportInterval = fromEnv.ReportInterval
		}
		if fromEnv.RateLimit != defaultRateLimit {
			merged.RateLimit = fromEnv.RateLimit
		}
		if fromEnv.GRPCAddr != emptyString {
			merged.GRPCAddr = fromEnv.GRPCAddr
		}
		merged.PublicKeyPath = fromEnv.PublicKeyPath
	}

	return &merged
}
