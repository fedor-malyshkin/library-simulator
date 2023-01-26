package config

import (
	"fmt"
	"github.com/spf13/viper"
	"path/filepath"
	"strings"
)

// Config has everything
type Config struct {
	Log   LogConfig   `yaml:"log"`
	Http  HttpConfig  `yaml:"http"`
	Kafka KafkaConfig `yaml:"kafka"`
}

// LogConfig describes what to do with logs
type LogConfig struct {
	LogFilePath string `yaml:"path"`
	MaxSize     int    `yaml:"maxSizeBytes"`
	MaxAge      int    `yaml:"maxAgeHours"`
	MaxBackups  int    `yaml:"maxBackups"`
	Compress    bool   `yaml:"compress"`
	Level       string `yaml:"level"`
}

type HttpConfig struct {
	IP   string `yaml:"ip"`
	Port int    `yaml:"port"`
}

type KafkaConfig struct {
	Brokers       []string `yaml:"brokers"`
	RequestTopic  string   `yaml:"requestTopic"`
	ResponseTopic string   `yaml:"responseTopic"`
}

func (c LogConfig) BindEnv(root string) {
	for _, key := range []string{"path", "maxSizeBytes", "maxAgeHours", "maxBackups", "compress", "level"} {
		_ = viper.BindEnv(root + key)
	}
}

func (c *Config) BindEnv() {
	c.Log.BindEnv("log.")
}

// FindConfig collects config data
func FindConfig(p string) (*Config, error) {
	viper.SetConfigType("yaml")
	var cfg Config
	cfg.BindEnv()

	if p == "" {
		// no --config passed
		//setupDefaultConfigPaths()
	} else {
		absP, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		viper.SetConfigFile(absP)
	}
	viper.SetEnvPrefix("RCPT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("can't find config: %w", err)
	}

	err = viper.Unmarshal(&cfg)

	return &cfg, nil
}
