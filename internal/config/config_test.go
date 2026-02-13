package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		{"Negative testcase witout port", new(SocketConfig), "localhost:", true},
		{"Negative testcase port is too big number", new(SocketConfig), "localhost:65536", true},
		{"Negative testcase port is not a number", new(SocketConfig), "localhost:6553i", true},
		{"Negative testcase too many arguments", new(SocketConfig), ":65536:xxx", true},
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
		want string
		Port uint
	}{
		{"Positive test", "localhost", "localhost:8081", 8081},
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

func TestServerConfig(t *testing.T) {
	tests := []struct {
		envVars        map[string]string
		name           string
		flags          []string
		expectedConfig ServerConfig
	}{
		{
			name:    "default values",
			envVars: map[string]string{},
			expectedConfig: ServerConfig{
				SocketConfig: SocketConfig{
					Host: defaultHost,
					Port: defaultPort,
				},
				StoreConfig: StoreConfig{
					StoreInterval:   time.Duration(defaultStoreInterval) * time.Second,
					FileStoragePath: defaultFileStoragePath,
					Restore:         nil,
				},

				PrivateKeyPath: "",
			},
		},
		{
			name:    "values from flags",
			envVars: map[string]string{},
			flags: []string{
				"-a",
				"127.0.0.1:9092",
				"-i",
				"20",
				"-f",
				"temp.tmp",
				"-r",
				"-audit-file",
				"audit_file.txt",
				"-audit-url",
				"http://localhost:8787",
				"-crypto-key",
				"private.key",
			},
			expectedConfig: ServerConfig{
				StoreConfig: StoreConfig{
					StoreInterval:   20 * time.Second,
					FileStoragePath: "temp.tmp",
					Restore:         func(b bool) *bool { return &b }(true),
				},
				SocketConfig: SocketConfig{
					Host: "127.0.0.1",
					Port: 9092,
				},
				AuditConfig: AuditConfig{
					AuditFile: Audit{"audit_file.txt"},
					AuditURL:  Audit{"http://localhost:8787"},
				},
				PrivateKeyPath: "private.key",
			},
		},
		{
			name: "values from Env Vars",
			envVars: map[string]string{
				"ADDRESS":           "0.0.0.0:8123",
				"STORE_INTERVAL":    "2",
				"FILE_STORAGE_PATH": "/tmp/tmp.tmp",
				"RESTORE":           "false",
				"AUDIT_FILE":        "audit_file.txt",
				"AUDIT_URL":         "http://localhost:8787",
				"CRYPTO_KEY":        "private.key",
			},
			flags: []string{"-a", "127.0.0.1:9092", "-i", "20", "-f", "temp.tmp", "-r"},
			expectedConfig: ServerConfig{
				StoreConfig: StoreConfig{
					StoreInterval:   time.Duration(uint(2)) * time.Second,
					FileStoragePath: "/tmp/tmp.tmp",
					Restore:         func(b bool) *bool { return &b }(false),
				},
				SocketConfig: SocketConfig{
					Host: "0.0.0.0",
					Port: 8123,
				},
				AuditConfig: AuditConfig{
					AuditFile: Audit{"audit_file.txt"},
					AuditURL:  Audit{"http://localhost:8787"},
				},
				PrivateKeyPath: "private.key",
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
			config := LoadServerConfig(tt.flags)

			// Проверяем значения
			assert.Equal(t, tt.expectedConfig, config)
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
			envVars: map[string]string{},
			expectedConfig: AgentConfig{
				PollInterval:   2 * time.Second,
				ReportInterval: 10 * time.Second,
				SocketConfig: SocketConfig{
					Host: "localhost",
					Port: 8080},
				RateLimit: 1,
			},
		},
		{
			name:    "flags values",
			envVars: map[string]string{},
			flags:   []string{"-a", "127.0.0.1:9090", "-p", "4", "-r", "6"},
			expectedConfig: AgentConfig{
				PollInterval:   4 * time.Second,
				ReportInterval: 6 * time.Second,
				SocketConfig: SocketConfig{
					Host: "127.0.0.1",
					Port: 9090},
				RateLimit: 1,
			},
		},
		{
			name: "custom env values",
			envVars: map[string]string{
				"POLL_INTERVAL":   "5",
				"REPORT_INTERVAL": "15",
				"ADDRESS":         "example.com:9090",
			},
			expectedConfig: AgentConfig{
				PollInterval:   5 * time.Second,
				ReportInterval: 15 * time.Second,
				SocketConfig: SocketConfig{
					Host: "example.com",
					Port: 9090},
				RateLimit: 1,
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

func TestLoadServerConfigFromFile(t *testing.T) {
	// Valid config data
	validConfig := ServerConfigJSON{
		Address:       "localhost:8080",
		Restore:       true,
		StoreInterval: "1",
		StoreFile:     "/tmp/restreFile",
		DatabaseDSN:   "postgres://localhost:9092",
		CryptoKey:     "private.key",
	}

	// Create a temporary directory
	tmpDir := t.TempDir()

	validFile := filepath.Join(tmpDir, "valid_config.json")
	invalidFile := filepath.Join(tmpDir, "invalid_config.json")
	missingFile := filepath.Join(tmpDir, "missing.json")
	emptyFile := filepath.Join(tmpDir, "empty.json")

	// Write valid JSON
	data, _ := json.Marshal(validConfig)
	err := os.WriteFile(validFile, data, 0644)
	assert.NoError(t, err)

	// Write invalid JSON
	err = os.WriteFile(invalidFile, []byte("{ invalid json }"), 0644)
	assert.NoError(t, err)

	// Write empty file
	err = os.WriteFile(emptyFile, []byte{}, 0644)
	assert.NoError(t, err)

	tests := []struct {
		name       string
		filename   string
		wantConfig *ServerConfigJSON
		wantErr    bool
	}{
		{
			name:       "valid config file",
			filename:   validFile,
			wantConfig: &validConfig,
			wantErr:    false,
		},
		{
			name:       "file not found",
			filename:   missingFile,
			wantConfig: nil,
			wantErr:    true,
		},
		{
			name:       "invalid json",
			filename:   invalidFile,
			wantConfig: nil,
			wantErr:    true,
		},
		{
			name:       "empty file",
			filename:   emptyFile,
			wantConfig: nil,
			wantErr:    true, // because unmarshal fails on empty input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadServerConfigFromFile(tt.filename)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantConfig, config)
			}
		})
	}
}

func TestLoadAgentConfigFromFile(t *testing.T) {
	// Valid config data
	validConfig := AgentConfigJSON{
		Address:        "localhost:8080",
		ReportInterval: "1",
		PollInterval:   "1",
		CryptoKey:      "private.key",
	}

	// Create a temporary directory
	tmpDir := t.TempDir()

	validFile := filepath.Join(tmpDir, "valid_config.json")
	invalidFile := filepath.Join(tmpDir, "invalid_config.json")
	missingFile := filepath.Join(tmpDir, "missing.json")
	emptyFile := filepath.Join(tmpDir, "empty.json")

	// Write valid JSON
	data, _ := json.Marshal(validConfig)
	err := os.WriteFile(validFile, data, 0644)
	assert.NoError(t, err)

	// Write invalid JSON
	err = os.WriteFile(invalidFile, []byte("{ invalid json }"), 0644)
	assert.NoError(t, err)

	// Write empty file
	err = os.WriteFile(emptyFile, []byte{}, 0644)
	assert.NoError(t, err)

	tests := []struct {
		name       string
		filename   string
		wantConfig *AgentConfigJSON
		wantErr    bool
	}{
		{
			name:       "valid config file",
			filename:   validFile,
			wantConfig: &validConfig,
			wantErr:    false,
		},
		{
			name:       "file not found",
			filename:   missingFile,
			wantConfig: nil,
			wantErr:    true,
		},
		{
			name:       "invalid json",
			filename:   invalidFile,
			wantConfig: nil,
			wantErr:    true,
		},
		{
			name:       "empty file",
			filename:   emptyFile,
			wantConfig: nil,
			wantErr:    true, // because unmarshal fails on empty input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadAgentConfigFromFile(tt.filename)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantConfig, config)
			}
		})
	}
}

// Additional tests for ConfigLoader internals
func TestConfigLoader_getServerEnvConfig(t *testing.T) {
	loader := NewConfigLoader([]string{})

	envVars := map[string]string{
		"ADDRESS":           "0.0.0.0:8123",
		"STORE_INTERVAL":    "2",
		"FILE_STORAGE_PATH": "/tmp/tmp.tmp",
		"RESTORE":           "false",
		"DATABASE_DSN":      "postgres://localhost:9092",
		"KEY":               "mykey",
		"AUDIT_FILE":        "audit_file.txt",
		"AUDIT_URL":         "http://localhost:8787",
		"CRYPTO_KEY":        "private.key",
	}

	// Set environment variables
	for k, v := range envVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	config := loader.getServerEnvConfig()

	expected := &ServerConfig{
		SocketConfig: SocketConfig{
			Host: "0.0.0.0",
			Port: 8123,
		},
		HashConfig: HashConfig{Key: "mykey"},
		StoreConfig: StoreConfig{
			StoreInterval:   2 * time.Second,
			FileStoragePath: "/tmp/tmp.tmp",
			Restore:         func(b bool) *bool { return &b }(false),
			DBConnStr:       "postgres://localhost:9092",
		},
		AuditConfig: AuditConfig{
			AuditFile: Audit{"audit_file.txt"},
			AuditURL:  Audit{"http://localhost:8787"},
		},
		PrivateKeyPath: "private.key",
	}

	assert.Equal(t, expected, config)
}

func TestConfigLoader_getAgentEnvConfig(t *testing.T) {
	loader := NewConfigLoader([]string{})

	envVars := map[string]string{
		"ADDRESS":         "example.com:9090",
		"POLL_INTERVAL":   "5",
		"REPORT_INTERVAL": "15",
		"KEY":             "mykey",
		"RATE_LIMIT":      "5",
		"CRYPTO_KEY":      "public.key",
	}

	// Set environment variables
	for k, v := range envVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	config := loader.getAgentEnvConfig()

	expected := &agentConfigValues{
		SocketConfig: SocketConfig{
			Host: "example.com",
			Port: 9090,
		},
		HashConfig:     HashConfig{Key: "mykey"},
		PollInterval:   5,
		ReportInterval: 15,
		RateLimit:      5,
		PublicKeyPath:  "public.key",
	}

	assert.Equal(t, expected, config)
}

func TestConfigLoader_parseServerFlags(t *testing.T) {
	flags := []string{
		"-a", "127.0.0.1:9092",
		"-i", "20",
		"-f", "temp.tmp",
		"-r",
		"-d", "postgres://localhost:9092",
		"-k", "mykey",
		"-audit-file", "audit_file.txt",
		"-audit-url", "http://localhost:8787",
		"-crypto-key", "private.key",
	}

	loader := NewConfigLoader(flags)
	config := loader.parseServerFlags()

	expected := &ServerConfig{
		SocketConfig: SocketConfig{
			Host: "127.0.0.1",
			Port: 9092,
		},
		HashConfig: HashConfig{Key: "mykey"},
		StoreConfig: StoreConfig{
			StoreInterval:   20 * time.Second,
			FileStoragePath: "temp.tmp",
			Restore:         func(b bool) *bool { return &b }(true),
			DBConnStr:       "postgres://localhost:9092",
		},
		AuditConfig: AuditConfig{
			AuditFile: Audit{"audit_file.txt"},
			AuditURL:  Audit{"http://localhost:8787"},
		},
		PrivateKeyPath: "private.key",
	}

	assert.Equal(t, expected, config)
}

func TestConfigLoader_parseAgentFlags(t *testing.T) {
	flags := []string{
		"-a", "127.0.0.1:9090",
		"-p", "4",
		"-r", "6",
		"-l", "5",
		"-k", "mykey",
		"-crypto-key", "public.key",
	}

	loader := NewConfigLoader(flags)
	config := loader.parseAgentFlags()

	expected := &agentConfigValues{
		SocketConfig: SocketConfig{
			Host: "127.0.0.1",
			Port: 9090,
		},
		HashConfig:     HashConfig{Key: "mykey"},
		PollInterval:   4,
		ReportInterval: 6,
		RateLimit:      5,
		PublicKeyPath:  "public.key",
	}

	assert.Equal(t, expected, config)
}

func TestConfigLoader_serverConfigJSONToValues(t *testing.T) {
	jsonConfig := &ServerConfigJSON{
		Address:       "localhost:8080",
		Restore:       true,
		StoreInterval: "10",
		StoreFile:     "/tmp/storage.dat",
		DatabaseDSN:   "postgres://localhost:9092",
		CryptoKey:     "private.key",
	}

	loader := NewConfigLoader([]string{})
	config := loader.serverConfigJSONToValues(jsonConfig)

	expected := &ServerConfig{
		SocketConfig: SocketConfig{
			Host: defaultHost,
			Port: defaultPort,
		},
		StoreConfig: StoreConfig{
			StoreInterval:   10 * time.Second,
			FileStoragePath: "/tmp/storage.dat",
			Restore:         func(b bool) *bool { return &b }(true),
			DBConnStr:       "postgres://localhost:9092",
		},
		AuditConfig: AuditConfig{
			AuditFile: Audit{""},
			AuditURL:  Audit{""},
		},
		PrivateKeyPath: "private.key",
	}

	assert.Equal(t, expected, config)
}

func TestConfigLoader_agentConfigJSONToValues(t *testing.T) {
	jsonConfig := &AgentConfigJSON{
		Address:        "localhost:8080",
		ReportInterval: "10",
		PollInterval:   "5",
		CryptoKey:      "public.key",
	}

	loader := NewConfigLoader([]string{})
	config := loader.agentConfigJSONToValues(jsonConfig)

	expected := &agentConfigValues{
		SocketConfig: SocketConfig{
			Host: defaultHost,
			Port: defaultPort,
		},
		PollInterval:   5,
		ReportInterval: 10,
		PublicKeyPath:  "public.key",
	}

	assert.Equal(t, expected, config)
}
