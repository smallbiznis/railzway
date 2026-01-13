package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application configuration.
type Config struct {
	AppName                     string
	AppVersion                  string
	Mode                        string
	Environment                 string
	Port                        string
	AuthCookieSecure            bool
	DefaultOrgID                int64
	AuthJWTSecret               string
	PaymentProviderConfigSecret string

	OTLPEndpoint string
	StaticDir    string

	Cloud     CloudConfig
	Bootstrap BootstrapConfig

	DBType            string
	DBHost            string
	DBPort            string
	DBName            string
	DBUser            string
	DBPassword        string
	DBSSLMode         string
	DBMaxIdleConn     int
	DBMaxOpenConn     int
	DBConnMaxLifetime int
	DBConnMaxIdleTime int

	OAuth2ClientID     string
	OAuth2ClientSecret string

	RateLimit RateLimitConfig
	Email     EmailConfig
	Logger    LoggerConfig
}

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

type LoggerConfig struct {
	Level string
}

type CloudConfig struct {
	OrganizationID   string
	OrganizationName string
	Metrics          CloudMetricsConfig
}

type CloudMetricsConfig struct {
	Enabled   bool
	Exporter  string
	Endpoint  string
	AuthToken string
}

type BootstrapConfig struct {
	EnsureDefaultOrgAndUser bool
	AllowSignUp             bool
	AllowAssignOrg          bool
	AllowAssignUserRole     string
	AutoAssignOrgID         string
	AutoAssignOrgRole       string
}

type RateLimitConfig struct {
	Enabled bool

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	UsageIngestOrgRate               float64
	UsageIngestOrgBurst              int
	UsageIngestEndpointRate          float64
	UsageIngestEndpointBurst         int
	UsageIngestConcurrencyTTLSeconds int
}

type BillingConfig struct {
	AgingBuckets []AgingBucket `mapstructure:"agingBuckets"`
	RiskLevels   []RiskLevel   `mapstructure:"riskLevels"`
}

type AgingBucket struct {
	Label   string `mapstructure:"label"`
	MinDays int    `mapstructure:"minDays"`
	MaxDays *int   `mapstructure:"maxDays"` // pointer = nullable
}

type RiskLevel struct {
	Level          string `mapstructure:"level"`
	MinOutstanding int64  `mapstructure:"minOutstanding"`
	MinDays        int    `mapstructure:"minDays"`
}

