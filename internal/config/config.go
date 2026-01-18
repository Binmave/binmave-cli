package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultServer  = "https://dib3oav9kh29t.cloudfront.net"
	DefaultTimeout = "5m"
	ConfigDir      = ".binmave"
	ConfigFile     = "config"
	CredentialsFile = "credentials"
)

type Config struct {
	Server  string `mapstructure:"server"`
	Timeout string `mapstructure:"timeout"`
}

var cfg *Config

// Init initializes the configuration
func Init() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	viper.SetConfigName(ConfigFile)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Set defaults
	viper.SetDefault("server", DefaultServer)
	viper.SetDefault("timeout", DefaultTimeout)

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return err
	}

	return nil
}

// Get returns the current configuration
func Get() *Config {
	if cfg == nil {
		cfg = &Config{
			Server:  DefaultServer,
			Timeout: DefaultTimeout,
		}
	}
	return cfg
}

// GetServer returns the configured server URL
func GetServer() string {
	return Get().Server
}

// SetServer updates the server URL
func SetServer(server string) error {
	viper.Set("server", server)
	return saveConfig()
}

// getConfigDir returns the path to the config directory
func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ConfigDir), nil
}

// GetConfigDir returns the config directory path
func GetConfigDir() (string, error) {
	return getConfigDir()
}

// saveConfig writes the current configuration to disk
func saveConfig() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, ConfigFile+".yaml")
	return viper.WriteConfigAs(configPath)
}
