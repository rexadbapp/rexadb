package cmd

import (
	"fmt"

	"github.com/rexadb/rexadb/pkg/provider"
)

func findInstance(instName string) (*provider.InstanceInfo, provider.Provider, error) {
	for _, dbType := range provider.GetRegisteredDatabases() {
		p, err := provider.GetProvider(dbType)
		if err != nil {
			continue
		}
		inst, exists := p.GetInstance(instName)
		if exists {
			return inst, p, nil
		}
	}
	return nil, nil, fmt.Errorf("instance not found")
}

func findInstanceByName(instName string) (*provider.InstanceInfo, bool) {
	for _, dbType := range provider.GetRegisteredDatabases() {
		p, err := provider.GetProvider(dbType)
		if err != nil {
			continue
		}
		inst, exists := p.GetInstance(instName)
		if exists {
			return inst, true
		}
	}
	return nil, false
}

func getAllInstances() []*provider.InstanceInfo {
	var all []*provider.InstanceInfo
	for _, dbType := range provider.GetRegisteredDatabases() {
		p, err := provider.GetProvider(dbType)
		if err != nil {
			continue
		}
		all = append(all, p.List()...)
	}
	return all
}

func getProviderForType(dbType string) (provider.Provider, error) {
	return provider.GetProvider(dbType)
}