// Load loads configuration from environment variables and .env file.
func Load() Config {
	_ = godotenv.Load()

	mode := normalizeMode(getenv("APP_MODE", ModeOSS))
	environment := getenv("ENVIRONMENT", "development")
	authCookieSecure := environment == "production"
	if !authCookieSecure {
		authCookieSecure = getenvBool("AUTH_COOKIE_SECURE", false)
	}

	cfg := Config{
		AppName:                     getenv("APP_SERVICE", "valora"),
		AppVersion:                  getenv("APP_VERSION", "0.1.0"),
		Mode:                        mode,
		Environment:                 environment,
		Port:                        getenv("PORT", "8080"),
		AuthCookieSecure:            authCookieSecure,
		DefaultOrgID:                getenvInt64("DEFAULT_ORG", 0),
		AuthJWTSecret:               strings.TrimSpace(getenv("AUTH_JWT_SECRET", "")),
		PaymentProviderConfigSecret: strings.TrimSpace(getenv("PAYMENT_PROVIDER_CONFIG_SECRET", "base64:Kq7N2f1Jx9yY4mFZp+u7qZb8c9d0eFQ1vS3nZk6hL2A=")),
		OTLPEndpoint:                getenv("OTLP_ENDPOINT", "localhost:4317"),
		StaticDir:                   getenv("STATIC_DIR", "apps/admin/dist"),
		Cloud: CloudConfig{
			OrganizationID:   strings.TrimSpace(getenv("CLOUD_ORGANIZATION_ID", "")),
			OrganizationName: getenv("CLOUD_ORGANIZATION_NAME", ""),
			Metrics: CloudMetricsConfig{
				Enabled:   getenvBool("CLOUD_METRICS_ENABLED", true),
				Exporter:  strings.ToLower(getenv("CLOUD_METRICS_EXPORTER", "")),
				Endpoint:  strings.TrimSpace(getenv("CLOUD_METRICS_ENDPOINT", "")),
				AuthToken: strings.TrimSpace(getenv("CLOUD_METRICS_AUTH_TOKEN", "")),
			},
		},
		Bootstrap: BootstrapConfig{
			EnsureDefaultOrgAndUser: getenvBool("ENSURE_DEFAULT_ORG_AND_USER", true),
			AllowSignUp:             getenvBool("ALLOW_SIGNUP", false),
			AllowAssignOrg:          getenvBool("ALLOW_ASSIGN_ORG", false),
			AllowAssignUserRole:     strings.TrimSpace(getenv("ALLOW_ASSIGN_USER_ROLE", "")),
			AutoAssignOrgID:         strings.TrimSpace(getenv("AUTO_ASSIGN_ORG_ID", "")),
			AutoAssignOrgRole:       strings.TrimSpace(getenv("AUTO_ASSIGN_ORG_ROLE", "")),
		},
		DBType:     getenv("DB_TYPE", "postgres"),
		DBHost:     getenv("DB_HOST", "localhost"),
		DBPort:     getenv("DB_PORT", "5433"),
		DBName:     getenv("DB_NAME", "postgres"),
		DBUser:     getenv("DB_USER", "postgres"),
		DBPassword: getenv("DB_PASSWORD", "35411231"),
		DBSSLMode:  getenv("DB_SSL_MODE", "disable"),

		// Database connection pool settings
		DBMaxIdleConn:     getenvInt("DB_MAX_IDLE_CONN", 10),
		DBMaxOpenConn:     getenvInt("DB_MAX_OPEN_CONN", 50),
		DBConnMaxLifetime: getenvInt("DB_CONN_MAX_LIFETIME", 300),
		DBConnMaxIdleTime: getenvInt("DB_CONN_MAX_IDLE_TIME", 60),

		// OAuth2 settings
		OAuth2ClientID:     strings.TrimSpace(getenv("OAUTH2_CLIENT_ID", "")),
		OAuth2ClientSecret: strings.TrimSpace(getenv("OAUTH2_CLIENT_SECRET", "")),

		// Ratelimit settings
		RateLimit: RateLimitConfig{
			Enabled:                          getenvBool("USAGE_INGEST_RATE_LIMIT_ENABLED", true),
			RedisAddr:                        strings.TrimSpace(getenv("RATE_LIMIT_REDIS_ADDR", "")),
			RedisPassword:                    strings.TrimSpace(getenv("RATE_LIMIT_REDIS_PASSWORD", "")),
			RedisDB:                          getenvInt("RATE_LIMIT_REDIS_DB", 0),
			UsageIngestOrgRate:               getenvFloat("USAGE_INGEST_ORG_RATE", 25),
			UsageIngestOrgBurst:              getenvInt("USAGE_INGEST_ORG_BURST", 50),
			UsageIngestEndpointRate:          getenvFloat("USAGE_INGEST_ENDPOINT_RATE", 15),
			UsageIngestEndpointBurst:         getenvInt("USAGE_INGEST_ENDPOINT_BURST", 30),
			UsageIngestConcurrencyTTLSeconds: clampInt(getenvInt("USAGE_INGEST_CONCURRENCY_TTL_SECONDS", 3), 2, 5),
		},

		Email: EmailConfig{
			SMTPHost:     getenv("SMTP_HOST", "localhost"),
			SMTPPort:     getenvInt("SMTP_PORT", 1025),
			SMTPUsername: getenv("SMTP_USERNAME", ""),
			SMTPPassword: getenv("SMTP_PASSWORD", ""),
			SMTPFrom:     getenv("SMTP_FROM", "no-reply@valora.test"),
		},

		Logger: LoggerConfig{
			Level: getenv("LOG_LEVEL", "info"),
		},

		// Vault settings
	}

	return cfg
}

const (
	ModeOSS        = "oss"
	ModeCloud      = "cloud"
	ModeStandalone = "standalone"
)

func (c Config) IsCloud() bool {
	return c.Mode == ModeCloud
}

func normalizeMode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case ModeCloud:
		return ModeCloud
	case ModeStandalone, ModeOSS:
		return ModeOSS
	default:
		return ModeOSS
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return def
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func getenvInt64(key string, def int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return def
	}
	return parsed
}

func getenvInt(key string, def int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return def
	}
	return parsed
}

func getenvFloat(key string, def float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return def
	}
	return parsed
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func parseServices(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		log.Println("no services enabled for migration")
	}
	return out
}
