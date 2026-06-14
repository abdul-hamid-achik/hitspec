package clientmgr

import (
	"context"
	"fmt"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
)

// ListEnvironments returns configured environments, including the active one.
func (m *Manager) ListEnvironments(ctx context.Context) ([]EnvironmentDTO, error) {
	_ = ctx
	m.configMu.RLock()
	defer m.configMu.RUnlock()

	envs := make([]EnvironmentDTO, 0)
	if m.fileConfig != nil && m.fileConfig.Environments != nil {
		for name, vars := range m.fileConfig.Environments {
			envs = append(envs, EnvironmentDTO{Name: name, Variables: vars})
		}
	}
	found := false
	for _, e := range envs {
		if e.Name == m.config.Env {
			found = true
			break
		}
	}
	if !found {
		envs = append(envs, EnvironmentDTO{Name: m.config.Env, Variables: map[string]any{}})
	}
	return envs, nil
}

// GetEnvironment returns one environment.
func (m *Manager) GetEnvironment(ctx context.Context, name string) (EnvironmentDTO, error) {
	_ = ctx
	if name == "" {
		return EnvironmentDTO{}, fmt.Errorf("name is required")
	}
	m.configMu.RLock()
	defer m.configMu.RUnlock()
	if m.fileConfig != nil && m.fileConfig.Environments != nil {
		if vars, ok := m.fileConfig.Environments[name]; ok {
			return EnvironmentDTO{Name: name, Variables: vars}, nil
		}
	}
	return EnvironmentDTO{Name: name, Variables: map[string]any{}}, nil
}

// SelectEnvironment sets the active environment.
func (m *Manager) SelectEnvironment(ctx context.Context, name string) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("name is required")
	}
	m.configMu.Lock()
	m.config.Env = name
	m.configMu.Unlock()
	m.publish("environment_changed", map[string]string{"name": name, "timestamp": nowISO()})
	return nil
}

// PutEnvironment persists an environment.
func (m *Manager) PutEnvironment(ctx context.Context, name string, vars map[string]any) (EnvironmentDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return EnvironmentDTO{}, err
	}
	if name == "" {
		return EnvironmentDTO{}, fmt.Errorf("name is required")
	}
	m.configMu.Lock()
	defer m.configMu.Unlock()
	if m.fileConfig == nil {
		m.fileConfig = config.DefaultConfig()
	}
	if m.fileConfig.Environments == nil {
		m.fileConfig.Environments = make(map[string]map[string]any)
	}
	m.fileConfig.Environments[name] = vars
	if err := m.saveConfigLocked(); err != nil {
		return EnvironmentDTO{}, err
	}
	return EnvironmentDTO{Name: name, Variables: vars}, nil
}

// GetConfig returns hitspec.yaml settings.
func (m *Manager) GetConfig(ctx context.Context) (ConfigDTO, error) {
	_ = ctx
	m.configMu.RLock()
	defer m.configMu.RUnlock()
	if m.fileConfig == nil {
		return ConfigDTO{}, nil
	}
	return ConfigDTO{
		DefaultEnvironment: m.fileConfig.DefaultEnvironment,
		Timeout:            m.fileConfig.Timeout,
		Retries:            m.fileConfig.Retries,
		FollowRedirects:    m.fileConfig.FollowRedirects,
		ValidateSSL:        m.fileConfig.ValidateSSL,
		Proxy:              m.fileConfig.Proxy,
		Headers:            m.fileConfig.Headers,
		Parallel:           m.fileConfig.Parallel,
		Concurrency:        m.fileConfig.Concurrency,
	}, nil
}

// PutConfig partially updates and persists hitspec.yaml settings.
func (m *Manager) PutConfig(ctx context.Context, dto ConfigDTO) (ConfigDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return ConfigDTO{}, err
	}
	m.configMu.Lock()
	defer m.configMu.Unlock()
	if m.fileConfig == nil {
		m.fileConfig = config.DefaultConfig()
	}
	if dto.DefaultEnvironment != "" {
		m.fileConfig.DefaultEnvironment = dto.DefaultEnvironment
	}
	if dto.Timeout > 0 {
		m.fileConfig.Timeout = dto.Timeout
	}
	if dto.Retries > 0 {
		m.fileConfig.Retries = dto.Retries
	}
	if dto.FollowRedirects != nil {
		m.fileConfig.FollowRedirects = dto.FollowRedirects
	}
	if dto.ValidateSSL != nil {
		m.fileConfig.ValidateSSL = dto.ValidateSSL
	}
	if dto.Proxy != "" {
		m.fileConfig.Proxy = dto.Proxy
	}
	if dto.Headers != nil {
		m.fileConfig.Headers = dto.Headers
	}
	if dto.Parallel != nil {
		m.fileConfig.Parallel = dto.Parallel
	}
	if dto.Concurrency > 0 {
		m.fileConfig.Concurrency = dto.Concurrency
	}
	if err := m.saveConfigLocked(); err != nil {
		return ConfigDTO{}, err
	}
	return dto, nil
}
