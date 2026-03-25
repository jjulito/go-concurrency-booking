package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	DBHost        string `mapstructure:"POSTGRES_HOST"`
	DBPort        string `mapstructure:"POSTGRES_PORT"`
	DBUser        string `mapstructure:"POSTGRES_USER"`
	DBPassword    string `mapstructure:"POSTGRES_PASSWORD"`
	DBName        string `mapstructure:"POSTGRES_DB"`
	DBSSLMode     string `mapstructure:"POSTGRES_SSLMODE"`

	RedisHost     string `mapstructure:"REDIS_HOST"`
	RedisPort     string `mapstructure:"REDIS_PORT"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`
	RedisDB       int    `mapstructure:"REDIS_DB"`

	AppPort             string `mapstructure:"APP_PORT"`
	StripeWebhookSecret string `mapstructure:"STRIPE_WEBHOOK_SECRET"`
}

func LoadConfig() (*Config, error) {
	viper.SetDefault("POSTGRES_HOST", "localhost")
	viper.SetDefault("POSTGRES_PORT", "5432")
	viper.SetDefault("POSTGRES_USER", "reserva_user")
	viper.SetDefault("POSTGRES_PASSWORD", "reserva_password")
	viper.SetDefault("POSTGRES_DB", "reserva_db")
	viper.SetDefault("POSTGRES_SSLMODE", "disable") // override to "require" in production
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_PASSWORD", "")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("STRIPE_WEBHOOK_SECRET", "")

	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
