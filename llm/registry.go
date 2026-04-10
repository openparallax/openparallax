package llm

import (
	"fmt"
	"sync"
)

// ModelEntry holds provider+model configuration for the registry.
type ModelEntry struct {
	Name      string
	Provider  string
	Model     string
	APIKeyEnv string
	BaseURL   string
}

// Registry holds multiple initialized LLM providers keyed by name.
// Roles (chat, shield, sub_agent) map to provider names for runtime resolution.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	entries   map[string]ModelEntry
	roles     map[string]string // role → model name
}

// NewRegistry creates an empty model registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		entries:   make(map[string]ModelEntry),
		roles:     make(map[string]string),
	}
}

// Register initializes and registers a model by name.
func (r *Registry) Register(entry ModelEntry) error {
	cfg := Config{
		Provider:  entry.Provider,
		Model:     entry.Model,
		APIKeyEnv: entry.APIKeyEnv,
		BaseURL:   entry.BaseURL,
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("model %q: %w", entry.Name, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[entry.Name] = provider
	r.entries[entry.Name] = entry
	return nil
}

// SetRole maps a role to a model name.
func (r *Registry) SetRole(role, modelName string) error {
	r.mu.Lock()
	_, ok := r.providers[modelName]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("model %q not found in registry", modelName)
	}
	r.roles[role] = modelName
	r.mu.Unlock()
	return nil
}

// ForRole returns the provider assigned to a role.
func (r *Registry) ForRole(role string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name, ok := r.roles[role]
	if !ok {
		return nil, fmt.Errorf("no model assigned to role %q", role)
	}
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("model %q assigned to role %q not found", name, role)
	}
	return provider, nil
}

// Chat returns the provider for the chat role.
func (r *Registry) Chat() (Provider, error) {
	return r.ForRole("chat")
}

// Shield returns the provider for the shield evaluator role.
func (r *Registry) Shield() (Provider, error) {
	return r.ForRole("shield")
}

// SubAgent returns the provider for the sub-agent role.
// Falls back to the cheapest model in the registry if no sub_agent role is set.
func (r *Registry) SubAgent() (Provider, error) {
	p, err := r.ForRole("sub_agent")
	if err == nil {
		return p, nil
	}
	// Fallback: use the chat provider.
	return r.ForRole("chat")
}

// RoleName returns the model name assigned to a role.
func (r *Registry) RoleName(role string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.roles[role]
}

// RoleMapping returns a copy of the current role → model name mapping.
func (r *Registry) RoleMapping() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := make(map[string]string, len(r.roles))
	for k, v := range r.roles {
		m[k] = v
	}
	return m
}

// ModelNames returns all registered model names.
func (r *Registry) ModelNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Entry returns the config entry for a model name.
func (r *Registry) Entry(name string) (ModelEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[name]
	return e, ok
}

// APIHosts returns all unique API hosts for sandbox whitelisting.
func (r *Registry) APIHosts() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	var hosts []string
	for _, entry := range r.entries {
		cfg := Config{Provider: entry.Provider, BaseURL: entry.BaseURL}
		host := APIHost(cfg)
		if host != "" && !seen[host] {
			seen[host] = true
			hosts = append(hosts, host)
		}
	}
	return hosts
}
