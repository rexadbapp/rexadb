package provider

import "context"

type Config struct {
	Port     int
	Host     string
	DataDir  string
	User     string
	Password string
	DBName   string
}

type Provider interface {
	Name() string
	Start(ctx context.Context, cfg Config, instanceName string) error
	Stop(instanceName string) error
	Status(instanceName string) (bool, error)
	ConnectionString(cfg Config) string
}

func ValidateConfig(name string, cfg Config) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return &ConfigError{name: name, field: "port", value: cfg.Port}
	}
	if cfg.DataDir == "" {
		return &ConfigError{name: name, field: "data_dir", value: "required"}
	}
	return nil
}

type ConfigError struct {
	name  string
	field string
	value interface{}
}

func (e *ConfigError) Error() string {
	return "invalid " + e.field + " for " + e.name + ": " + formatValue(e.value)
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case int:
		return string(rune(val))
	case string:
		return val
	default:
		return "invalid"
	}
}
