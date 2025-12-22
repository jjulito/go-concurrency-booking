package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	DBHost     string `mapstructure:"POSTGRES_HOST"`
	DBPort     string `mapstructure:"POSTGRES_PORT"`
	DBUser     string `mapstructure:"POSTGRES_USER"`
	DBPassword string `mapstructure:"POSTGRES_PASSWORD"`
	DBName     string `mapstructure:"POSTGRES_DB"`

	RedisHost string `mapstructure:"REDIS_HOST"`
	RedisPort string `mapstructure:"REDIS_PORT"`

	AppPort string `mapstructure:"APP_PORT"`
}

func LoadConfig() (*Config, error) {
	viper.SetDefault("POSTGRES_HOST", "localhost")
	viper.SetDefault("POSTGRES_PORT", "5432")
	viper.SetDefault("POSTGRES_USER", "reserva_user")
	viper.SetDefault("POSTGRES_PASSWORD", "reserva_password")
	viper.SetDefault("POSTGRES_DB", "reserva_db")
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("APP_PORT", "8080")

	viper.AutomaticEnv()
	
	// Viper reads environment variables are case-sensitive by default on some systems, 
	// but we generally use UPPERCASE for env vars.
	// We can bind specific env vars or just rely on AutomaticEnv matching the struct keys if configured correctly.
	// For simplicity, we trust mapstructure and AutomaticEnv.

	var cfg Config
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}
	
	// Manual override if viper doesn't pick up specific env naming conventions automatically without SetEnvPrefix
	// Or we can simple rely on defaults for now if not set.
	
	return &cfg, nil
}
