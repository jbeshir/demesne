package codex

import (
	"errors"
	"fmt"

	"github.com/jbeshir/demesne/internal/agents"
)

// ModelName is an alias for agents.ModelName so callers in this package
// can write ModelGPT55/ModelGPT54/etc. without an extra import.
type ModelName = agents.ModelName

// Model constants for Codex model IDs verified against rust-v0.134.0.
// These are the full identifiers passed to `codex exec -m`.
// Additions or removals require a whitelist update.
const (
	ModelGPT55      ModelName = "gpt-5.5"
	ModelGPT54      ModelName = "gpt-5.4"
	ModelGPT54Mini  ModelName = "gpt-5.4-mini"
	ModelGPT53Codex ModelName = "gpt-5.3-codex"
	ModelGPT52      ModelName = "gpt-5.2"
)

// Models is the Codex model whitelist exposed via sandbox_agent's
// `model` parameter.
var Models = []ModelName{
	ModelGPT55,
	ModelGPT54,
	ModelGPT54Mini,
	ModelGPT53Codex,
	ModelGPT52,
}

// DefaultModel is the model used when the caller does not specify one.
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
