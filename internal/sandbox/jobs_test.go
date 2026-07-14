package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestManager returns a JobManager wired with a test clock.
func makeTestManager(t *testing.T) (*JobManager, *atomic.Int64) {
	t.Helper()
	var tick atomic.Int64
	now := func() time.Time {
		return time.Unix(tick.Load(), 0)
	}
	m := newJobManager(now)
	t.Cleanup(m.Shutdown)
	return m, &tick
}

// syncRun returns a run func that blocks on release, then returns outcome.
func syncRun(
	release <-chan struct{},
	outcome JobOutcome,
	runErr error,
) func(context.Context, JobHooks) (JobOutcome, error) {
	return func(_ context.Context, _ JobHooks) (JobOutcome, error) {
		<-release
		return outcome, runErr
	}
}

// TestJobStartStatusSucceed verifies the basic happy path: start → poll status
// (running) → release → poll status (succeeded) → outcome matches.
func TestJobStartStatusSucceed(t *testing.T) {
	m, _ := makeTestManager(t)

	release := make(chan struct{})
	want := JobOutcome{ResultText: "ok", ExitCode: 0}
	id := m.Start("", ToolSandboxScript, syncRun(release, want, nil))

	// Status while running.
	s, err := m.Status(id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusRunning, s.Status)
	assert.Equal(t, id, s.JobID)

	// Let the run finish.
	close(release)

	// Wait for goroutine to complete.
	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	require.NotNil(t, j)
	<-j.done

	s, err = m.Status(id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusSucceeded, s.Status)
}

// TestJobFailedOnError verifies that a run func returning an error sets
// status to failed.
func TestJobFailedOnError(t *testing.T) {
	m, _ := makeTestManager(t)

	release := make(chan struct{})
	id := m.Start("", ToolSandboxScript, syncRun(release, JobOutcome{}, errors.New("boom")))
	close(release)

	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	<-j.done

	s, err := m.Status(id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusFailed, s.Status)
}

// TestJobCancelIdempotent verifies that cancelling an already-cancelled job
// is idempotent and does not return an error.
func TestJobCancelIdempotent(t *testing.T) {
	m, _ := makeTestManager(t)

	block := make(chan struct{})
	id := m.Start("", ToolSandboxScript, func(ctx context.Context, _ JobHooks) (JobOutcome, error) {
		<-ctx.Done()
		return JobOutcome{}, ctx.Err()
	})
	_ = block

	res, err := m.Cancel(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCancelled, res.Status)

	// Second cancel must succeed without error.
	res2, err := m.Cancel(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCancelled, res2.Status)
}

