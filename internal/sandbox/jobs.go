// Package sandbox — job lifecycle core.
//
// JobManager, Job, JobStatus, and their methods (Start / Status / Wait /
// Cancel) form the core of the async-orchestration primitive. A future
// adapter layer (MCP Tasks, SEP-1686) will sit on top of these four
// entry points; no code from that layer lives here yet — only the seam.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// jobTTL is how long a terminal job remains in the in-memory registry
// before the reaper deletes it.
const jobTTL = time.Hour

// reapInterval is how often the TTL reaper goroutine sweeps the registry.
const reapInterval = 5 * time.Minute

// defaultWaitTimeout is used when the caller passes timeout <= 0.
const defaultWaitTimeout = 30 * time.Second

// maxWaitTimeout is the server-side cap on sandbox_wait timeout.
const maxWaitTimeout = 120 * time.Second

// statusStdoutTailBytes is the maximum bytes of stdout tail returned by Status.
const statusStdoutTailBytes int64 = 16 * 1024

// Internal atomic state constants for a job's state field.
const (
	stateRunning   int32 = 0
	stateSucceeded int32 = 1
	stateFailed    int32 = 2
	stateCancelled int32 = 3
)

// JobStatus is the observable lifecycle state of a background job.
type JobStatus string

const (
	// JobStatusRunning means the job goroutine is still executing.
	JobStatusRunning JobStatus = "running"
	// JobStatusSucceeded means the run function returned without error.
	JobStatusSucceeded JobStatus = "succeeded"
	// JobStatusFailed means the run function returned an error or panicked.
	JobStatusFailed JobStatus = "failed"
	// JobStatusCancelled means Cancel was called and won the terminal-state race.
	JobStatusCancelled JobStatus = "cancelled"
)

// ErrJobNotFound is returned when an operation references an unknown JobID.
var ErrJobNotFound = errors.New("job not found")

// JobHooks are mid-run persistence checkpoints the JobManager sets on each
// run so it can write outHost/sandboxID to disk WHILE the job runs. This lets
// live Status calls read cost/stdout from disk and lets a restart identify the
// container via its sandboxID. There is deliberately NO end hook: the
// goroutine in JobManager.Start owns completion — it sets outcome/finishedAt/
// state after run() returns. Both callbacks may be nil. Self is populated by
// JobManager.Start before invoking run so the closure can stamp its own handle
// onto spawned children.
type JobHooks struct {
	// Self is the public JobID handle for the job executing with these hooks.
	Self JobID
	// OnOutputReady is called once the run has minted its run-job UUID and
	// output directory; runJobID is the internal uuid, outHost is the host
	// path of the job's /out directory, and resultsHost is the sidecar-results
	// dir where the proxy writes usage.json during the run (empty for scripts).
	OnOutputReady func(runJobID JobID, outHost, resultsHost string)
	// OnSandboxCreated is called once the underlying sandbox container has been
	// created; id is its runtime ID.
	OnSandboxCreated func(SandboxID)
}

// JobOutcome carries the result of a completed (succeeded or failed) job.
type JobOutcome struct {
	// ResultText is the human-readable result text surfaced to the MCP caller.
	ResultText string
	// OutputPath is the host path of the job's /out directory.
	OutputPath string
	// ExitCode is the sandbox process exit code.
	ExitCode int
	// CostUSD is the indicative API spend for this job's own run.
	CostUSD float64
	// TotalUsageUSD adds CostUSD and the spend of every descendant sandbox.
	TotalUsageUSD float64
}

// StatusResult is the response shape for JobManager.Status.
type StatusResult struct {
	// JobID is the public handle for the job.
	JobID JobID
	// Status is the current lifecycle state.
	Status JobStatus
	// ElapsedSeconds is wall time since the job was registered.
	ElapsedSeconds float64
	// StdoutTail is the last statusStdoutTailBytes of the job's stdout file.
	StdoutTail string
	// CostUSD is the incremental API spend read from outHost/usage.json.
	CostUSD float64
	// TotalUsageUSD is the total spend including descendants from results.json.
	TotalUsageUSD float64
	// ExitCode is populated once the job is terminal.
	ExitCode int
	// Message carries informational text (e.g. "still running").
	Message string
}

