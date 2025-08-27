package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSocketConfig_Set(t *testing.T) {

	tests := []struct {
		name    string
		a       *SocketConfig
		flagVal string
		wantErr bool
	}{
		{"Positive testcase", new(SocketConfig), "localhost:8080", false},
		{"Positive testcase witout host", new(SocketConfig), ":8080", false},
		{"Negativee testcase witout port", new(SocketConfig), "localhost:", true},
		{"Negativee testcase port is too big number", new(SocketConfig), "localhost:65536", true},
		{"Negativee testcase port is not a number", new(SocketConfig), "localhost:6553i", true},
		{"Negativee testcase too many arguments", new(SocketConfig), ":65536:xxx", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.a.Set(tt.flagVal); (err != nil) != tt.wantErr {
				t.Errorf("SocketConfig.Set() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSocketConfig_String(t *testing.T) {
	tests := []struct {
		name string
		Host string
		Port uint
		want string
	}{
		// TODO: Add test cases.
		{"Positive test", "localhost", 8081, "localhost:8081"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := SocketConfig{tt.Host, tt.Port}
			if got := a.String(); got != tt.want {
				t.Errorf("SocketConfig.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadAgentConfig(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		flags          []string
		expectedConfig AgentConfig
	}{
		{
			name:    "default values",
			envVars: map[string]string{
				// Пусто - используем значения по умолчанию
			},
			expectedConfig: AgentConfig{
				PollInterval:   2 * time.Second,
				ReportInterval: 10 * time.Second,
				SocketConfig: SocketConfig{
					Host: "localhost",
					Port: 8080},
			},
		},
		{
			name:    "flags values",
			envVars: map[string]string{
				// Пусто - используем значения по умолчанию
			},
			flags: []string{"-a", "127.0.0.1:9090", "-p", "4", "-r", "6"},
			expectedConfig: AgentConfig{
				PollInterval:   4 * time.Second,
				ReportInterval: 6 * time.Second,
				SocketConfig: SocketConfig{
					Host: "127.0.0.1",
					Port: 9090},
			},
		},
		{
			name: "custom env values",
			envVars: map[string]string{
				"POLL_INTERVAL":   "5",
				"REPORT_INTERVAL": "15",
				"SERVER_HOST":     "example.com",
				"SERVER_PORT":     "9090",
			},
			expectedConfig: AgentConfig{
				PollInterval:   5 * time.Second,
				ReportInterval: 15 * time.Second,
				SocketConfig: SocketConfig{
					Host: "example.com",
					Port: 9090},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Устанавливаем переменные окружения для теста
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Получаем конфиг
			config := LoadAgentConfig(tt.flags)

			// Проверяем значения
			assert.Equal(t, tt.expectedConfig, config)
		})
	}
}

func TestGetEnvGeneral(t *testing.T) {
	tests := []struct {
		name        string
		envVarName  string
		envVarValue string
		expected    interface{}
		expectError bool
	}{
		{
			name:        "string value",
			envVarName:  "TEST_STRING",
			envVarValue: "test_value",
			expected:    "test_value",
			expectError: false,
		},
		{
			name:        "integer value",
			envVarName:  "TEST_INT",
			envVarValue: "42",
			expected:    42,
			expectError: false,
		},
		{
			name:        "missing variable",
			envVarName:  "NON_EXISTENT",
			envVarValue: "",
			expected:    0,
			expectError: true,
		},
		{
			name:        "invalid integer",
			envVarName:  "INVALID_INT",
			envVarValue: "not_a_number",
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVarValue != "" {
				os.Setenv(tt.envVarName, tt.envVarValue)
				defer os.Unsetenv(tt.envVarName)
			}

			// Тестируем для string
			if str, ok := tt.expected.(string); ok {
				result, err := getEnvGeneral[string](tt.envVarName)
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, str, result)
				}
			}

			// Тестируем для int
			if num, ok := tt.expected.(int); ok {
				result, err := getEnvGeneral[int](tt.envVarName)
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, num, result)
				}
			}
		})
	}
}

func TestGetEnvWithDefaults(t *testing.T) {
	tests := []struct {
		name         string
		envVarName   string
		envVarValue  string
		defaultValue any
		expected     any
	}{
		{
			name:         "string with default",
			envVarName:   "MISSING_STRING",
			envVarValue:  "",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "string with value",
			envVarName:   "EXISTING_STRING",
			envVarValue:  "actual",
			defaultValue: "default",
			expected:     "actual",
		},
		{
			name:         "uint with default",
			envVarName:   "MISSING_UINT",
			envVarValue:  "",
			defaultValue: uint(999),
			expected:     uint(999),
		},
		{
			name:         "uint with value",
			envVarName:   "EXISTING_UINT",
			envVarValue:  "123",
			defaultValue: uint(999),
			expected:     uint(123),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVarValue != "" {
				os.Setenv(tt.envVarName, tt.envVarValue)
				defer os.Unsetenv(tt.envVarName)
			}

			// Тестируем для string
			if str, ok := tt.expected.(string); ok {
				defaultVal := tt.defaultValue.(string)
				result := getEnvWithDefaults[string](tt.envVarName, defaultVal)
				assert.Equal(t, str, result)
			}

			// Тестируем для uint
			if num, ok := tt.expected.(uint); ok {
				defaultVal := tt.defaultValue.(uint)
				result := getEnvWithDefaults[uint](tt.envVarName, defaultVal)
				assert.Equal(t, num, result)
			}
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		name         string
		envVarName   string
		envVarValue  string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{
			name:         "valid duration",
			envVarName:   "TEST_DURATION",
			envVarValue:  "1s",
			defaultValue: 2 * time.Second,
			expected:     1 * time.Second,
		},
		{
			name:         "invalid duration",
			envVarName:   "INVALID_DURATION",
			envVarValue:  "not_a_duration",
			defaultValue: time.Hour,
			expected:     time.Hour,
		},
		{
			name:         "missing variable",
			envVarName:   "MISSING_DURATION",
			envVarValue:  "",
			defaultValue: time.Minute,
			expected:     time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVarValue != "" {
				os.Setenv(tt.envVarName, tt.envVarValue)
				defer os.Unsetenv(tt.envVarName)
			}

			result := getEnvDuration(tt.envVarName, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}