// TestJobCancelUnknown verifies that cancelling an unknown job returns
// ErrJobNotFound.
func TestJobCancelUnknown(t *testing.T) {
	m, _ := makeTestManager(t)
	_, err := m.Cancel(context.Background(), JobID("job-does-not-exist"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrJobNotFound)
}

// TestJobStatusUnknown verifies that Status on an unknown job returns
// ErrJobNotFound.
func TestJobStatusUnknown(t *testing.T) {
	m, _ := makeTestManager(t)
	_, err := m.Status(JobID("job-no-such"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrJobNotFound)
}

// TestJobCancelRecursesChildrenDepthFirst verifies that when a parent job is
// cancelled, its children are cancelled before the parent, depth-first.
//
// We inject spy cancel functions directly into job structs rather than relying
// on goroutine scheduling: cancelSubtree is synchronous so the spy-recording
// order is fully deterministic.
func TestJobCancelRecursesChildrenDepthFirst(t *testing.T) {
	m, _ := makeTestManager(t)

	var cancelOrder []JobID
	parentID := JobID("job-parent-spy")
	childID := JobID("job-child-spy")

	// Pre-close done channels: no real goroutines run in this test.
	parentDone := make(chan struct{})
	childDone := make(chan struct{})
	close(parentDone)
	close(childDone)

	parentJob := &job{
		id:        parentID,
		state:     stateRunning,
		cancel:    func() { cancelOrder = append(cancelOrder, parentID) },
		done:      parentDone,
		startedAt: m.now(),
	}
	childJob := &job{
		id:        childID,
		state:     stateRunning,
		cancel:    func() { cancelOrder = append(cancelOrder, childID) },
		done:      childDone,
		startedAt: m.now(),
		parent:    parentID,
	}

	m.mu.Lock()
	m.jobs[parentID] = parentJob
	m.jobs[childID] = childJob
	m.children[parentID] = []JobID{childID}
	m.mu.Unlock()

	_, err := m.Cancel(context.Background(), parentID)
	require.NoError(t, err)

	require.Len(t, cancelOrder, 2, "both jobs should have been cancelled")
	assert.Equal(t, childID, cancelOrder[0], "child must be cancelled before parent (depth-first)")
	assert.Equal(t, parentID, cancelOrder[1], "parent must be cancelled after child")
}

// TestJobWaitTimeoutReturnsRunning verifies that Wait returns a running result
// with the expected message when the job does not finish within the timeout.
func TestJobWaitTimeoutReturnsRunning(t *testing.T) {
	m, _ := makeTestManager(t)

	block := make(chan struct{}) // never released in this test
	id := m.Start("", ToolSandboxScript, func(ctx context.Context, _ JobHooks) (JobOutcome, error) {
		select {
		case <-block:
		case <-ctx.Done():
		}
		return JobOutcome{}, nil
	})
	defer close(block)

	res, err := m.Wait(context.Background(), id, 10*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, JobStatusRunning, res.Status)
	assert.Contains(t, res.Message, "still running")
}

// TestJobWaitReturnsTerminal verifies that Wait returns the terminal status
// (with outcome) once the job finishes.
func TestJobWaitReturnsTerminal(t *testing.T) {
	m, _ := makeTestManager(t)

	release := make(chan struct{})
	want := JobOutcome{ResultText: "done", ExitCode: 42}
	id := m.Start("", ToolSandboxScript, syncRun(release, want, nil))
	close(release)

	res, err := m.Wait(context.Background(), id, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, JobStatusSucceeded, res.Status)
	assert.Equal(t, want.ResultText, res.ResultText)
	assert.Equal(t, want.ExitCode, res.ExitCode)
}

// TestJobWaitUnknown verifies that Wait on an unknown job returns ErrJobNotFound.
func TestJobWaitUnknown(t *testing.T) {
	m, _ := makeTestManager(t)
	_, err := m.Wait(context.Background(), JobID("job-no-such"), time.Second)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrJobNotFound)
}

func TestJobWaitCancellationDoesNotCancelJob(t *testing.T) {
	m, _ := makeTestManager(t)
	release := make(chan struct{})
	id := m.Start("", ToolSandboxScript, syncRun(release, JobOutcome{}, nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.Wait(ctx, id, time.Second)
	require.ErrorIs(t, err, context.Canceled)
	status, err := m.Status(id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusRunning, status.Status)
	close(release)
}

func TestJobStartDoesNotInheritRequestCancellation(t *testing.T) {
	m, _ := makeTestManager(t)
	reached := make(chan struct{})
	id := m.Start("", ToolSandboxScript, func(ctx context.Context, _ JobHooks) (JobOutcome, error) {
		close(reached)
		<-ctx.Done()
		return JobOutcome{}, ctx.Err()
	})
	select {
	case <-reached:
	case <-time.After(time.Second):
		t.Fatal("background job did not start")
	}
	status, err := m.Status(id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusRunning, status.Status)
}

// TestJobTTLReap verifies that terminal jobs are deleted from the registry
// once their finishedAt is older than jobTTL.
func TestJobTTLReap(t *testing.T) {
	m, tick := makeTestManager(t)

	release := make(chan struct{})
	id := m.Start("", ToolSandboxScript, syncRun(release, JobOutcome{}, nil))
	close(release)

	// Wait for completion.
	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	<-j.done

	// Advance clock well past TTL.
	tick.Store(int64(jobTTL/time.Second + 10))

	// Trigger reap directly.
	m.reapExpired()

	// Job should be gone.
	m.mu.RLock()
	_, found := m.jobs[id]
	m.mu.RUnlock()
	assert.False(t, found, "expected job to be reaped after TTL")
}

// TestJobTTLNotReapedEarly verifies that a terminal job is NOT reaped before
// jobTTL elapses.
func TestJobTTLNotReapedEarly(t *testing.T) {
	m, tick := makeTestManager(t)

	release := make(chan struct{})
	id := m.Start("", ToolSandboxScript, syncRun(release, JobOutcome{}, nil))
	close(release)

	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	<-j.done

	// Advance clock less than TTL.
	tick.Store(int64(jobTTL/time.Second - 1))
	m.reapExpired()

	m.mu.RLock()
	_, found := m.jobs[id]
	m.mu.RUnlock()
	assert.True(t, found, "expected job to still be present before TTL")
}

// TestJobHooksOnOutputReady verifies that the OnOutputReady hook is called with
// the outHost the run function provides.
func TestJobHooksOnOutputReady(t *testing.T) {
	m, _ := makeTestManager(t)

	var gotOutHost string
	var wg sync.WaitGroup
	wg.Add(1)

	id := m.Start("", ToolSandboxScript,
		func(_ context.Context, h JobHooks) (JobOutcome, error) {
			if h.OnOutputReady != nil {
				h.OnOutputReady("/tmp/out", "")
				gotOutHost = "/tmp/out"
			}
			wg.Done()
			return JobOutcome{}, nil
		},
	)

	wg.Wait()
	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	<-j.done

	assert.Equal(t, "/tmp/out", gotOutHost)
}

// TestJobHooksOnOutputReadyRecordsFields verifies that invoking the
// OnOutputReady hook stores outHost into the job's internal fields, and that a
// subsequent Status call succeeds (outHost is non-empty so Status attempts file
// reads; missing files are silently tolerated).
func TestJobHooksOnOutputReadyRecordsFields(t *testing.T) {
	m, _ := makeTestManager(t)

	const wantOutHost = "/tmp/demesne-test/out"

	var wg sync.WaitGroup
	wg.Add(1)

	id := m.Start("", ToolSandboxScript,
		func(_ context.Context, h JobHooks) (JobOutcome, error) {
			if h.OnOutputReady != nil {
				h.OnOutputReady(wantOutHost, "")
			}
			wg.Done()
			return JobOutcome{}, nil
		},
	)

	wg.Wait()
	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	require.NotNil(t, j)
	<-j.done

	j.mu.Lock()
	outHost := j.outHost
	j.mu.Unlock()

	assert.Equal(t, wantOutHost, outHost)

	// Status must succeed even when outHost points to a non-existent path
	// (files are read best-effort; missing files produce zero values, not errors).
	_, err := m.Status(id)
	require.NoError(t, err)
}

// TestJobPanicRecoveredAsFailed verifies that a panicking run func transitions
// the job to failed rather than leaving it in running state.
func TestJobPanicRecoveredAsFailed(t *testing.T) {
	m, _ := makeTestManager(t)

	id := m.Start("", ToolSandboxScript, func(_ context.Context, _ JobHooks) (JobOutcome, error) {
		panic("test panic")
	})

	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	<-j.done

	s, err := m.Status(id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusFailed, s.Status)
}

// TestJobWaitClampsTimeout verifies that a negative or zero timeout is
// clamped to defaultWaitTimeout (job finishes immediately here, so
// we just verify no error occurs and result is terminal).
func TestJobWaitClampsTimeout(t *testing.T) {
	m, _ := makeTestManager(t)
	release := make(chan struct{})
	id := m.Start("", ToolSandboxScript, syncRun(release, JobOutcome{}, nil))
	close(release)

	res, err := m.Wait(context.Background(), id, 0)
	require.NoError(t, err)
	assert.Equal(t, JobStatusSucceeded, res.Status)
}

// TestStatusIncrementalCostFromResultsHost verifies that Status surfaces a
// non-zero CostUSD from a resultsHost usage.json while the job is still running.
func TestStatusIncrementalCostFromResultsHost(t *testing.T) {
	m, _ := makeTestManager(t)

	// Create a temp dir as the sidecar-results dir and write a fake usage.json.
	resultsHost := t.TempDir()
	const wantCost = 0.042
	usageJSON := fmt.Sprintf(`{"cost_usd":%f}`, wantCost)
	require.NoError(t, os.WriteFile(filepath.Join(resultsHost, "usage.json"), []byte(usageJSON), 0o600))

	outHost := t.TempDir()
	block := make(chan struct{})
	id := m.Start("", ToolSandboxAgent,
		func(_ context.Context, h JobHooks) (JobOutcome, error) {
			if h.OnOutputReady != nil {
				h.OnOutputReady(outHost, resultsHost)
			}
			<-block
			return JobOutcome{}, nil
		},
	)
	defer close(block)

	// Poll until the hook has fired and Status returns the incremental cost.
	require.Eventually(t, func() bool {
		s, err := m.Status(id)
		return err == nil && s.CostUSD > 0
	}, time.Second, 5*time.Millisecond, "expected non-zero CostUSD from resultsHost")

	s, err := m.Status(id)
	require.NoError(t, err)
	assert.Equal(t, JobStatusRunning, s.Status)
	assert.InDelta(t, wantCost, s.CostUSD, 1e-6)
}
