package mcp

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AuthHeaderName string `yaml:"AuthHeaderName"`
	AuthSecret     string `yaml:"AuthSecret"`
	UseSSE         bool   `yaml:"UseSSE"`
}

func NewConfigFromFile(configPath string) (Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}
	return NewConfig(data)
}

func NewConfig(data []byte) (Config, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}

	// Set default value for AuthHeaderName if not set
	if config.AuthHeaderName == "" {
		config.AuthHeaderName = "Authorization"
	}
	return config, nil
}
