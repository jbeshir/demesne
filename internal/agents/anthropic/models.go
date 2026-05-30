package anthropic

import (
	"errors"
	"fmt"

	"github.com/jbeshir/demesne/internal/agents"
	proxyanthropic "github.com/jbeshir/demesne/internal/proxies/anthropic"
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
// `model` parameter. Derived from proxyanthropic.Aliases() so the
// whitelist stays in sync with the pricing catalog automatically.
var Models = func() []ModelName {
	a := proxyanthropic.Aliases()
	out := make([]ModelName, len(a))
	for i, s := range a {
		out[i] = ModelName(s)
	}
	return out
}()

// DefaultModel is the model used when the caller does not specify one.
// This MUST equal the alias at index 0 of proxyanthropic's modelCatalog.
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
