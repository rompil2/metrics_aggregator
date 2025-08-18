package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/constraints"
)

type SocketConfig struct {
	ServerHost string
	ServerPort uint
}

func LoadServerConfig() SocketConfig {
	return SocketConfig{
		ServerHost: getEnv("SERVER_HOST", "localhost"),
		ServerPort: getEnvUint("SERVER_PORT", 8080),
	}
}

type AgentConfig struct {
	SocketConfig
	PollInterval   time.Duration
	ReportInterval time.Duration
}

func LoadAgentConfig() AgentConfig {
	ac := AgentConfig{
		PollInterval:   getEnvDuration("POLL_INTERVAL", 2*time.Second),
		ReportInterval: getEnvDuration("REPORT_INTERVAL", 10*time.Second),
	}
	ac.SocketConfig = LoadServerConfig()
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
		// I suppose th eduration is set in seconds with a fraction part
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
var getEnv = getEnvWithDefaults[string]
var getEnvUint = getEnvWithDefaults[uint]

func (s *SocketConfig) String() string {
	port64 := uint64(s.ServerPort)
	portAsString := strconv.FormatUint(port64, 10)
	return strings.Join([]string{s.ServerHost, portAsString}, ":") // like localhost:port
}
