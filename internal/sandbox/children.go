package sandbox

import (
	"errors"
	"fmt"
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
