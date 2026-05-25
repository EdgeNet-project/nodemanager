package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config holds the nodemanager configuration
type Config struct {
	Server   string `mapstructure:"server"`
	Identity string `mapstructure:"identity"`
	State    string `mapstructure:"state"`

	// Kubernetes specific configuration
	KubernetesVersion string `mapstructure:"kubernetes_version"`
	APIServerIP       string `mapstructure:"api_server_ip"`
}

// Load loads the configuration from the given path
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("ini")

	if configPath == "" {
		fmt.Println("No config file provided, using default values")
		return &Config{}, nil
	}

	if _, err := os.Stat(configPath); err != nil {
		fmt.Printf("Config file %s not found, using default values\n", configPath)
		return &Config{}, nil
	}

	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	fmt.Printf("Config file %s loaded:\n", configPath)
	if data, err := os.ReadFile(configPath); err == nil {
		fmt.Println(string(data))
	}

	var cfg Config
	if err := v.UnmarshalKey("nodemanager", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if cfg.Server == "" {
		return nil, fmt.Errorf("server (node API endpoint) is not defined in config")
	}

	if cfg.APIServerIP == "" {
		cfg.APIServerIP = "10.0.40.79"
	}

	if cfg.KubernetesVersion == "" {
		cfg.KubernetesVersion = "1.30"
	}

	return &cfg, nil
}
