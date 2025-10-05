package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	AppName      string             `env:"APP_NAME" envDefault:"go-little-userbot-maker"`
	Environment  string             `env:"APP_ENV" envDefault:"development"`
	Wizard       WizardConfig       `envPrefix:"WIZARD_"`
	Orchestrator OrchestratorConfig `envPrefix:"ORCH_"`
	Database     DatabaseConfig     `envPrefix:"DB_"`
	Security     SecurityConfig     `envPrefix:"SEC_"`
}

type WizardConfig struct {
	BotToken        string        `env:"BOT_TOKEN"`
	ListenAddr      string        `env:"LISTEN_ADDR" envDefault:":8081"`
	OrchestratorURL string        `env:"ORCHESTRATOR_URL" envDefault:"http://localhost:8080"`
	StoragePath     string        `env:"STORAGE_PATH" envDefault:"./storage/logs/wizard"`
	StateTTL        time.Duration `env:"STATE_TTL" envDefault:"10m"`
	AdminIDs        []int64       `env:"ADMIN_IDS" envSeparator:","`
	UseMock         bool          `env:"USE_MOCK" envDefault:"false"`
	Debug           bool          `env:"DEBUG" envDefault:"false"`
}

type OrchestratorConfig struct {
	ListenAddr     string        `env:"LISTEN_ADDR" envDefault:":8080"`
	SecretKey      string        `env:"SECRET_KEY"`
	HealthInterval time.Duration `env:"HEALTH_INTERVAL" envDefault:"30s"`
	MaxWorkers     int           `env:"MAX_WORKERS" envDefault:"64"`
	BootstrapDelay time.Duration `env:"BOOTSTRAP_DELAY" envDefault:"2s"`
	EnableMock     bool          `env:"ENABLE_MOCK" envDefault:"false"`
}

type DatabaseConfig struct {
	URL      string        `env:"URL" envDefault:"postgres://postgres:postgres@localhost:5432/little_userbot?sslmode=disable"`
	MaxConns int           `env:"MAX_CONNS" envDefault:"10"`
	MaxIdle  int           `env:"MAX_IDLE" envDefault:"5"`
	MaxLife  time.Duration `env:"MAX_LIFE" envDefault:"30m"`
}

type SecurityConfig struct {
	SessionSalt string `env:"SESSION_SALT"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	return &cfg, nil
}