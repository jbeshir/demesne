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
const defaultWaitTimeout = 30 * time.Minute

// maxWaitTimeout is the server-side cap on sandbox_wait timeout.
const maxWaitTimeout = 48 * time.Hour

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

// JobHooks carries the mid-run callback and the job's own handle. There is
// deliberately NO end hook: the goroutine in JobManager.Start owns completion
// — it sets outcome/finishedAt/state after run() returns. OnOutputReady is
// the single hook: it records outHost/resultsHost as in-memory fields for live
// Status reads. Self is populated before run so the closure can stamp its own
// handle onto spawned children.
type JobHooks struct {
	// Self is the public JobID handle for the job executing with these hooks.
	Self JobID
	// OnOutputReady is called once the run has minted its output directory;
	// outHost is the host path of the job's /out directory and resultsHost is
	// the sidecar-results dir where the proxy writes usage.json during the run
	// (empty for scripts).
	OnOutputReady func(outHost, resultsHost string)
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

// TerminalNotifier is called once after a background job wins its transition
// to a terminal state. Delivery is advisory: implementations must return
// promptly and handle their own errors without changing job state.
type TerminalNotifier func(JobID, JobStatus)

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
	cancel    context.CancelFunc
	done      chan struct{} // closed exactly once when the goroutine exits
	startedAt time.Time
	parent    JobID
	notify    TerminalNotifier

	// mu protects all mutable fields below.
	mu          sync.Mutex
	outHost     string
	resultsHost string // sidecar-results dir; proxy writes usage.json here during run
	finishedAt  time.Time
	outcome     JobOutcome
}

// JobManager manages the lifecycle of background jobs. Each job runs in its
// own goroutine whose context derives from the manager's root context, never
// from any request context, so jobs outlive the RPC that created them.
//
// Jobs are held in memory only; they do not survive a process restart.
// MCP Tasks (SEP-1686) adapter is a future layer over Start/Status/Wait/Cancel.
type JobManager struct {
	mu         sync.RWMutex
	jobs       map[JobID]*job
	children   map[JobID][]JobID // parent → children
	rootCtx    context.Context
	rootCancel context.CancelFunc
	wg         sync.WaitGroup
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
func newJobManager(now func() time.Time) *JobManager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &JobManager{
		jobs:       make(map[JobID]*job),
		children:   make(map[JobID][]JobID),
		rootCtx:    ctx,
		rootCancel: cancel,
		now:        now,
	}

	go m.reapLoop()
	return m
}

// NewJobManager creates a new in-memory-only JobManager. Jobs do not survive
// a process restart; orphaned containers are reaped by ReapOrphans.
func NewJobManager() *JobManager {
	return newJobManager(time.Now)
}

// Shutdown cancels all running jobs and waits for their goroutines to exit.
func (m *JobManager) Shutdown() {
	m.rootCancel()
	m.wg.Wait()
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
	notify TerminalNotifier,
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
		notify:         notify,
	}

	// Register BEFORE launching the goroutine so an early Cancel finds the job.
	m.mu.Lock()
	m.jobs[id] = j
	if parent != "" {
		m.children[parent] = append(m.children[parent], id)
	}
	m.mu.Unlock()

	hooks := JobHooks{
		Self: id,
		OnOutputReady: func(outHost, resultsHost string) {
			j.mu.Lock()
			j.outHost = outHost
			j.resultsHost = resultsHost
			j.mu.Unlock()
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
					if j.notify != nil {
						j.notify(j.id, JobStatusFailed)
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
			if j.notify != nil {
				j.notify(j.id, jobStateToStatus(newState))
			}
		}
	}()

	return id
}

// Status returns the current observable state of job id. Cost and, when
// requested, the stdout tail are read from the job's outHost on a best-effort
// basis; missing files return zero values rather than errors.
func (m *JobManager) Status(id JobID, includeStdoutTail bool) (StatusResult, error) {
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

	if includeStdoutTail {
		tail, err := tailFile(filepath.Join(outHost, stdoutBasename), statusStdoutTailBytes)
		if err != nil {
			log.Printf("sandbox: status tail for job %s: %v", id, err)
		}
		res.StdoutTail = tail
	}

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

	// Fast path: already terminal. Wait for j.done so the goroutine has
	// finished writing j.outcome (state flips terminal before the outcome
	// write; done is closed after it).
	if isTerminal(atomic.LoadInt32(&j.state)) {
		<-j.done
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
// Called after j.done is closed (state + outcome are already set). Terminal
// jobs carry their full outcome in memory.
func (m *JobManager) buildWaitResult(j *job) WaitResult {
	state := atomic.LoadInt32(&j.state)
	j.mu.Lock()
	outcome := j.outcome
	j.mu.Unlock()

	return WaitResult{
		JobID:         j.id,
		Status:        jobStateToStatus(state),
		ResultText:    outcome.ResultText,
		OutputPath:    outcome.OutputPath,
		ExitCode:      outcome.ExitCode,
		CostUSD:       outcome.CostUSD,
		TotalUsageUSD: outcome.TotalUsageUSD,
	}
}

// Cancel requests cancellation of job id and all of its descendants
// (depth-first: children are cancelled before their parent). Cancel is
// idempotent: subsequent calls on an already-terminal job return the
// current status without error. An unknown id returns ErrJobNotFound.
func (m *JobManager) Cancel(ctx context.Context, id JobID) (CancelResult, error) {
	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return CancelResult{}, fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}

	m.cancelSubtree(id)

	return CancelResult{
		JobID:  id,
		Status: jobStateToStatus(atomic.LoadInt32(&j.state)),
	}, nil
}

// cancelSubtree recursively cancels id and all of its registered children,
// depth-first (children are cancelled before their parent). Idempotent.
func (m *JobManager) cancelSubtree(id JobID) {
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
		m.cancelSubtree(childID)
	}

	// Cancel this node: try to win the terminal-state race.
	if !atomic.CompareAndSwapInt32(&j.state, stateRunning, stateCancelled) {
		return // already terminal
	}
	j.mu.Lock()
	j.finishedAt = m.now()
	j.mu.Unlock()

	j.cancel()
	if j.notify != nil {
		j.notify(j.id, JobStatusCancelled)
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
		}
	}
	m.mu.Unlock()
}
