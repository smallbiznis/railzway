package observability

import (
	"os"
	"strconv"
	"strings"

	"github.com/smallbiznis/valora/internal/config"
)

// Config holds observability configuration derived from environment variables.
type Config struct {
	ServiceName string
	Environment string
	Version     string

	LogLevel  string
	LogFormat string

	OtelEnabled          bool
	OtelExporterEndpoint string
	OtelExporterProtocol string
	OtelSamplingRatio    float64
}

func LoadConfig(cfg config.Config) Config {
	serviceName := strings.TrimSpace(cfg.AppName)
	if serviceName == "" {
		serviceName = "valora"
	}
	environment := getenv("DEPLOYMENT_ENV", cfg.Environment)
	version := getenv("SERVICE_VERSION", cfg.AppVersion)
	logLevel := strings.ToLower(strings.TrimSpace(getenv("LOG_LEVEL", "info")))
	logFormat := strings.ToLower(strings.TrimSpace(getenv("LOG_FORMAT", "json")))
	otlpEndpoint := getenv("OTEL_EXPORTER_OTLP_ENDPOINT", cfg.OTLPEndpoint)
	otlpProtocol := strings.ToLower(strings.TrimSpace(getenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")))
	if tracesProtocol := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")); tracesProtocol != "" {
		otlpProtocol = strings.ToLower(tracesProtocol)
	}

	samplingRatio := getenvFloat("OTEL_SAMPLING_RATIO", 0.1)
	enabled := getenvBool("OTEL_ENABLED", true)

	return Config{
		ServiceName:          serviceName,
		Environment:          strings.TrimSpace(environment),
		Version:              strings.TrimSpace(version),
		LogLevel:             logLevel,
		LogFormat:            logFormat,
		OtelEnabled:          enabled,
		OtelExporterEndpoint: strings.TrimSpace(otlpEndpoint),
		OtelExporterProtocol: otlpProtocol,
		OtelSamplingRatio:    samplingRatio,
	}
}

func (c Config) Debug() bool {
	level := strings.ToLower(strings.TrimSpace(c.LogLevel))
	if level == "debug" {
		return true
	}
	return isDevEnv(c.Environment)
}

func isDevEnv(env string) bool {
	env = strings.ToLower(strings.TrimSpace(env))
	switch env {
	case "dev", "development", "local", "test":
		return true
	default:
		return false
	}
}

func getenv(key, def string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
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
