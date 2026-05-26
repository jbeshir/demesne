package anthropic

import (
	"errors"
	"fmt"

	"github.com/jbeshir/demesne/internal/agents"
)

// ModelName is an alias for agents.ModelName so callers in this package
// can write ModelOpus/ModelSonnet/ModelHaiku without an extra import.
type ModelName = agents.ModelName

// Model alias constants for the three Claude tiers. Claude Code resolves
// these short aliases to the latest concrete Anthropic model ID on its side.
const (
	ModelOpus   ModelName = "opus"
	ModelSonnet ModelName = "sonnet"
	ModelHaiku  ModelName = "haiku"
)

// Models is the Anthropic model whitelist exposed via sandbox_agent's
// `model` parameter.
var Models = []ModelName{ModelOpus, ModelSonnet, ModelHaiku}

// DefaultModel is the model used when the caller does not specify one.
const DefaultModel ModelName = ModelSonnet

// ErrUnknownModel is the sentinel wrapped by ResolveModel when the
// requested model is not in the whitelist. Use errors.Is to distinguish
// this from operational errors without inspecting the text.
var ErrUnknownModel = errors.New("is not in the Anthropic whitelist")

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
