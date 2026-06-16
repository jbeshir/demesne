package sandbox

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"

	"github.com/jbeshir/demesne/internal/agents"
)

// spawnContext is the spawning context an agent run exposes to its
// in-sandbox demesne tools. Every agent run registers one (keyed by
// its jobID); children spawned from it inherit the same /in mounts and
// /workspace, and land in <outHost>/child/<name>. depth is advisory
// (there is intentionally no cap).
type spawnContext struct {
	inputVolumes  []opensandbox.Volume
	inputs        []agents.InputInfo
	workspaceHost string
	outHost       string
	depth         int
	// bgJobID is the public JobManager handle for the background job that
	// owns this spawn context, or "" for blocking (non-background) parents.
	// Child background jobs read this to register themselves under the
	// correct parent in the job tree.
	bgJobID JobID

	mu        sync.Mutex
	usedNames map[string]bool
	// siblingOutputs records each spawned child's name -> outHost so a
	// later sibling can mount earlier ones' /out read-only under
	// /in/previous-jobs/<name>. Recorded once a child's output dir
	// exists; siblings spawn sequentially within a parent turn.
	siblingOutputs map[string]string
}

// childSpawn identifies a child being created: its name and the parent
// context it derives from. Passed through sandboxPrepOptions /
// internalAgentSpec. isolated requests a research-style child with no
// inherited /in inputs, no /in/previous-jobs, and a fresh private
// /workspace (only its /out/child/<name> links back to the parent).
type childSpawn struct {
	name     string
	parent   *spawnContext
	isolated bool
}

// reserveName validates name and records it, failing if another child
// of the same parent already used it. Uniqueness is enforced per
// parent (the workflowy spec requirement).
func (c *spawnContext) reserveName(name string) error {
	if err := validateChildName(name); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.usedNames[name] {
		return fmt.Errorf("child name %q is already used in this sandbox", name)
	}
	c.usedNames[name] = true
	return nil
}

// priorSiblings returns a snapshot of the siblings recorded so far as a
// name -> outHost map. The copy is safe for the caller to iterate while
// later spawns mutate the original.
func (c *spawnContext) priorSiblings() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.siblingOutputs))
	for name, host := range c.siblingOutputs {
		out[name] = host
	}
	return out
}

// recordSibling records a spawned child's name -> outHost so subsequent
// siblings can mount its /out under /in/previous-jobs/<name>.
func (c *spawnContext) recordSibling(name, outHost string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.siblingOutputs[name] = outHost
}

// previousJobNames returns the completed siblings' names, sorted, for
// the context-file note. Returns nil for the empty case.
func previousJobNames(siblings map[string]string) []string {
	if len(siblings) == 0 {
		return nil
	}
	names := make([]string, 0, len(siblings))
	for name := range siblings {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// previousJobVolumes turns a name -> outHost map of completed siblings
// into read-only mounts at /in/previous-jobs/<name>. Volume names are
// prefixed so they never collide with inherited /in input volumes.
// Returns nil for the empty case.
func previousJobVolumes(siblings map[string]string) []opensandbox.Volume {
	if len(siblings) == 0 {
		return nil
	}
	names := make([]string, 0, len(siblings))
	for name := range siblings {
		names = append(names, name)
	}
	sort.Strings(names)
	volumes := make([]opensandbox.Volume, 0, len(names))
	for _, name := range names {
		volumes = append(volumes, opensandbox.Volume{
			Name:      "prevjob-" + name,
			Host:      &opensandbox.Host{Path: siblings[name]},
			MountPath: "/in/previous-jobs/" + name,
			ReadOnly:  true,
		})
	}
	return volumes
}

// validateChildName restricts names to a lowercase DNS-1123-style label
// (lowercase letters, digits, and interior hyphens). The name is both a
// path segment (<parentOut>/child/<name>) and part of an OpenSandbox
// volume name (prevjob-<name>), which must be a valid DNS-1123 label —
// uppercase, '_', '.', or leading/trailing hyphens would produce an
// invalid volume name and break the spawn (and, via previous-jobs, every
// later sibling). Capped so prevjob-<name> stays within the 63-char limit.
func validateChildName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 40 {
		return errors.New("name must be at most 40 characters")
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-' && i > 0 && i < len(name)-1:
			// interior hyphen only — not first or last
		default:
			return fmt.Errorf(
				"invalid child name %q: use lowercase letters, digits, and interior hyphens only", name)
		}
	}
	return nil
}

// ChildRegistry is the live-run identity store: maps a running agent
// sandbox's jobID to the spawning context its in-sandbox demesne tools
// use. Safe for concurrent access.
type ChildRegistry struct {
	mu      sync.Mutex
	entries map[JobID]*spawnContext
}

func newChildRegistry() *ChildRegistry {
	return &ChildRegistry{entries: map[JobID]*spawnContext{}}
}

func (cr *ChildRegistry) Register(jobID JobID, c *spawnContext) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.entries[jobID] = c
}

func (cr *ChildRegistry) Lookup(jobID JobID) (*spawnContext, bool) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	c, ok := cr.entries[jobID]
	return c, ok
}

func (cr *ChildRegistry) Deregister(jobID JobID) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	delete(cr.entries, jobID)
}
