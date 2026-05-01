package cmd

import (
	"fmt"
	"strings"

	"github.com/rexadb/rexadb/pkg/provider"
)

func getDbTypeFromInstanceName(instName string) string {
	parts := strings.SplitN(instName, "-", 2)
	return parts[0]
}

func findInstance(dbType string, instName string) (*provider.InstanceInfo, provider.Provider, error) {
	p, err := provider.GetProvider(dbType)
	if err != nil {
		return nil, nil, err
	}
	inst, exists := p.GetInstance(instName)
	if exists {
		return inst, p, nil
	}
	return nil, nil, fmt.Errorf("instance not found")
}

func findInstanceByName(dbType string, instName string) (*provider.InstanceInfo, bool) {
	p, err := provider.GetProvider(dbType)
	if err != nil {
		return nil, false
	}
	inst, exists := p.GetInstance(instName)
	if exists {
		return inst, true
	}
	return nil, false
}

func findInstanceByDataDir(dbType string, dataDir string) (*provider.InstanceInfo, provider.Provider, bool) {
	p, err := provider.GetProvider(dbType)
	if err != nil {
		return nil, nil, false
	}
	for _, inst := range p.List() {
		if inst.DataDir == dataDir {
			return inst, p, true
		}
	}
	return nil, nil, false
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
