package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviousJobVolumes_Empty(t *testing.T) {
	assert.Nil(t, previousJobVolumes(nil))
	assert.Nil(t, previousJobVolumes(map[string]string{}))
}

func TestPreviousJobVolumes_SortedReadOnly(t *testing.T) {
	vols := previousJobVolumes(map[string]string{
		"beta":  "/out/job/child/beta",
		"alpha": "/out/job/child/alpha",
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
	assert.Equal(t, []string{"alpha", "beta"}, previousJobNames(map[string]string{
		"beta":  "/x",
		"alpha": "/y",
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
