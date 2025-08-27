package config

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/exp/constraints"
)

const (
	default_host           = "localhost"
	default_port           = 8080
	default_pollInterval   = 2
	default_reportInterval = 10
)

type SocketConfig struct {
	Host string
	Port uint
}

func (s *SocketConfig) String() string {
	port64 := uint64(s.Port)
	portAsString := strconv.FormatUint(port64, 10)
	return net.JoinHostPort(s.Host, portAsString)
}

func (s *SocketConfig) Set(flagVal string) error {
	host, portStr, err := net.SplitHostPort(flagVal)
	if err != nil {
		return fmt.Errorf("invalid address format: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return errors.New("port should be a valid decimal number")
	}
	if port > 65535 { // The maximum possible port number for IPv4
		return errors.New("port should be not grater than 65535")
	}
	s.Host = host // it migth be an empty string
	s.Port = uint(port)
	return nil
}

func LoadServerConfig() SocketConfig {
	socket := new(SocketConfig)
	socket.Host = default_host
	socket.Port = default_port
	flag.Var(socket, "a", "-a=<host>:<port>")
	flag.Parse()

	if val, err := getEnv("SERVER_HOST"); err == nil {
		socket.Host = val
	}
	if val, err := getEnvUint("SERVER_PORT"); err == nil {
		socket.Port = val
	}
	return *socket
}

type AgentConfig struct {
	SocketConfig
	PollInterval   time.Duration
	ReportInterval time.Duration
}

func LoadAgentConfig(args []string) AgentConfig {
	ac := AgentConfig{}
	flagSet := flag.NewFlagSet("agent", flag.ContinueOnError)
	ac.Host = default_host
	ac.Port = default_port
	flagSet.Var(&ac.SocketConfig, "a", "-a=<host>:<port>")
	pollInterval := flagSet.Uint("p", default_pollInterval, "polling Interval in sec")
	reportInterval := flagSet.Uint("r", default_reportInterval, "report Interval in sec")
	flagSet.Parse(args)

	if val, err := getEnvUint("POLL_INTERVAL"); err == nil {
		*pollInterval = val
	}
	if val, err := getEnvUint("REPORT_INTERVAL"); err == nil {
		*reportInterval = val
	}
	if val, err := getEnv("SERVER_HOST"); err == nil {
		ac.Host = val
	}
	if val, err := getEnvUint("SERVER_PORT"); err == nil {
		ac.Port = val
	}
	ac.PollInterval = time.Duration(*pollInterval) * time.Second
	ac.ReportInterval = time.Duration(*reportInterval) * time.Second
	return ac
}

func getEnvGeneral[T constraints.Integer | ~string](envVarName string) (T, error) {
	var noResult T

	envVarStr, exists := os.LookupEnv(envVarName)
	if !exists {
		return noResult, fmt.Errorf("%s does not exist", envVarName)
	}
	switch any(noResult).(type) {

	case string:
		return any(envVarStr).(T), nil

	case time.Duration:
		// I suppose the duration is set in seconds with a fraction part
		d, err := time.ParseDuration(envVarStr)
		if err != nil {
			return noResult, fmt.Errorf("cannot convert %s to Duration type", envVarStr)
		}
		return any(d).(T), nil
	case int:
		v, err := strconv.Atoi(envVarStr)
		if err != nil {
			return noResult, fmt.Errorf("cannot convert %s to number", envVarStr)
		}
		return any(v).(T), nil
	case uint:
		v, err := strconv.Atoi(envVarStr)
		if err != nil {
			return noResult, fmt.Errorf("cannot convert %s to number", envVarStr)
		}
		return any(uint(v)).(T), nil
	default:
		panic("the type %T is not supported yet")
	}
}

func getEnvWithDefaults[T constraints.Integer | string](envVarName string, defaultValue T) T {
	value, err := getEnvGeneral[T](envVarName)
	if err != nil {
		return defaultValue
	}
	return value
}

var getEnvDuration = getEnvWithDefaults[time.Duration]
var getEnv = getEnvGeneral[string]
var getEnvUint = getEnvGeneral[uint]
