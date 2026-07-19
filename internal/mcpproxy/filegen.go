package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

const (
	imageURLKey = "image_url"
	filesKey    = "files"
	pathKey     = "path"
)

// parentKeyT is the unexported context key type for the parent job ID.
type parentKeyT struct{}

var parentKey parentKeyT

// parentFromHTTP extracts proxymcp.ParentHeader from the request and
// stashes it in the context under parentKey when non-empty.
func parentFromHTTP(ctx context.Context, r *http.Request) context.Context {
	if v := r.Header.Get(proxymcp.ParentHeader); v != "" {
		return context.WithValue(ctx, parentKey, v)
	}
	return ctx
}

// ParentJobIDFromContext returns the parent job ID stored by
// parentFromHTTP, or "" if absent.
func ParentJobIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(parentKey).(string)
	return v
}

// FileDeliverer copies host-side files produced by file-generating MCP
// upstreams into a sandbox's /workspace/generated/ directory, so the
// sandboxed agent can read them. Implementations bridge mcpproxy (which
// has no sandbox visibility) to the sandbox package (which owns the
// per-run workspace path).
type FileDeliverer interface {
	// DeliveryDir returns the host directory the deliverer will copy
	// into and the corresponding in-sandbox path (always
	// "/workspace/generated"). The host directory is created if needed.
	DeliveryDir(parentJobID string) (hostDir, sandboxDir string, err error)
	// Deliver copies each host path into the delivery dir (skipping any
	// path already inside it) and returns a host->sandbox path map for
	// RewriteResult to apply.
	Deliver(parentJobID string, hostPaths []string) (map[string]string, error)
}

// FileGenAdapter is the per-server interface for adapting file-generating
// MCP upstreams: preparing arguments, extracting produced host paths from
// results, and rewriting those paths to sandbox-visible equivalents.
type FileGenAdapter interface {
	Server() string
	Tools() []string
	PrepareArgs(args map[string]any, hostDeliveryDir string) map[string]any
	ExtractHostPaths(*mcp.CallToolResult) []string
	RewriteResult(*mcp.CallToolResult, map[string]string)
}

// cloneArgs returns a shallow copy of args, or a new empty map if nil.
func cloneArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args)+2)
	for k, v := range args {
		out[k] = v
	}
	return out
}

// ---- mermaidAdapter ----

var mermaidSeq atomic.Int64

func nextMermaidSeq() int64 {
	return mermaidSeq.Add(1)
}

var savedToRE = regexp.MustCompile(`(?i)(?:also )?saved to:\s*(\S+)`)

type mermaidAdapter struct{}

func (mermaidAdapter) Server() string { return serverMermaid }

func (mermaidAdapter) Tools() []string { return []string{toolGenerate} }

func (mermaidAdapter) PrepareArgs(args map[string]any, hostDeliveryDir string) map[string]any {
	out := cloneArgs(args)
	out["folder"] = hostDeliveryDir
	out["name"] = fmt.Sprintf("mermaid-%d-%d", time.Now().UnixNano(), nextMermaidSeq())
	return out
}

