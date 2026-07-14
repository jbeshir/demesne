package codex

import (
	"errors"
	"fmt"

	"github.com/jbeshir/demesne/internal/agents"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
)

// ModelName is an alias for agents.ModelName so callers in this package
// can write ModelGPT56Sol/ModelGPT55/etc. without an extra import.
type ModelName = agents.ModelName

// Model constants for Codex model IDs validated against the live Codex CLI
// on ChatGPT-account billing. Unsupported variants are rejected by the backend
// with "not supported when using Codex with a ChatGPT account".
const (
	ModelGPT56Sol   ModelName = "gpt-5.6-sol"
	ModelGPT56Terra ModelName = "gpt-5.6-terra"
	ModelGPT56Luna  ModelName = "gpt-5.6-luna"
	ModelGPT55      ModelName = "gpt-5.5"
	ModelGPT54Mini  ModelName = "gpt-5.4-mini"
)

// Models is the Codex model allowlist exposed via sandbox_agent's
// `model` parameter. Derived from proxyopenai.Aliases() so the allowlist
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
const DefaultModel ModelName = ModelGPT56Sol

// ErrUnknownModel is the sentinel wrapped by ResolveModel when the
// requested model is not in the allowlist. Use errors.Is to distinguish
// this from operational errors without inspecting the text.
var ErrUnknownModel = errors.New("is not in the Codex allowlist")

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
