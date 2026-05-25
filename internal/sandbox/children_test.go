package sandbox

import (
	"path/filepath"
	"testing"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testChildAlpha = "alpha"
	testChildBeta  = "beta"
)

func TestPreviousJobVolumes_Empty(t *testing.T) {
	assert.Nil(t, previousJobVolumes(nil))
	assert.Nil(t, previousJobVolumes(map[string]string{}))
}

func TestPreviousJobVolumes_SortedReadOnly(t *testing.T) {
	vols := previousJobVolumes(map[string]string{
		testChildBeta:  "/out/job/child/" + testChildBeta,
		testChildAlpha: "/out/job/child/" + testChildAlpha,
	})
	require.Len(t, vols, 2)

	// Deterministic, name-sorted order.
	assert.Equal(t, "prevjob-alpha", vols[0].Name)
	assert.Equal(t, "/in/previous-jobs/alpha", vols[0].MountPath)
	assert.Equal(t, "/out/job/child/alpha", vols[0].Host.Path)
	assert.True(t, vols[0].ReadOnly)

	assert.Equal(t, "prevjob-beta", vols[1].Name)
	assert.Equal(t, "/in/previous-jobs/beta", vols[1].MountPath)
	assert.Equal(t, "/out/job/child/beta", vols[1].Host.Path)
	assert.True(t, vols[1].ReadOnly)
}

func TestPreviousJobNames_SortedAndEmpty(t *testing.T) {
	assert.Nil(t, previousJobNames(nil))
	assert.Equal(t, []string{testChildAlpha, testChildBeta}, previousJobNames(map[string]string{
		testChildBeta:  "/x",
		testChildAlpha: "/y",
	}))
}

func TestRecordSibling_SnapshotIndependence(t *testing.T) {
	c := &childContext{}

	// First child sees no prior siblings.
	assert.Empty(t, c.priorSiblings())
	c.recordSibling("first", "/out/job/child/first")

	// Snapshot taken now reflects only "first".
	snap := c.priorSiblings()
	require.Equal(t, map[string]string{"first": "/out/job/child/first"}, snap)

	// Recording a later sibling does not mutate the earlier snapshot.
	c.recordSibling("second", "/out/job/child/second")
	assert.Equal(t, map[string]string{"first": "/out/job/child/first"}, snap)
	assert.Len(t, c.priorSiblings(), 2)
}

func TestBuildChildLayout_IsolatedVsInherited(t *testing.T) {
	r := NewRunner(Config{OutputRoot: t.TempDir()})
	parentOut := t.TempDir()
	parent := &childContext{
		inputVolumes:   []opensandbox.Volume{{Name: "in-0", MountPath: "/in/x"}},
		workspaceHost:  "/parent/workspace",
		outHost:        parentOut,
		usedNames:      map[string]bool{},
		siblingOutputs: map[string]string{"earlier": "/some/out"},
	}

	// Isolated (research): no inherited inputs, no previous-jobs, a fresh
	// private workspace, but /out still nests under the parent.
	iso, err := r.buildChildLayout(&childSpawn{name: "research1", parent: parent, isolated: true})
	require.NoError(t, err)
	assert.Empty(t, iso.inputVolumes)
	assert.Empty(t, iso.previousJobs)
	assert.NotEqual(t, parent.workspaceHost, iso.workspaceHost)
	assert.DirExists(t, iso.workspaceHost)
	assert.Equal(t, filepath.Join(parentOut, "child", "research1"), iso.outHost)

	// Inherited (agent/script): shares /in, /workspace, and previous-jobs.
	// (Siblings are recorded after a successful create, not in
	// buildChildLayout, so only the pre-seeded "earlier" is visible here.)
	inh, err := r.buildChildLayout(&childSpawn{name: "impl1", parent: parent})
	require.NoError(t, err)
	assert.Equal(t, parent.inputVolumes, inh.inputVolumes)
	assert.Equal(t, parent.workspaceHost, inh.workspaceHost)
	assert.Contains(t, inh.previousJobs, "earlier")
}
