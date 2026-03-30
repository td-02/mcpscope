package appconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment  string       `json:"environment"`
	AuthToken    string       `json:"authToken"`
	Notification Notification `json:"notification"`
	Proxy        ProxyConfig  `json:"proxy"`
}

type Notification struct {
	WebhookURLs []string `json:"webhookUrls"`
}

type ProxyConfig struct {
	DB         string   `json:"db"`
	Port       int      `json:"port"`
	Transport  string   `json:"transport"`
	RetainFor  string   `json:"retainFor"`
	MaxTraces  int      `json:"maxTraces"`
	RedactKeys []string `json:"redactKeys"`
	EnableOTEL bool     `json:"otel"`
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config file: %w", err)
	}

	return cfg, nil
}

func (c Config) RetentionDuration() time.Duration {
	if strings.TrimSpace(c.Proxy.RetainFor) == "" {
		return 0
	}
	duration, err := time.ParseDuration(c.Proxy.RetainFor)
	if err != nil {
		return 0
	}
	return duration
}
