package provider

import (
	"context"
	"fmt"
	"strings"
)

type Config struct {
	Port     int
	Host     string
	DataDir  string
	User     string
	Password string
	DBName   string
}

type InstanceInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	DataDir string `json:"data_dir"`
	PID     int    `json:"pid"`
}

type Provider interface {
	Name() string
	Start(ctx context.Context, cfg Config, instanceName string) error
	Stop(instanceName string) error
	Delete(instanceName string) error
	Status(instanceName string) (bool, error)
	List() []*InstanceInfo
	GetInstance(name string) (*InstanceInfo, bool)
	IsRunning(dataDir string) bool
	ConnectionString(cfg Config) string
}

type providerFactory func() (Provider, error)

var factories = make(map[string]providerFactory)

func Register(name string, factory providerFactory) {
	factories[name] = factory
}

func GetProvider(dbType string) (Provider, error) {
	dbType = strings.ToLower(dbType)

	factory, ok := factories[dbType]
	if !ok {
		supported := []string{}
		for k := range factories {
			supported = append(supported, k)
		}
		return nil, fmt.Errorf("unsupported database: %s (supported: %v)", dbType, supported)
	}

	return factory()
}

func GetSupportedDatabases() []string {
	supported := []string{}
	for k := range factories {
		supported = append(supported, k)
	}
	return supported
}

func GetRegisteredDatabases() []string {
	registered := []string{}
	for k := range factories {
		registered = append(registered, k)
	}
	return registered
}

func ListProviderNames() []string {
	return GetRegisteredDatabases()
}

func ValidateConfig(name string, cfg Config) error {
	if name != "sqlite" && (cfg.Port <= 0 || cfg.Port > 65535) {
		return fmt.Errorf("invalid port for %s: %d (must be 1-65535)", name, cfg.Port)
	}
	return nil
}
