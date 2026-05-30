package codex

import (
	"errors"
	"fmt"

	"github.com/jbeshir/demesne/internal/agents"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
)

// ModelName is an alias for agents.ModelName so callers in this package
// can write ModelGPT55/ModelGPT54Mini/etc. without an extra import.
type ModelName = agents.ModelName

// Model constants for Codex model IDs validated against the live Codex CLI
// on ChatGPT-account billing. gpt-5.5-pro and gpt-5.4-nano are rejected by
// the backend with "not supported when using Codex with a ChatGPT account".
const (
	ModelGPT55     ModelName = "gpt-5.5"
	ModelGPT54Mini ModelName = "gpt-5.4-mini"
)

// Models is the Codex model whitelist exposed via sandbox_agent's
// `model` parameter. Derived from proxyopenai.Aliases() so the whitelist
// stays in sync with the pricing catalog.
var Models = func() []ModelName {
	a := proxyopenai.Aliases()
	out := make([]ModelName, len(a))
	for i, s := range a {
		out[i] = ModelName(s)
	}
	return out
}()

// DefaultModel is the model used when the caller does not specify one.
// Must equal the alias at index 0 of proxyopenai's modelCatalog.
const DefaultModel ModelName = ModelGPT55

// ErrUnknownModel is the sentinel wrapped by ResolveModel when the
// requested model is not in the whitelist. Use errors.Is to distinguish
// this from operational errors without inspecting the text.
var ErrUnknownModel = errors.New("is not in the Codex whitelist")

// ResolveModel validates a model name against Models. Empty input
// resolves to DefaultModel.
func ResolveModel(name string) (ModelName, error) {
	if name == "" {
		return DefaultModel, nil
	}
	for _, m := range Models {
		if string(m) == name {
			return m, nil
		}
	}
	return "", fmt.Errorf("model %q %w (%v)", name, ErrUnknownModel, Models)
}