// WaitResult is the response shape for JobManager.Wait.
type WaitResult struct {
	// JobID is the public handle for the job.
	JobID JobID
	// Status is the lifecycle state at the point Wait returned.
	Status JobStatus
	// Message is set when Wait returns before the job is terminal.
	Message string
	// ResultText, OutputPath, ExitCode, CostUSD, TotalUsageUSD are populated
	// when Status is terminal (succeeded / failed / cancelled).
	ResultText    string
	OutputPath    string
	ExitCode      int
	CostUSD       float64
	TotalUsageUSD float64
}

// CancelResult is the response shape for JobManager.Cancel.
type CancelResult struct {
	// JobID is the public handle for the cancelled job.
	JobID JobID
	// Status is the lifecycle state after the cancel attempt.
	Status JobStatus
}

// job is the in-memory representation of one background job.
type job struct {
	id             JobID
	tool           string
	stdoutBasename string
	// state is the atomic lifecycle state; use stateXxx constants.
	state     int32
	cancel    context.CancelFunc // nil for disk-loaded jobs
	done      chan struct{}      // closed exactly once when the goroutine exits
	startedAt time.Time
	parent    JobID

	// mu protects all mutable fields below.
	mu          sync.Mutex
	runJobID    JobID
	outHost     string
	resultsHost string // sidecar-results dir; proxy writes usage.json here during run
	sandboxID   SandboxID
	finishedAt  time.Time
	outcome     JobOutcome
}

// JobManager manages the lifecycle of background jobs. Each job runs in its
// own goroutine whose context derives from the manager's root context, never
// from any request context, so jobs outlive the RPC that created them.
//
// MCP Tasks (SEP-1686) adapter is a future layer over Start/Status/Wait/Cancel.
type JobManager struct {
	mu         sync.RWMutex
	jobs       map[JobID]*job
	children   map[JobID][]JobID // parent → children
	rootCtx    context.Context
	rootCancel context.CancelFunc
	wg         sync.WaitGroup
	instanceID string // unique identity for this process: "<pid>-<startnanos>"
	jobsRoot   string // the shared .jobs root dir passed as the stateDir param
	stateDir   string // this instance's own subdir: <jobsRoot>/<instanceID>
	destroyer  func(context.Context, SandboxID) error
	now        func() time.Time
}

// jobStateToStatus converts an atomic state constant to the public JobStatus.
func jobStateToStatus(s int32) JobStatus {
	switch s {
	case stateSucceeded:
		return JobStatusSucceeded
	case stateFailed:
		return JobStatusFailed
	case stateCancelled:
		return JobStatusCancelled
	default:
		return JobStatusRunning
	}
}

// isTerminal reports whether the atomic state constant is a terminal state.
func isTerminal(s int32) bool {
	return s == stateSucceeded || s == stateFailed || s == stateCancelled
}

// stdoutBasenameForTool returns the basename of the file that captures
// the job's primary output, derived from the tool that launched it.
func stdoutBasenameForTool(tool string) string {
	switch tool {
	case ToolSandboxAgent, ToolSandboxResearch:
		return agentTranscriptBasename
	default:
		return "stdout.log"
	}
}

