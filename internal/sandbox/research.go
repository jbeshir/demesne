package sandbox

import "context"

// Research runs an agent with no input mounts and unrestricted
// outbound internet egress against the caller's prompt — the
// long-running research variant of Agent. The agent-vendor proxy
// stays in front of the model API.
//
// Research has no Files/Directories or Egress knobs by design: the
// MCP tool surface refuses to combine read-only host inputs with
// open egress (that combination is the data-exfiltration shape we
// want kept off the surface). Callers that need inputs use Agent
// instead.
func (r *Runner) Research(ctx context.Context, req ResearchRequest) (ResearchResult, error) {
	spec := internalAgentSpec{
		agentName: req.Agent,
		model:     req.Model,
		prompt:    req.Prompt,
		preamble:  req.Preamble,
		egress:    EgressOpen,
		tool:      "sandbox_research",
	}
	res, err := r.runAgent(ctx, spec)
	if err != nil {
		return ResearchResult{}, err
	}
	return ResearchResult(res), nil
}
