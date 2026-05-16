package anthropic

import "fmt"

// Models is the Anthropic model whitelist exposed via sandbox_agent's
// `model` parameter. Aliases (opus / sonnet / haiku) — Claude Code
// resolves them to the latest concrete model ID on its side.
var Models = []string{"opus", "sonnet", "haiku"}

// DefaultModel is the model used when the caller does not specify one.
const DefaultModel = "sonnet"

// ResolveModel validates a model name against Models. Empty input
// resolves to DefaultModel.
func ResolveModel(name string) (string, error) {
	if name == "" {
		return DefaultModel, nil
	}
	for _, m := range Models {
		if m == name {
			return m, nil
		}
	}
	return "", fmt.Errorf("model %q is not in the Anthropic whitelist (%v)", name, Models)
}