// newJobManager is the internal constructor that accepts a clock seam for
// testing. Production code uses NewJobManager.
func newJobManager(
	stateDir string,
	destroyer func(context.Context, SandboxID) error,
	now func() time.Time,
) *JobManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Capture process identity using the real OS PID and wall time, NOT the
	// injected clock. This is identity, not logical time: the actual PID and
	// nanosecond start time distinguish this instance from siblings and restarts.
	pid := os.Getpid()
	startNanos := time.Now().UnixNano()

	m := &JobManager{
		jobs:       make(map[JobID]*job),
		children:   make(map[JobID][]JobID),
		rootCtx:    ctx,
		rootCancel: cancel,
		destroyer:  destroyer,
		now:        now,
	}
	m.instanceID = fmt.Sprintf("%d-%d", pid, startNanos)
	m.jobsRoot = stateDir
	if stateDir != "" {
		m.stateDir = filepath.Join(stateDir, m.instanceID)
	}

	// Sweep stale instance dirs from previously crashed demesne processes.
	// Cross-restart recovery of job status by a NEW process is intentionally
	// dropped: a restarted demesne is a new instance; old job_ids →
	// ErrJobNotFound, which is honest. Orphaned CONTAINERS are still reaped by
	// ReapOrphans (separate path that does not touch this registry).
	if m.jobsRoot != "" {
		sweepStaleInstanceDirs(m.jobsRoot, m.instanceID, processAlive)
	}
	m.loadAndReconcile()
	go m.reapLoop()
	return m
}

// NewJobManager creates a new JobManager that persists state under stateDir.
// destroyer is called best-effort when cancelling a disk-loaded job that has
// no in-memory goroutine; pass r.Destroy wrapped in a closure.
func NewJobManager(stateDir string, destroyer func(context.Context, SandboxID) error) *JobManager {
	return newJobManager(stateDir, destroyer, time.Now)
}

// Shutdown cancels all running jobs and waits for their goroutines to exit.
func (m *JobManager) Shutdown() {
	m.rootCancel()
	m.wg.Wait()
	// Terminal state is already in memory; nothing cross-process needs the
	// instance subdir after all goroutines have exited.
	if m.stateDir != "" {
		if err := os.RemoveAll(m.stateDir); err != nil {
			log.Printf("sandbox: remove instance job dir %s: %v", m.stateDir, err)
		}
	}
}

// Start registers a new background job and launches it in a goroutine.
// parent is the JobID of the owning parent job, or "" for a root job.
// tool identifies the MCP tool (e.g. ToolSandboxAgent).
// run is the blocking function that performs the actual sandbox work.
//
// The returned JobID is the public opaque handle ("job-<uuid>"), distinct
// from the internal run-uuid that run() generates internally.
func (m *JobManager) Start(
	parent JobID,
	tool string,
	run func(ctx context.Context, h JobHooks) (JobOutcome, error),
) JobID {
	id := JobID("job-" + uuid.NewString())
	jobCtx, jobCancel := context.WithCancel(m.rootCtx)

	j := &job{
		id:             id,
		tool:           tool,
		stdoutBasename: stdoutBasenameForTool(tool),
		state:          stateRunning,
		cancel:         jobCancel,
		done:           make(chan struct{}),
		startedAt:      m.now(),
		parent:         parent,
	}

	// Register BEFORE launching the goroutine so an early Cancel finds the job.
	m.mu.Lock()
	m.jobs[id] = j
	if parent != "" {
		m.children[parent] = append(m.children[parent], id)
	}
	m.mu.Unlock()

	// Persist the initial running record outside the lock.
	if err := m.persistJobRecord(j); err != nil {
		log.Printf("sandbox: persist new job %s: %v", id, err)
	}

	hooks := JobHooks{
		Self: id,
		OnOutputReady: func(runJobID JobID, outHost, resultsHost string) {
			j.mu.Lock()
			j.runJobID = runJobID
			j.outHost = outHost
			j.resultsHost = resultsHost
			j.mu.Unlock()
			if err := m.persistJobRecord(j); err != nil {
				log.Printf("sandbox: persist job %s on output ready: %v", j.id, err)
			}
		},
		OnSandboxCreated: func(id SandboxID) {
			j.mu.Lock()
			j.sandboxID = id
			j.mu.Unlock()
			if err := m.persistJobRecord(j); err != nil {
				log.Printf("sandbox: persist job %s on sandbox created: %v", j.id, err)
			}
		},
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(j.done)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("sandbox: job %s panicked: %v", j.id, r)
				if atomic.CompareAndSwapInt32(&j.state, stateRunning, stateFailed) {
					j.mu.Lock()
					j.finishedAt = m.now()
					j.mu.Unlock()
					if err := m.persistJobRecord(j); err != nil {
						log.Printf("sandbox: persist job %s after panic: %v", j.id, err)
					}
				}
			}
		}()

		outcome, err := run(jobCtx, hooks)

		newState := stateSucceeded
		if err != nil {
			newState = stateFailed
		}
		if atomic.CompareAndSwapInt32(&j.state, stateRunning, newState) {
			j.mu.Lock()
			j.outcome = outcome
			j.finishedAt = m.now()
			j.mu.Unlock()
			if err := m.persistJobRecord(j); err != nil {
				log.Printf("sandbox: persist job %s on complete: %v", j.id, err)
			}
		}
	}()

	return id
}

