package mcpproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ToolName is the name of a tool exposed by an upstream MCP server.
type ToolName string

// ServerName is the name of an upstream MCP server.
type ServerName string

// Sentinel string values recognised in the allowlist override file.
// Anything else must be an explicit array of tool names.
const (
	allowAllSentinel   = "*"
	useDefaultSentinel = "default"
)

// ServerAllowlist is the resolved allowlist for one server. AllowAll
// means "expose every tool the upstream reports at runtime"; Tools
// is the explicit set otherwise. AllowAll wins when both are set.
type ServerAllowlist struct {
	AllowAll bool
	Tools    map[ToolName]struct{}
}

// Allowed reports whether the given tool name is permitted.
func (a ServerAllowlist) Allowed(tool ToolName) bool {
	if a.AllowAll {
		return true
	}
	_, ok := a.Tools[tool]
	return ok
}

// overrideFile is the user file's wire shape. Each value is decoded
// raw and then interpreted in parseOverrideEntry.
type overrideFile map[string]json.RawMessage

// ResolveAllowlist combines the built-in defaults with the
// optional user override file. The result has one entry per
// server that's actually exposed; servers absent from both are
// not in the returned map.
//
// If the override file path is empty or the file is missing, only
// built-in defaults are returned. If the file is present but
// malformed, the error is returned.
func ResolveAllowlist(overridePath string) (map[string]ServerAllowlist, error) {
	out := make(map[string]ServerAllowlist, len(defaultAllowlist))
	for name, set := range defaultAllowlist {
		out[string(name)] = ServerAllowlist{Tools: cloneSet(set)}
	}
	if overridePath == "" {
		return out, nil
	}
	data, err := os.ReadFile(overridePath) //nolint:gosec // operator config
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out, nil
		}
		return nil, fmt.Errorf("read %s: %w", overridePath, err)
	}
	return applyOverride(out, data)
}

func applyOverride(base map[string]ServerAllowlist, data []byte) (map[string]ServerAllowlist, error) {
	var f overrideFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse allowlist override: %w", err)
	}
	for name, raw := range f {
		if strings.HasPrefix(name, "_") {
			// Skip metadata keys (e.g. "_doc" emitted by SeedOverrideFile).
			continue
		}
		entry, err := parseOverrideEntry(name, raw)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			// Empty array: explicitly disable.
			delete(base, name)
			continue
		}
		base[name] = *entry
	}
	return base, nil
}

// parseOverrideEntry interprets one server's override value:
//   - string "*"       → AllowAll
//   - string "default" → built-in default (error if none exists)
//   - []string         → explicit tool names
//   - []               → returns nil entry, signalling "disable"
func parseOverrideEntry(name string, raw json.RawMessage) (*ServerAllowlist, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case allowAllSentinel:
			return &ServerAllowlist{AllowAll: true}, nil
		case useDefaultSentinel:
			set, ok := defaultAllowlist[ServerName(name)]
			if !ok {
				return nil, fmt.Errorf(
					"allowlist override for %q says %q but demesne ships no default for that server",
					name, useDefaultSentinel,
				)
			}
			return &ServerAllowlist{Tools: cloneSet(set)}, nil
		default:
			return nil, fmt.Errorf(
				"allowlist override for %q has unknown string value %q (use %q, %q, or a list)",
				name, s, useDefaultSentinel, allowAllSentinel,
			)
		}
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("allowlist override for %q: %w", name, err)
	}
	if len(list) == 0 {
		return nil, nil
	}
	tools := make(map[ToolName]struct{}, len(list))
	for _, tool := range list {
		if tool == "" {
			return nil, errors.New("allowlist override " + name + ": tool name cannot be empty")
		}
		tools[ToolName(tool)] = struct{}{}
	}
	return &ServerAllowlist{Tools: tools}, nil
}

// SeedOverrideFile writes a starter override file at the given
// path if no file exists there. The seed contains a "default"
// entry for every known default server so the user has something
// to edit. Existing files are left untouched.
func SeedOverrideFile(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	names := sortedDefaultNames()
	var b strings.Builder
	b.WriteString("{\n  \"_doc\": ")
	b.WriteString(jsonString("Per-server tool allowlist. Value is one of: " +
		"\"default\" (use demesne's built-in read-only set), " +
		"\"*\" (every tool the upstream advertises), " +
		"a list of tool names, or [] to disable the server."))
	for _, n := range names {
		b.WriteString(",\n  ")
		b.WriteString(jsonString(n))
		b.WriteString(`: "default"`)
	}
	b.WriteString("\n}\n")
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func cloneSet(in map[ToolName]struct{}) map[ToolName]struct{} {
	out := make(map[ToolName]struct{}, len(in))
	for k := range in {
		out[k] = struct{}{}
	}
	return out
}

func sortedDefaultNames() []string {
	names := make([]string, 0, len(defaultAllowlist))
	for n := range defaultAllowlist {
		names = append(names, string(n))
	}
	sort.Strings(names)
	return names
}

func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
