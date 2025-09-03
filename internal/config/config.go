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

const (
	defaultHost            = "localhost"
	defaultPort            = 8080
	defaultPollInterval    = 2
	defaultReportInterval  = 10
	defaultStoreInterval   = 300
	defaultFileStoragePath = "./storage.txt"
	defaultRestore         = false
)

type SocketConfig struct {
	Host string
	Port uint
}

var log = logger.Get()

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

type StoreConfig struct {
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
}

type ServerConfig struct {
	SocketConfig
	StoreConfig
}

func LoadServerConfig(args []string) ServerConfig {
	flagSet := flag.NewFlagSet("server", flag.ContinueOnError)

	socket := &SocketConfig{
		Host: defaultHost,
		Port: defaultPort,
	}
	flagSet.Var(socket, "a", "-a=<host>:<port>")

	storeInterval := flagSet.Uint("i", defaultStoreInterval, "storing interval in seconds")
	fileStoragePath := flagSet.String("f", defaultFileStoragePath, "path to a file to store data")
	restore := flagSet.Bool("r", defaultRestore, "should restore data")

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

	return ServerConfig{
		SocketConfig: *socket,
		StoreConfig: StoreConfig{
			StoreInterval:   time.Duration(*storeInterval) * time.Second,
			FileStoragePath: *fileStoragePath,
			Restore:         *restore,
		},
	}
}

type AgentConfig struct {
	SocketConfig
	PollInterval   time.Duration
	ReportInterval time.Duration
}

func LoadAgentConfig(args []string) AgentConfig {
	flagSet := flag.NewFlagSet("agent", flag.ContinueOnError)

	socket := SocketConfig{
		Host: defaultHost,
		Port: defaultPort,
	}
	flagSet.Var(&socket, "a", "-a=<host>:<port>")

	pollInterval := flagSet.Uint("p", defaultPollInterval, "polling interval in seconds")
	reportInterval := flagSet.Uint("r", defaultReportInterval, "report interval in seconds")

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

	return AgentConfig{
		SocketConfig:   socket,
		PollInterval:   time.Duration(*pollInterval) * time.Second,
		ReportInterval: time.Duration(*reportInterval) * time.Second,
	}
}
