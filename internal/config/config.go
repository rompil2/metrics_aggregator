package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/constraints"
)

type SocketConfig struct {
	Host string
	Port uint
}

func (s *SocketConfig) String() string {
	port64 := uint64(s.Port)
	portAsString := strconv.FormatUint(port64, 10)
	return strings.Join([]string{s.Host, portAsString}, ":") // like localhost:port
}

func (s *SocketConfig) Set(flagVal string) error {
	paramsArr := strings.Split(flagVal, ":")
	if len(paramsArr) != 2 {
		return errors.New("contains to many arguments")
	}
	if paramsArr[1] == "" {
		return errors.New("port should be set")
	}
	port, err := strconv.Atoi(paramsArr[1])
	if err != nil {
		return errors.New("port should be a valid decimal number")
	}
	if port > 65535 { // The maximum possible port numberfor IPv4
		return errors.New("port should be not grater than 65535")
	}
	s.Host = paramsArr[0] // it migth be an empty string
	s.Port = uint(port)
	return nil
}

func LoadServerConfig() SocketConfig {
	socket := new(SocketConfig)
	socket.Host = "localhost"
	socket.Port = 8080
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
