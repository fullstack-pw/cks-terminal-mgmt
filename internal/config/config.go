package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port       int
	SSHKeyPath string
	SSHUser    string
	LogLevel   string
}

func Load() *Config {
	return &Config{
		Port:       getEnvInt("PORT", 8080),
		SSHKeyPath: getEnv("SSH_KEY_PATH", "/home/appuser/.ssh/id_ed25519"),
		SSHUser:    getEnv("SSH_USER", "suporte"),
		LogLevel:   getEnv("LOG_LEVEL", "INFO"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
