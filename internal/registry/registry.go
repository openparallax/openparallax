// Package registry manages the global agent registry at ~/.openparallax/agents.json.
// It tracks all agents on the machine, their workspaces, ports, and running state.
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// defaultNextPort is the starting port for new agents.
const defaultNextPort = 3100

// GRPCPortOffset is added to the web port to derive the gRPC port.
const GRPCPortOffset = 1000

// AgentRecord describes a single agent in the registry.
type AgentRecord struct {
	// Name is the human-readable agent name (e.g. "Nova").
	Name string `json:"name"`

	// Slug is the filesystem-safe name (e.g. "nova").
	Slug string `json:"slug"`

	// Workspace is the absolute path to the agent's workspace directory.
	Workspace string `json:"workspace"`

	// ConfigPath is the absolute path to the agent's config.yaml.
	ConfigPath string `json:"config_path"`

	// WebPort is the HTTP/WS port for the Web UI.
	WebPort int `json:"web_port"`

	// GRPCPort is the gRPC port for CLI-Engine communication.
	GRPCPort int `json:"grpc_port"`

	// CreatedAt is when the agent was first created.
	CreatedAt time.Time `json:"created_at"`
}

// PIDFile returns the path to the engine PID file for this agent.
func (r AgentRecord) PIDFile() string {
	return filepath.Join(r.Workspace, ".openparallax", "engine.pid")
}

// Registry is the global agent registry stored as a JSON file.
type Registry struct {
	// Agents is the list of all registered agents.
	Agents []AgentRecord `json:"agents"`

	// NextWebPort is the next port to assign to a new agent.
	NextWebPort int `json:"next_port"`

	// path is the filesystem location of agents.json.
	path string
}

// DefaultPath returns the default registry path at ~/.openparallax/agents.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".openparallax", "agents.json"), nil
}

// Load reads the registry from disk. If the file does not exist, an empty
// registry is returned with NextWebPort set to the default starting port.
func Load(path string) (*Registry, error) {
	r := &Registry{
		NextWebPort: defaultNextPort,
		path:        path,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	r.path = path
	return r, nil
}

// Save writes the registry to disk atomically via temp file + rename.
func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	data = append(data, '\n')

	// Atomic write: temp file in same directory, then rename.
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write registry temp: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename registry: %w", err)
	}
	return nil
}

// Add registers a new agent. Validates that name, slug, and ports do not
// collide with existing agents.
func (r *Registry) Add(rec AgentRecord) error {
	for _, a := range r.Agents {
		if strings.EqualFold(a.Name, rec.Name) || strings.EqualFold(a.Slug, rec.Slug) {
			return fmt.Errorf("agent %q already exists", rec.Name)
		}
		if a.WebPort == rec.WebPort {
			return fmt.Errorf("web port %d already assigned to %q", rec.WebPort, a.Name)
		}
		if a.GRPCPort == rec.GRPCPort {
			return fmt.Errorf("gRPC port %d already assigned to %q", rec.GRPCPort, a.Name)
		}
	}
	r.Agents = append(r.Agents, rec)
	if rec.WebPort >= r.NextWebPort {
		r.NextWebPort = rec.WebPort + 1
	}
	return r.Save()
}

// Remove deletes an agent by slug.
func (r *Registry) Remove(slug string) error {
	idx := -1
	for i, a := range r.Agents {
		if strings.EqualFold(a.Slug, slug) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("agent %q not found", slug)
	}
	r.Agents = append(r.Agents[:idx], r.Agents[idx+1:]...)
	return r.Save()
}

// Lookup finds an agent by name or slug (case-insensitive).
func (r *Registry) Lookup(nameOrSlug string) (*AgentRecord, bool) {
	for i := range r.Agents {
		if strings.EqualFold(r.Agents[i].Name, nameOrSlug) ||
			strings.EqualFold(r.Agents[i].Slug, nameOrSlug) {
			return &r.Agents[i], true
		}
	}
	return nil, false
}

// List returns all registered agents.
func (r *Registry) List() []AgentRecord {
	result := make([]AgentRecord, len(r.Agents))
	copy(result, r.Agents)
	return result
}

// AllocatePort returns the next available web port and increments the counter.
// The caller must call Save() to persist the change (Add does this automatically).
func (r *Registry) AllocatePort() int {
	port := r.NextWebPort
	r.NextWebPort++
	return port
}

// FindSingle returns the sole agent if exactly one is registered.
// Returns an error listing available agents otherwise.
func (r *Registry) FindSingle() (*AgentRecord, error) {
	switch len(r.Agents) {
	case 0:
		return nil, fmt.Errorf("no agents registered: run 'openparallax init' first")
	case 1:
		return &r.Agents[0], nil
	default:
		names := make([]string, len(r.Agents))
		for i, a := range r.Agents {
			names[i] = a.Name
		}
		return nil, fmt.Errorf("multiple agents registered (%s): specify an agent name", strings.Join(names, ", "))
	}
}

// Path returns the filesystem path of the registry file.
func (r *Registry) Path() string {
	return r.path
}