// Status returns the current observable state of job id. Cost and stdout tail
// are read from the job's outHost on a best-effort basis; missing files
// return zero values rather than errors.
func (m *JobManager) Status(id JobID) (StatusResult, error) {
	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return StatusResult{}, fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}

	state := atomic.LoadInt32(&j.state)
	status := jobStateToStatus(state)

	j.mu.Lock()
	startedAt := j.startedAt
	outHost := j.outHost
	resultsHost := j.resultsHost
	stdoutBasename := j.stdoutBasename
	exitCode := j.outcome.ExitCode
	j.mu.Unlock()

	elapsed := m.now().Sub(startedAt).Seconds()
	res := StatusResult{
		JobID:          id,
		Status:         status,
		ElapsedSeconds: elapsed,
		ExitCode:       exitCode,
	}

	if outHost == "" {
		return res, nil
	}

	// While running with a live sidecar-results dir, read cost from there so
	// the proxy's live writes are visible before the job finishes.
	// When terminal, read from /out (copyUsageToOut wrote usage.json there,
	// and results.json carries the rolled-up total including descendants).
	if !isTerminal(state) && resultsHost != "" {
		usage := readUsageSnapshot(resultsHost)
		res.CostUSD = usage.CostUSD
	} else {
		usage := readUsageSnapshot(outHost)
		res.CostUSD = usage.CostUSD
		if r, ok := readResultsFile(outHost); ok {
			res.TotalUsageUSD = r.TotalUsageUSD
		}
	}

	tail, err := tailFile(filepath.Join(outHost, stdoutBasename), statusStdoutTailBytes)
	if err != nil {
		log.Printf("sandbox: status tail for job %s: %v", id, err)
	}
	res.StdoutTail = tail

	return res, nil
}

// Wait blocks until job id reaches a terminal state or the timeout elapses.
// timeout is clamped to [1ms, maxWaitTimeout]; zero or negative uses
// defaultWaitTimeout. When the timeout fires before the job completes, a
// non-error result with Status "running" is returned so the caller can poll
// again. ctx cancellation abandons the wait without affecting the job.
func (m *JobManager) Wait(ctx context.Context, id JobID, timeout time.Duration) (WaitResult, error) {
	if timeout <= 0 {
		timeout = defaultWaitTimeout
	}
	if timeout > maxWaitTimeout {
		timeout = maxWaitTimeout
	}

	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return WaitResult{}, fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}

	// Fast path: already terminal.
	if isTerminal(atomic.LoadInt32(&j.state)) {
		return m.buildWaitResult(j), nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-j.done:
		return m.buildWaitResult(j), nil
	case <-timer.C:
		return WaitResult{
			JobID:   id,
			Status:  JobStatusRunning,
			Message: "still running; call sandbox_wait again",
		}, nil
	case <-ctx.Done():
		return WaitResult{}, ctx.Err()
	}
}

