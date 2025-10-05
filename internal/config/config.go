package config

import (
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

type StoreConfig struct {
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
	DBConnStr       string
}

type ServerConfig struct {
	SocketConfig
	StoreConfig
	HashConfig
}

func LoadServerConfig(args []string) ServerConfig {
	flagSet := flag.NewFlagSet("server", flag.ContinueOnError)

	socket := SocketConfig{
		Host: defaultHost,
		Port: defaultPort,
	}
	hashKey := HashConfig{
		Key: emptyString,
	}
	flagSet.Var(&socket, "a", "-a=<host>:<port>")
	flagSet.Var(&hashKey, "k", "-k=<key_for_hash>")
	storeInterval := flagSet.Uint("i", defaultStoreInterval, "storing interval in seconds")
	fileStoragePath := flagSet.String("f", defaultFileStoragePath, "path to a file to store data")
	restore := flagSet.Bool("r", defaultRestore, "should restore data")
	database := flagSet.String("d", "", "A DB connection string")

	if err := flagSet.Parse(args); err != nil {
		log.Error().Err(err).Msg("Error parsing flags")
	}

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
		log.Info().Bool("FILE_STORAGE_PATH", *restore).Send()
	}

	if val, ok := os.LookupEnv("DATABASE_DSN"); ok {
		*database = strings.ToLower(val)
		log.Info().Str("DATABASE_DSN", *database).Send()
	}
	if val, ok := os.LookupEnv("KEY"); ok {
		hashKey.Set(val)
		log.Info().Str("KEY", val).Send()
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
	}
}

type AgentConfig struct {
	SocketConfig
	HashConfig
	PollInterval   time.Duration
	ReportInterval time.Duration
	RateLimit      uint
}

func LoadAgentConfig(args []string) AgentConfig {
	flagSet := flag.NewFlagSet("agent", flag.ContinueOnError)

	socket := SocketConfig{
		Host: defaultHost,
		Port: defaultPort,
	}
	hashKey := HashConfig{
		Key: emptyString,
	}

	flagSet.Var(&socket, "a", "-a=<host>:<port>")
	flagSet.Var(&hashKey, "k", "-k=<key_for_hash>")

	pollInterval := flagSet.Uint("p", defaultPollInterval, "polling interval in seconds")
	reportInterval := flagSet.Uint("r", defaultReportInterval, "report interval in seconds")
	rateLimit := flagSet.Uint("l", 1, "rate limit for agent")

	if err := flagSet.Parse(args); err != nil {
		log.Error().Err(err).Msg("Error parsing flags")
	}

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

	return AgentConfig{
		SocketConfig:   socket,
		PollInterval:   time.Duration(*pollInterval) * time.Second,
		ReportInterval: time.Duration(*reportInterval) * time.Second,
		HashConfig:     hashKey,
		RateLimit:      *rateLimit,
	}
}
