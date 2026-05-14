// Package config provides application configuration via environment variables / .env
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
// Values are loaded from .env / environment variables via Viper.
type Config struct {
	// ── App ──────────────────────────────────────────────────
	AppName    string `mapstructure:"APP_NAME"`
	AppVersion string `mapstructure:"APP_VERSION"`
	Env        string `mapstructure:"ENVIRONMENT"`
	LogLevel   string `mapstructure:"LOG_LEVEL"`
	Port       string `mapstructure:"PORT"`
	Currency   string `mapstructure:"APP_CURRENCY"`
	Timezone   string `mapstructure:"APP_TIMEZONE"`

	// ── MongoDB ───────────────────────────────────────────────
	MongoURI     string `mapstructure:"MONGODB_URI"`
	MongoDB      string `mapstructure:"MONGODB_DB_NAME"`
	MongoPoolMin uint64 `mapstructure:"MONGODB_POOL_MIN"`
	MongoPoolMax uint64 `mapstructure:"MONGODB_POOL_MAX"`

	// ── Redis ─────────────────────────────────────────────────
	RedisURL string `mapstructure:"REDIS_URL"`

	// ── JWT ───────────────────────────────────────────────────
	JWTSecret        string        `mapstructure:"JWT_SECRET"`
	JWTAccessExpiry  time.Duration `mapstructure:"JWT_ACCESS_EXPIRY"`
	JWTRefreshExpiry time.Duration `mapstructure:"JWT_REFRESH_EXPIRY"`

	// ── OTP ───────────────────────────────────────────────────
	OTPExpiry      time.Duration `mapstructure:"OTP_EXPIRY"`
	OTPMaxAttempts int           `mapstructure:"OTP_MAX_ATTEMPTS"`

	// ── CORS ──────────────────────────────────────────────────
	CORSOrigins string `mapstructure:"CORS_ORIGINS"`

	// ── Email (SMTP) ──────────────────────────────────────────
	MailHost     string `mapstructure:"MAIL_HOST"`
	MailPort     int    `mapstructure:"MAIL_PORT"`
	MailUser     string `mapstructure:"MAIL_USERNAME"`
	MailPassword string `mapstructure:"MAIL_PASSWORD"`
	MailFrom     string `mapstructure:"MAIL_FROM"`
	MailFromName string `mapstructure:"MAIL_FROM_NAME"`

	// ── Rate Limiting ─────────────────────────────────────────
	RateLimitRequests int           `mapstructure:"RATE_LIMIT_REQUESTS"`
	RateLimitWindow   time.Duration `mapstructure:"RATE_LIMIT_WINDOW"`
	LoginMaxAttempts  int           `mapstructure:"LOGIN_MAX_ATTEMPTS"`

	// ── Admin defaults ────────────────────────────────────────
	AdminEmail    string `mapstructure:"ADMIN_EMAIL"`
	AdminPassword string `mapstructure:"ADMIN_PASSWORD"`

	// ── Automation ────────────────────────────────────────────
	AttendanceThreshold float64 `mapstructure:"ATTENDANCE_THRESHOLD"`
	MonthlyReminderDay  int     `mapstructure:"MONTHLY_REMINDER_DAY"`
	EventReminderHours  int     `mapstructure:"EVENT_REMINDER_HOURS"`
	EnableNotifications bool    `mapstructure:"ENABLE_NOTIFICATIONS"`

	// ── Worker pool ───────────────────────────────────────────
	WorkerPoolSize int `mapstructure:"WORKER_POOL_SIZE"`
	TaskQueueSize  int `mapstructure:"TASK_QUEUE_SIZE"`
}

// Load reads configuration from .env file and environment variables.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("APP_NAME", "INHERITANCE CHOIR API")
	v.SetDefault("APP_VERSION", "1.0.0")
	v.SetDefault("ENVIRONMENT", "development")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("PORT", "8080")
	v.SetDefault("APP_CURRENCY", "RWF")
	v.SetDefault("APP_TIMEZONE", "Africa/Kigali")

	v.SetDefault("MONGODB_DB_NAME", "inheritance_choir")
	v.SetDefault("MONGODB_POOL_MIN", 2)
	v.SetDefault("MONGODB_POOL_MAX", 50)

	v.SetDefault("REDIS_URL", "redis://localhost:6379/0")

	v.SetDefault("JWT_SECRET", "change-this-in-production-minimum-32-chars!")
	v.SetDefault("JWT_ACCESS_EXPIRY", "1h")
	v.SetDefault("JWT_REFRESH_EXPIRY", "720h") // 30 days

	v.SetDefault("OTP_EXPIRY", "5m")
	v.SetDefault("OTP_MAX_ATTEMPTS", 3)

	v.SetDefault("CORS_ORIGINS", "http://localhost:3000,http://localhost:5173")

	v.SetDefault("MAIL_FROM_NAME", "Inheritance Choir")

	v.SetDefault("RATE_LIMIT_REQUESTS", 100)
	v.SetDefault("RATE_LIMIT_WINDOW", "1m")
	v.SetDefault("LOGIN_MAX_ATTEMPTS", 5)

	v.SetDefault("ATTENDANCE_THRESHOLD", 70.0)
	v.SetDefault("MONTHLY_REMINDER_DAY", 5)
	v.SetDefault("EVENT_REMINDER_HOURS", 24)
	v.SetDefault("ENABLE_NOTIFICATIONS", true)

	v.SetDefault("WORKER_POOL_SIZE", 20)
	v.SetDefault("TASK_QUEUE_SIZE", 500)

	// Read .env file
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig() // OK if file doesn't exist (use env vars)

	// Allow environment variables to override
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.MongoURI == "" {
		return fmt.Errorf("MONGODB_URI is required")
	}
	if c.AdminEmail == "" {
		return fmt.Errorf("ADMIN_EMAIL is required")
	}
	if c.AdminPassword == "" {
		return fmt.Errorf("ADMIN_PASSWORD is required")
	}
	if c.EnableNotifications {
		if c.MailHost == "" {
			return fmt.Errorf("MAIL_HOST is required when notifications are enabled")
		}
		if c.MailPort == 0 {
			return fmt.Errorf("MAIL_PORT is required when notifications are enabled")
		}
		if c.MailUser == "" {
			return fmt.Errorf("MAIL_USERNAME is required when notifications are enabled")
		}
		if c.MailPassword == "" {
			return fmt.Errorf("MAIL_PASSWORD is required when notifications are enabled")
		}
		if c.MailFrom == "" {
			return fmt.Errorf("MAIL_FROM is required when notifications are enabled")
		}
	}
	return nil
}

// IsDevelopment returns true when running in development mode.
func (c *Config) IsDevelopment() bool { return c.Env == "development" }

// IsProduction returns true when running in production mode.
func (c *Config) IsProduction() bool { return c.Env == "production" }

// CORSOriginsList returns CORS origins as a slice.
func (c *Config) CORSOriginsList() []string {
	origins := strings.Split(c.CORSOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	return origins
}