func (mermaidAdapter) ExtractHostPaths(result *mcp.CallToolResult) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, c := range result.Content {
		tc, ok := c.(mcp.TextContent)
		if !ok {
			continue
		}
		for _, m := range savedToRE.FindAllStringSubmatch(tc.Text, -1) {
			p := m[1]
			if _, dup := seen[p]; !dup {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func (mermaidAdapter) RewriteResult(result *mcp.CallToolResult, mapping map[string]string) {
	filtered := result.Content[:0]
	for _, c := range result.Content {
		switch c.(type) {
		case mcp.ImageContent, mcp.AudioContent:
			// drop binary content blocks — the file is delivered via mapping
		default:
			if tc, ok := c.(mcp.TextContent); ok {
				for host, sandbox := range mapping {
					tc.Text = strings.ReplaceAll(tc.Text, host, sandbox)
				}
				filtered = append(filtered, tc)
			} else {
				filtered = append(filtered, c)
			}
		}
	}
	result.Content = filtered
}

// ---- imageGenAdapter ----

type imageGenAdapter struct{}

func (imageGenAdapter) Server() string { return serverImageGen }

func (imageGenAdapter) Tools() []string { return []string{toolGenerateImg, toolEditImage} }

func (imageGenAdapter) PrepareArgs(args map[string]any, _ string) map[string]any {
	return args
}

func (imageGenAdapter) ExtractHostPaths(result *mcp.CallToolResult) []string {
	seen := map[string]struct{}{}
	var paths []string

	addFileURL := func(raw string) {
		const scheme = "file://"
		if !strings.HasPrefix(raw, scheme) {
			return
		}
		p := strings.TrimPrefix(raw, scheme)
		if _, dup := seen[p]; !dup {
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}

	if sc, ok := result.StructuredContent.(map[string]any); ok {
		if u, ok := sc[imageURLKey].(string); ok {
			addFileURL(u)
		}
	}

	for _, c := range result.Content {
		tc, ok := c.(mcp.TextContent)
		if !ok {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &obj); err != nil {
			continue
		}
		if u, ok := obj[imageURLKey].(string); ok {
			addFileURL(u)
		}
	}

	return paths
}

func (imageGenAdapter) RewriteResult(result *mcp.CallToolResult, mapping map[string]string) {
	fileURLMapping := make(map[string]string, len(mapping))
	for host, sandbox := range mapping {
		fileURLMapping["file://"+host] = sandbox
	}

	if sc, ok := result.StructuredContent.(map[string]any); ok {
		if u, ok := sc[imageURLKey].(string); ok {
			for oldURL, newPath := range fileURLMapping {
				u = strings.ReplaceAll(u, oldURL, newPath)
			}
			sc[imageURLKey] = u
		}
	}

	for i, c := range result.Content {
		tc, ok := c.(mcp.TextContent)
		if !ok {
			continue
		}
		for oldURL, newPath := range fileURLMapping {
			tc.Text = strings.ReplaceAll(tc.Text, oldURL, newPath)
		}
		result.Content[i] = tc
	}
}

// ---- assetsAdapter ----

// assetsAdapter bridges the "assets" MCP server, whose file-producing get_*
// tools write asset files to their own output directory and report them in a
// native structured result shaped
// {"files":[{"path",...}],"count":N}. Unlike imageGenAdapter's file:// URLs,
// the reported paths are plain absolute host paths. Like imageGenAdapter, the
// server picks its own output location, so PrepareArgs is a no-op.
type assetsAdapter struct{}

func (assetsAdapter) Server() string { return serverAssets }

func (assetsAdapter) Tools() []string {
	return []string{
		toolGetIcon,
		toolGetIllustration,
		toolGetFont,
		toolGetPhoto,
		toolGetTexture,
		toolGetModel,
		toolGetAudio,
		toolGetSprite,
		toolGetPack,
	}
}

func (assetsAdapter) PrepareArgs(args map[string]any, _ string) map[string]any {
	return args
}

func (assetsAdapter) ExtractHostPaths(result *mcp.CallToolResult) []string {
	seen := map[string]struct{}{}
	var paths []string
	addManifest := func(manifest map[string]any) {
		files, ok := manifest[filesKey].([]any)
		if !ok {
			return
		}
		for _, f := range files {
			entry, ok := f.(map[string]any)
			if !ok {
				continue
			}
			p, ok := entry[pathKey].(string)
			if !ok || p == "" {
				continue
			}
			if _, dup := seen[p]; !dup {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}

	if sc, ok := result.StructuredContent.(map[string]any); ok {
		addManifest(sc)
	}
	for _, c := range result.Content {
		tc, ok := c.(mcp.TextContent)
		if !ok {
			continue
		}
		var manifest map[string]any
		if json.Unmarshal([]byte(tc.Text), &manifest) == nil {
			addManifest(manifest)
		}
	}
	return paths
}

func (assetsAdapter) RewriteResult(result *mcp.CallToolResult, mapping map[string]string) {
	if sc, ok := result.StructuredContent.(map[string]any); ok {
		if files, ok := sc[filesKey].([]any); ok {
			for _, f := range files {
				entry, ok := f.(map[string]any)
				if !ok {
					continue
				}
				if p, ok := entry[pathKey].(string); ok {
					if sandbox, ok := mapping[p]; ok {
						entry[pathKey] = sandbox
					}
				}
			}
		}
	}

	for i, c := range result.Content {
		tc, ok := c.(mcp.TextContent)
		if !ok {
			continue
		}
		for host, sandbox := range mapping {
			tc.Text = strings.ReplaceAll(tc.Text, host, sandbox)
		}
		result.Content[i] = tc
	}
}

// ---- registry ----

type fileGenRegistry struct {
	servers  map[string]bool
	adapters map[string]map[string]FileGenAdapter
}

func newFileGenRegistry(adapters ...FileGenAdapter) *fileGenRegistry {
	r := &fileGenRegistry{
		servers:  make(map[string]bool),
		adapters: make(map[string]map[string]FileGenAdapter),
	}
	for _, a := range adapters {
		srv := a.Server()
		r.servers[srv] = true
		if r.adapters[srv] == nil {
			r.adapters[srv] = make(map[string]FileGenAdapter)
		}
		for _, tool := range a.Tools() {
			r.adapters[srv][tool] = a
		}
	}
	return r
}

func (r *fileGenRegistry) IsFileGenServer(name string) bool {
	return r.servers[name]
}

func (r *fileGenRegistry) AdapterFor(server, tool string) FileGenAdapter {
	if tools, ok := r.adapters[server]; ok {
		return tools[tool]
	}
	return nil
}

var defaultFileGenRegistry = newFileGenRegistry(&mermaidAdapter{}, &imageGenAdapter{}, &assetsAdapter{})

// IsFileGenServer reports whether the given upstream is a known
// file-generating server. Used by the agent-side wiring to gate
// parent-identity header injection.
func IsFileGenServer(name string) bool {
	return defaultFileGenRegistry.IsFileGenServer(name)
}

// FileGenAdapterFor returns the adapter for the given (server, tool)
// pair, or nil if not covered by the default registry.
func FileGenAdapterFor(server, tool string) FileGenAdapter {
	return defaultFileGenRegistry.AdapterFor(server, tool)
}