// buildWaitResult reads the terminal outcome from j and returns a WaitResult.
// Called after j.done is closed (state + outcome are already set). For
// disk-loaded jobs (cancel == nil), outcome is zero-valued; recoverable
// fields are filled from the job's outHost.
func (m *JobManager) buildWaitResult(j *job) WaitResult {
	state := atomic.LoadInt32(&j.state)
	j.mu.Lock()
	outcome := j.outcome
	outHost := j.outHost
	j.mu.Unlock()

	wr := WaitResult{
		JobID:         j.id,
		Status:        jobStateToStatus(state),
		ResultText:    outcome.ResultText,
		OutputPath:    outcome.OutputPath,
		ExitCode:      outcome.ExitCode,
		CostUSD:       outcome.CostUSD,
		TotalUsageUSD: outcome.TotalUsageUSD,
	}

	// Disk-loaded jobs have no in-memory goroutine so outcome is zero-valued;
	// recover what we can from the job's outHost.
	if j.cancel == nil && outHost != "" {
		if wr.OutputPath == "" {
			wr.OutputPath = outHost
		}
		if r, ok := readResultsFile(outHost); ok {
			if wr.CostUSD == 0 {
				wr.CostUSD = r.OwnUsageUSD
			}
			if wr.TotalUsageUSD == 0 {
				wr.TotalUsageUSD = r.TotalUsageUSD
			}
		}
	}

	return wr
}

// Cancel requests cancellation of job id and all of its descendants
// (depth-first: children are cancelled before their parent). Cancel is
// idempotent: subsequent calls on an already-terminal job return the
// current status without error. An unknown id returns ErrJobNotFound.
func (m *JobManager) Cancel(ctx context.Context, id JobID) (CancelResult, error) {
	m.mu.RLock()
	_, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return CancelResult{}, fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}

	m.cancelSubtree(ctx, id)

	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	return CancelResult{
		JobID:  id,
		Status: jobStateToStatus(atomic.LoadInt32(&j.state)),
	}, nil
}

// cancelSubtree recursively cancels id and all of its registered children,
// depth-first (children are cancelled before their parent). Idempotent.
func (m *JobManager) cancelSubtree(ctx context.Context, id JobID) {
	// Snapshot children and the job pointer under read lock.
	m.mu.RLock()
	children := append([]JobID(nil), m.children[id]...)
	j := m.jobs[id]
	m.mu.RUnlock()

	if j == nil {
		return
	}

	// Recurse into children first (depth-first).
	for _, childID := range children {
		m.cancelSubtree(ctx, childID)
	}

	// Cancel this node: try to win the terminal-state race.
	if !atomic.CompareAndSwapInt32(&j.state, stateRunning, stateCancelled) {
		return // already terminal
	}
	j.mu.Lock()
	j.finishedAt = m.now()
	sandboxID := j.sandboxID
	j.mu.Unlock()

	if err := m.persistJobRecord(j); err != nil {
		log.Printf("sandbox: persist job %s on cancel: %v", id, err)
	}

	if j.cancel != nil {
		// Live job: cancel its context so the goroutine's existing deferred
		// sidecar.Remove + killSandbox teardown path runs. Do NOT duplicate
		// teardown here.
		j.cancel()
	} else if sandboxID != "" && m.destroyer != nil {
		// Fallback for a disk-loaded job that has a recorded sandbox ID but no
		// in-memory goroutine. Container teardown for running orphans after a
		// restart is normally handled by ReapOrphans in main; this path fires
		// if a Cancel is called for such a job before the reaper runs.
		if err := m.destroyer(ctx, sandboxID); err != nil {
			log.Printf("sandbox: destroy sandbox %s for cancelled job %s: %v",
				sandboxID, id, err)
		}
	}
}

// persistJobRecord snapshots the job's current state and writes it to disk.
// Must be called outside any mutex (snapshotting uses the job's own mu, not
// the registry mu).
func (m *JobManager) persistJobRecord(j *job) error {
	// Snapshot children while holding the registry read lock.
	m.mu.RLock()
	rawChildren := m.children[j.id]
	childIDs := make([]string, len(rawChildren))
	for i, c := range rawChildren {
		childIDs[i] = string(c)
	}
	m.mu.RUnlock()

	// Snapshot mutable job fields under the job's own lock.
	j.mu.Lock()
	runJobID := string(j.runJobID)
	outHost := j.outHost
	resultsHost := j.resultsHost
	sandboxID := string(j.sandboxID)
	parent := string(j.parent)
	exitCode := j.outcome.ExitCode
	updatedAt := m.now()
	j.mu.Unlock()

	state := atomic.LoadInt32(&j.state)

	rec := JobRecord{
		ID:          string(j.id),
		Tool:        j.tool,
		Status:      string(jobStateToStatus(state)),
		StartedAt:   j.startedAt,
		UpdatedAt:   updatedAt,
		RunJobID:    runJobID,
		OutHost:     outHost,
		ResultsHost: resultsHost,
		SandboxID:   sandboxID,
		Parent:      parent,
		ChildIDs:    childIDs,
		ExitCode:    exitCode,
	}

	return writeJobRecord(m.stateDir, rec)
}

