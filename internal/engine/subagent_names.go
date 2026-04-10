package engine

// defaultNamePool is the pool of names for sub-agents.
var defaultNamePool = []string{
	"phoenix", "cortex", "helix", "quark", "nebula", "cipher",
	"vertex", "prism", "flux", "orbit", "nexus", "pulse",
	"vector", "spark", "echo", "relay", "sigma", "theta",
	"omega", "delta", "gamma", "vortex", "aurora", "comet",
	"zenith", "plasma", "photon", "axiom", "tensor", "matrix",
	"shard", "lumen",
}

// pickName returns an unused name from the pool. If all names are used,
// appends a numeric suffix to the first name in the pool.
func pickName(used map[string]bool) string {
	for _, name := range defaultNamePool {
		if !used[name] {
			return name
		}
	}
	// All names exhausted — append suffix.
	for i := 2; ; i++ {
		candidate := defaultNamePool[0] + "-" + itoa(i)
		if !used[candidate] {
			return candidate
		}
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
