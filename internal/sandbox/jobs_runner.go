package sandbox

import (
	"context"
	"time"
)

// startScriptJob registers a background script job with the JobManager.
// parent is the owning background job, or "" for a root job.
func (r *Runner) startScriptJob(req ScriptRequest, child *childSpawn, parent JobID) JobID {
	run := func(ctx context.Context, h JobHooks) (JobOutcome, error) {
		res, err := r.runScript(ctx, req, child, h)
		if err != nil {
			return JobOutcome{}, err
		}
		return JobOutcome{
			ResultText: res.Stdout,
			OutputPath: res.OutputPath,
			ExitCode:   res.ExitCode,
		}, nil
	}
	return r.jobs.Start(parent, ToolSandboxScript, run)
}

// startAgentJob registers a background agent job with the JobManager.
// The closure captures spec and overwrites the hook fields from the
// JobHooks the manager supplies at run time. parent is the owning
// background job, or "" for a root job.
func (r *Runner) startAgentJob(spec internalAgentSpec, parent JobID) JobID {
	run := func(ctx context.Context, h JobHooks) (JobOutcome, error) {
		s := spec
		s.onOutputReady = h.OnOutputReady
		s.bgSelf = h.Self
		res, err := r.runAgent(ctx, s)
		if err != nil {
			return JobOutcome{}, err
		}
		return JobOutcome{
			ResultText:    res.Stdout,
			OutputPath:    res.OutputPath,
			ExitCode:      res.ExitCode,
			CostUSD:       res.CostUSD,
			TotalUsageUSD: res.TotalUsageUSD,
		}, nil
	}
	return r.jobs.Start(parent, spec.tool, run)
}

// StartScript starts a sandbox_script run in the background and returns
// its public JobID handle immediately. Use Status/Wait to poll and
// Cancel to abort.
func (r *Runner) StartScript(req ScriptRequest) JobID {
	return r.startScriptJob(req, nil, "")
}

// StartAgent starts a sandbox_agent run in the background and returns
// its public JobID handle immediately.
func (r *Runner) StartAgent(req AgentRequest) JobID {
	spec := internalAgentSpec{
		model:           req.Model,
		prompt:          req.Prompt,
		preamble:        req.Preamble,
		files:           req.Files,
		directories:     req.Directories,
		egress:          egressOrDefault(req.Egress, EgressNone),
		tool:            ToolSandboxAgent,
		outputPath:      req.OutputPath,
		outputFormat:    req.OutputFormat,
		successCriteria: req.SuccessCriteria,
	}
	return r.startAgentJob(spec, "")
}

// StartResearch starts a sandbox_research run in the background and
// returns its public JobID handle immediately.
func (r *Runner) StartResearch(req ResearchRequest) JobID {
	spec := internalAgentSpec{
		model:           req.Model,
		prompt:          req.Prompt,
		preamble:        req.Preamble,
		egress:          EgressOpen,
		tool:            ToolSandboxResearch,
		outputPath:      req.OutputPath,
		outputFormat:    req.OutputFormat,
		successCriteria: req.SuccessCriteria,
	}
	return r.startAgentJob(spec, "")
}

// Status returns the current observable state of a background job.
func (r *Runner) Status(req StatusRequest) (StatusResult, error) {
	return r.jobs.Status(req.JobID)
}

// Wait blocks until the background job reaches a terminal state or the
// timeout in req elapses. ctx cancellation abandons the wait without
// affecting the job itself.
func (r *Runner) Wait(ctx context.Context, req WaitRequest) (WaitResult, error) {
	return r.jobs.Wait(ctx, req.JobID, time.Duration(req.TimeoutSeconds)*time.Second)
}

// Cancel requests cancellation of the background job and all of its
// descendants (depth-first). Idempotent on already-terminal jobs.
func (r *Runner) Cancel(ctx context.Context, req CancelRequest) (CancelResult, error) {
	return r.jobs.Cancel(ctx, req.JobID)
}
