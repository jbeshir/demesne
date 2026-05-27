package codex

import (
	"errors"
	"fmt"

	"github.com/jbeshir/demesne/internal/agents"
)

// ModelName is an alias for agents.ModelName so callers in this package
// can write ModelGPT55/ModelGPT54/etc. without an extra import.
type ModelName = agents.ModelName

// Model constants for the current Codex model IDs. These are the full
// model identifiers passed to `codex exec -m`. Model IDs are taken from
// the Codex documentation; additions or removals require a whitelist update.
const (
	ModelGPT55           ModelName = "gpt-5.5"
	ModelGPT54           ModelName = "gpt-5.4"
	ModelGPT54Mini       ModelName = "gpt-5.4-mini"
	ModelGPT53Codex      ModelName = "gpt-5.3-codex"
	ModelGPT53CodexSpark ModelName = "gpt-5.3-codex-spark"
	ModelGPT52           ModelName = "gpt-5.2"
)

// Models is the Codex model whitelist exposed via sandbox_agent's
// `model` parameter.
var Models = []ModelName{
	ModelGPT55,
	ModelGPT54,
	ModelGPT54Mini,
	ModelGPT53Codex,
	ModelGPT53CodexSpark,
	ModelGPT52,
}

// DefaultModel is the model used when the caller does not specify one.
// UNVERIFIED: research shows gpt-5.5 as the recommended/sample default and
// the Codex docs say "starts with gpt-5.5 for most tasks", but no hard-coded
// default is explicitly documented.
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
