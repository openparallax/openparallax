package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)
	assert.Empty(t, reg.Agents)
	assert.Equal(t, defaultNextPort, reg.NextWebPort)
}

func TestAddAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	rec := AgentRecord{
		Name:       "Nova",
		Slug:       "nova",
		Workspace:  "/home/user/.openparallax/nova",
		ConfigPath: "/home/user/.openparallax/nova/config.yaml",
		WebPort:    3100,
		GRPCPort:   4100,
		CreatedAt:  time.Now(),
	}
	require.NoError(t, reg.Add(rec))
	assert.Equal(t, 3101, reg.NextWebPort)

	// Reload from disk.
	reg2, err := Load(path)
	require.NoError(t, err)
	require.Len(t, reg2.Agents, 1)
	assert.Equal(t, "Nova", reg2.Agents[0].Name)
	assert.Equal(t, "nova", reg2.Agents[0].Slug)
	assert.Equal(t, 3100, reg2.Agents[0].WebPort)
}

func TestDuplicateNameRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	rec := AgentRecord{Name: "Nova", Slug: "nova", WebPort: 3100, GRPCPort: 4100, CreatedAt: time.Now()}
	require.NoError(t, reg.Add(rec))

	dup := AgentRecord{Name: "nova", Slug: "nova", WebPort: 3101, GRPCPort: 4101, CreatedAt: time.Now()}
	assert.Error(t, reg.Add(dup))
}

func TestDuplicatePortRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	rec := AgentRecord{Name: "Nova", Slug: "nova", WebPort: 3100, GRPCPort: 4100, CreatedAt: time.Now()}
	require.NoError(t, reg.Add(rec))

	dup := AgentRecord{Name: "Jarvis", Slug: "jarvis", WebPort: 3100, GRPCPort: 4101, CreatedAt: time.Now()}
	assert.Error(t, reg.Add(dup))
}

func TestRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	require.NoError(t, reg.Add(AgentRecord{Name: "Nova", Slug: "nova", WebPort: 3100, GRPCPort: 4100, CreatedAt: time.Now()}))
	require.NoError(t, reg.Add(AgentRecord{Name: "Jarvis", Slug: "jarvis", WebPort: 3101, GRPCPort: 4101, CreatedAt: time.Now()}))
	assert.Len(t, reg.Agents, 2)

	require.NoError(t, reg.Remove("nova"))
	assert.Len(t, reg.Agents, 1)
	assert.Equal(t, "Jarvis", reg.Agents[0].Name)
}

func TestRemoveNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)
	assert.Error(t, reg.Remove("nonexistent"))
}

func TestLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	require.NoError(t, reg.Add(AgentRecord{Name: "Nova", Slug: "nova", WebPort: 3100, GRPCPort: 4100, CreatedAt: time.Now()}))

	// By name (case-insensitive).
	rec, ok := reg.Lookup("NOVA")
	assert.True(t, ok)
	assert.Equal(t, "Nova", rec.Name)

	// By slug.
	rec, ok = reg.Lookup("nova")
	assert.True(t, ok)
	assert.Equal(t, "Nova", rec.Name)

	// Missing.
	_, ok = reg.Lookup("jarvis")
	assert.False(t, ok)
}

func TestAllocatePort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	p1 := reg.AllocatePort()
	p2 := reg.AllocatePort()
	p3 := reg.AllocatePort()

	assert.Equal(t, 3100, p1)
	assert.Equal(t, 3101, p2)
	assert.Equal(t, 3102, p3)
}

func TestFindSingleNone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	_, err = reg.FindSingle()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no agents registered")
}

func TestFindSingleOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	require.NoError(t, reg.Add(AgentRecord{Name: "Nova", Slug: "nova", WebPort: 3100, GRPCPort: 4100, CreatedAt: time.Now()}))

	rec, err := reg.FindSingle()
	require.NoError(t, err)
	assert.Equal(t, "Nova", rec.Name)
}

func TestFindSingleMultiple(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	require.NoError(t, reg.Add(AgentRecord{Name: "Nova", Slug: "nova", WebPort: 3100, GRPCPort: 4100, CreatedAt: time.Now()}))
	require.NoError(t, reg.Add(AgentRecord{Name: "Jarvis", Slug: "jarvis", WebPort: 3101, GRPCPort: 4101, CreatedAt: time.Now()}))

	_, err = reg.FindSingle()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "multiple agents")
	assert.Contains(t, err.Error(), "Nova")
	assert.Contains(t, err.Error(), "Jarvis")
}

func TestPIDFile(t *testing.T) {
	rec := AgentRecord{Workspace: "/home/user/.openparallax/nova"}
	assert.Equal(t, "/home/user/.openparallax/nova/.openparallax/engine.pid", rec.PIDFile())
}

func TestAtomicSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	reg, err := Load(path)
	require.NoError(t, err)

	require.NoError(t, reg.Add(AgentRecord{Name: "Nova", Slug: "nova", WebPort: 3100, GRPCPort: 4100, CreatedAt: time.Now()}))

	// Verify file exists and no temp file remains.
	_, err = os.Stat(path)
	assert.NoError(t, err)
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))
}
