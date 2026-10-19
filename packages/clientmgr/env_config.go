package clientmgr

import (
	"context"
	"fmt"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
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
	return configToDTO(m.fileConfig), nil
}

func configToDTO(c *config.Config) ConfigDTO {
	return ConfigDTO{
		DefaultEnvironment: c.DefaultEnvironment,
		Timeout:            c.Timeout,
		Retries:            c.Retries,
		RetryDelay:         c.RetryDelay,
		FollowRedirects:    c.FollowRedirects,
		MaxRedirects:       c.MaxRedirects,
		ValidateSSL:        c.ValidateSSL,
		Proxy:              c.Proxy,
		Headers:            c.Headers,
		Parallel:           c.Parallel,
		Concurrency:        c.Concurrency,
	}
}

// PutConfig partially updates and persists hitspec.yaml settings. Zero values
// in dto are left unchanged; use PatchConfig to set retries to 0 or clear the
// proxy. It returns the resulting settings.
func (m *Manager) PutConfig(ctx context.Context, dto ConfigDTO) (ConfigDTO, error) {
	var patch config.Patch
	if dto.DefaultEnvironment != "" {
		patch.DefaultEnvironment = &dto.DefaultEnvironment
	}
	if dto.Timeout > 0 {
		patch.Timeout = &dto.Timeout
	}
	if dto.Retries > 0 {
		patch.Retries = &dto.Retries
	}
	if dto.RetryDelay > 0 {
		patch.RetryDelay = &dto.RetryDelay
	}
	patch.FollowRedirects = dto.FollowRedirects
	if dto.MaxRedirects > 0 {
		patch.MaxRedirects = &dto.MaxRedirects
	}
	patch.ValidateSSL = dto.ValidateSSL
	if dto.Proxy != "" {
		patch.Proxy = &dto.Proxy
	}
	patch.Headers = dto.Headers
	patch.Parallel = dto.Parallel
	if dto.Concurrency > 0 {
		patch.Concurrency = &dto.Concurrency
	}
	return m.PatchConfig(ctx, patch)
}

// PatchConfig applies a partial update where only the fields that are set
// (non-nil) change, so retries can be set to 0 and the proxy cleared. The
// change is persisted to hitspec.yaml and the resulting settings returned; a
// rejected patch (negative number) changes nothing.
func (m *Manager) PatchConfig(ctx context.Context, patch config.Patch) (ConfigDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return ConfigDTO{}, err
	}
	m.configMu.Lock()
	defer m.configMu.Unlock()
	current := m.fileConfig
	if current == nil {
		current = config.DefaultConfig()
	}
	next := *current
	if err := next.Apply(patch); err != nil {
		return ConfigDTO{}, err
	}
	prev := m.fileConfig
	m.fileConfig = &next
	if err := m.saveConfigLocked(); err != nil {
		m.fileConfig = prev
		return ConfigDTO{}, err
	}
	return configToDTO(&next), nil
}
