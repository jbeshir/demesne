package server

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

const manifestRelPath = "../../manifest.json"

type manifestEntry struct {
	Name string `json:"name"`
}

type manifestFile struct {
	Tools []manifestEntry `json:"tools"`
}

func readManifestNames() (map[string]bool, error) {
	data, err := os.ReadFile(manifestRelPath)
	if err != nil {
		return nil, err
	}
	var mf manifestFile
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(mf.Tools))
	for _, entry := range mf.Tools {
		names[entry.Name] = true
	}
	return names, nil
}

func diffNameSets(a, b map[string]bool) (onlyInA, onlyInB []string) {
	for name := range a {
		if !b[name] {
			onlyInA = append(onlyInA, name)
		}
	}
	for name := range b {
		if !a[name] {
			onlyInB = append(onlyInB, name)
		}
	}
	return onlyInA, onlyInB
}

// TestToolParityWithManifest asserts that the set of MCP tool names registered
// by the server exactly matches the set declared in manifest.json.
func TestToolParityWithManifest(t *testing.T) {
	registered := NewServer(&fakeRunner{}).mcpServer.ListTools()
	serverNames := make(map[string]bool, len(registered))
	for name := range registered {
		serverNames[name] = true
	}

	manifestNames, err := readManifestNames()
	if err != nil {
		t.Fatalf("reading manifest.json: %v", err)
	}

	onlyInServer, onlyInManifest := diffNameSets(serverNames, manifestNames)
	if len(onlyInServer) == 0 && len(onlyInManifest) == 0 {
		return
	}

	slices.Sort(onlyInServer)
	slices.Sort(onlyInManifest)
	var b strings.Builder
	b.WriteString("tool name parity check failed:\n")
	if len(onlyInServer) > 0 {
		b.WriteString("  in server but not manifest.json: " + strings.Join(onlyInServer, ", ") + "\n")
	}
	if len(onlyInManifest) > 0 {
		b.WriteString("  in manifest.json but not server: " + strings.Join(onlyInManifest, ", ") + "\n")
	}
	t.Fatal(b.String())
}
