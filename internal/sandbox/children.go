package sandbox

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"

	"github.com/jbeshir/demesne/internal/agents"
)

// childContext is the spawning context an agent run exposes to its
// in-sandbox demesne tools. Every agent run registers one (keyed by
// its jobID); children spawned from it inherit the same /in mounts and
// /workspace, and land in <outHost>/child/<name>. depth is advisory
// (there is intentionally no cap).
type childContext struct {
	inputVolumes  []opensandbox.Volume
	inputs        []agents.InputInfo
	workspaceHost string
	outHost       string
	depth         int

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
// internalAgentSpec.
type childSpawn struct {
	name   string
	parent *childContext
}

// reserveName validates name and records it, failing if another child
// of the same parent already used it. Uniqueness is enforced per
// parent (the workflowy spec requirement).
func (c *childContext) reserveName(name string) error {
	if err := validateChildName(name); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.usedNames == nil {
		c.usedNames = map[string]bool{}
	}
	if c.usedNames[name] {
		return fmt.Errorf("child name %q is already used in this sandbox", name)
	}
	c.usedNames[name] = true
	return nil
}

// priorSiblings returns a snapshot of the siblings recorded so far as a
// name -> outHost map. The copy is safe for the caller to iterate while
// later spawns mutate the original.
func (c *childContext) priorSiblings() map[string]string {
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
func (c *childContext) recordSibling(name, outHost string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.siblingOutputs == nil {
		c.siblingOutputs = map[string]string{}
	}
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

// validateChildName restricts names to a single safe path segment so
// the child output dir <parentOut>/child/<name> can't escape the
// parent's tree.
func validateChildName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 64 {
		return errors.New("name must be at most 64 characters")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
		default:
			return fmt.Errorf("invalid child name %q: only [A-Za-z0-9._-] are allowed", name)
		}
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid child name %q", name)
	}
	return nil
}

func (r *Runner) registerChild(jobID string, c *childContext) {
	r.childMu.Lock()
	defer r.childMu.Unlock()
	r.children[jobID] = c
}

func (r *Runner) deregisterChild(jobID string) {
	r.childMu.Lock()
	defer r.childMu.Unlock()
	delete(r.children, jobID)
}

func (r *Runner) lookupChild(jobID string) (*childContext, bool) {
	r.childMu.Lock()
	defer r.childMu.Unlock()
	c, ok := r.children[jobID]
	return c, ok
}