// loadAndReconcile loads persisted job records from THIS instance's own subdir
// (m.stateDir). On a fresh PID the subdir does not exist yet, so this is
// defensive — normally a no-op. Any records whose status is "running" are
// marked "failed" (the goroutines that owned them are gone).
func (m *JobManager) loadAndReconcile() {
	if m.stateDir == "" {
		return
	}
	recs, err := loadJobs(m.stateDir)
	if err != nil {
		log.Printf("sandbox: load jobs: %v", err)
		return
	}

	recs = reconcileRunning(recs, m.stateDir, m.now())

	// Populate the in-memory registry. Disk-loaded jobs have no goroutine
	// (cancel == nil) and their done channel is pre-closed.
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rec := range recs {
		done := make(chan struct{})
		close(done)

		state := stateRunning
		switch JobStatus(rec.Status) {
		case JobStatusRunning:
			state = stateRunning
		case JobStatusSucceeded:
			state = stateSucceeded
		case JobStatusFailed:
			state = stateFailed
		case JobStatusCancelled:
			state = stateCancelled
		}

		j := &job{
			id:             JobID(rec.ID),
			tool:           rec.Tool,
			stdoutBasename: stdoutBasenameForTool(rec.Tool),
			state:          state,
			done:           done,
			startedAt:      rec.StartedAt,
			parent:         JobID(rec.Parent),
		}
		j.mu.Lock()
		j.runJobID = JobID(rec.RunJobID)
		j.outHost = rec.OutHost
		j.resultsHost = rec.ResultsHost
		j.sandboxID = SandboxID(rec.SandboxID)
		j.finishedAt = rec.UpdatedAt
		j.outcome.ExitCode = rec.ExitCode
		j.mu.Unlock()

		m.jobs[JobID(rec.ID)] = j
		if rec.Parent != "" {
			m.children[JobID(rec.Parent)] = append(
				m.children[JobID(rec.Parent)], JobID(rec.ID),
			)
		}
	}
}

// reapLoop is the background TTL reaper goroutine. It ticks every
// reapInterval and removes terminal jobs that have been finished longer
// than jobTTL. It exits when the manager's root context is cancelled.
func (m *JobManager) reapLoop() {
	ticker := time.NewTicker(reapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.rootCtx.Done():
			return
		case <-ticker.C:
			m.reapExpired()
		}
	}
}

// reapExpired deletes terminal jobs whose finishedAt is older than jobTTL.
func (m *JobManager) reapExpired() {
	cutoff := m.now().Add(-jobTTL)

	// Collect expired IDs under write lock; do NOT do file IO while holding mu.
	var expired []JobID
	m.mu.Lock()
	for id, j := range m.jobs {
		if !isTerminal(atomic.LoadInt32(&j.state)) {
			continue
		}
		j.mu.Lock()
		finished := j.finishedAt
		j.mu.Unlock()
		// Skip jobs whose finishedAt hasn't been recorded yet to avoid the
		// sub-microsecond window where state is terminal but finishedAt is zero.
		if !finished.IsZero() && finished.Before(cutoff) {
			delete(m.jobs, id)
			delete(m.children, id)
			expired = append(expired, id)
		}
	}
	m.mu.Unlock()

	// Delete disk records after releasing the lock.
	if m.stateDir == "" {
		return
	}
	for _, id := range expired {
		if err := removeJobRecord(m.stateDir, string(id)); err != nil {
			log.Printf("sandbox: remove reaped job record %s: %v", id, err)
		}
	}
}
