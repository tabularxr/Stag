package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	LogLevel string         `mapstructure:"log_level"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL      string `mapstructure:"url"`
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// Load loads configuration from environment and config files
func Load() (*Config, error) {
	// Set defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("database.url", "http://localhost:8529")
	viper.SetDefault("database.database", "stag")
	viper.SetDefault("database.username", "root")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.path", "/metrics")

	// Environment variables
	viper.SetEnvPrefix("STAG")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Try to read config file
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/stag/")

	// Read config file if exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found; ignore error and use defaults/env
	}

	// Check for ArangoDB password from multiple sources
	if viper.GetString("database.password") == "" {
		// Try ARANGO_PASSWORD env var
		if pwd := viper.GetString("ARANGO_PASSWORD"); pwd != "" {
			viper.Set("database.password", pwd)
		} else {
			return nil, fmt.Errorf("database password is required (set STAG_DATABASE_PASSWORD or ARANGO_PASSWORD)")
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}
	if c.Database.URL == "" {
		return fmt.Errorf("database URL is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}
	if c.Database.Username == "" {
		return fmt.Errorf("database username is required")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("database password is required")
	}
	return nil
}