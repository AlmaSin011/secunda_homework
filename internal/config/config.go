package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv string `mapstructure:"APP_ENV"`
	Port   string `mapstructure:"PORT"`
	LogLvl string `mapstructure:"LOG_LEVEL"`
}

func Load() (*Config, error) {
	v := viper.New()

	// Defaults — used if neither .env nor real env vars provide a value.
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("PORT", "8080")
	v.SetDefault("LOG_LEVEL", "info")

	// .env is optional: ignore "not found" errors, fail on parse errors.
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
