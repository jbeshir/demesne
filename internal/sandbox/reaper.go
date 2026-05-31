package sandbox

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"

	"github.com/jbeshir/demesne/internal/sidecar"
)

// ownerVerdict is the result of classifying a single sandbox's
// demesne.owner label. Three outcomes: keep (alive), reap (confirmed
// dead), or skip (missing/malformed — fail-closed, leave alone).
type ownerVerdict int

const (
	verdictKeep ownerVerdict = iota
	verdictReap
	verdictSkip
)

// reaperListFn enumerates candidate sandboxes. Tests replace this to inject fakes.
var reaperListFn = defaultReaperList

// reaperDestroyFn kills a sandbox by ID via the OpenSandbox SDK. Tests replace this.
var reaperDestroyFn = defaultReaperDestroy

// reaperSidecarRemoveFn removes the demesne sidecar for a sandbox. Tests replace this.
var reaperSidecarRemoveFn = defaultReaperSidecarRemove

// reaperProcessAliveFn checks whether a process is still running. Tests replace this.
var reaperProcessAliveFn = defaultReaperProcessAlive

// classifyOwner is the pure, exhaustively-testable alive/dead decision.
// processAlive is a seam that tests use to avoid touching real processes.
//
// Label format: "bootID_pid_starttime" (exactly three underscore-separated
// parts; see ComputeOwner for why '_'). An empty label, wrong part count, empty
// boot piece, or non-numeric pid/starttime yields verdictSkip — fail-closed
// means leave the sandbox alone.
func classifyOwner(
	label, currentBootID string,
	processAlive func(pid, starttime uint64) bool,
) ownerVerdict {
	if label == "" {
		return verdictSkip
	}
	parts := strings.Split(label, "_")
	if len(parts) != 3 {
		return verdictSkip
	}
	bootID := parts[0]
	if bootID == "" {
		return verdictSkip
	}
	pid, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return verdictSkip
	}
	starttime, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return verdictSkip
	}

	if bootID != currentBootID {
		// Host has rebooted since this sandbox was created; the labelled
		// owner process cannot be alive on this boot.
		return verdictReap
	}
	if processAlive(pid, starttime) {
		return verdictKeep
	}
	// Same boot, but either the process is gone or PID has been reused
	// with a different starttime — the seam folds both signals into false.
	return verdictReap
}

// ReapOrphans destroys sandboxes whose demesne owner process is confirmed dead.
// Called once at startup before serving any requests. Errors are accumulated
// and returned; the function never panics and always returns. Pass a bounded
// context (e.g. 30 s) to cap the total runtime.
func ReapOrphans(ctx context.Context, cfg Config) (reaped int, errs []error) {
	bootID, err := readBootID()
	if err != nil {
		return 0, []error{fmt.Errorf("reaper: read boot_id: %w", err)}
	}

	infos, err := reaperListFn(ctx, cfg)
	if err != nil {
		// Fail-closed: if enumeration fails, reap nothing.
		return 0, []error{fmt.Errorf("reaper: list sandboxes: %w", err)}
	}

	for _, info := range infos {
		label := info.Metadata[metadataDemesneOwner]
		verdict := classifyOwner(label, bootID, reaperProcessAliveFn)
		if verdict != verdictReap {
			continue
		}

		id := info.ID

		// Remove our sidecar before killing the sandbox. The sidecar shares
		// the OpenSandbox egress sidecar's network and PID namespace; podman
		// refuses to remove the egress sidecar (which KillSandbox triggers)
		// while ours still exists, which would leak both containers. This
		// mirrors the ordering in Destroy(). Best-effort: a sidecar-remove
		// failure alone does not block "reaped" from counting — the
		// KillSandbox call marks the sandbox dead regardless, and any leaked
		// sidecar will be reclaimed on the next startup reap or when podman
		// tears down the shared namespace. Both Remove and KillSandbox are
		// idempotent, so duplicate calls on overlap are safe.
		if err := reaperSidecarRemoveFn(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("reaper: sidecar remove %s: %w", id, err))
		}
		if err := reaperDestroyFn(ctx, cfg, id); err != nil {
			errs = append(errs, fmt.Errorf("reaper: destroy %s: %w", id, err))
			continue
		}
		reaped++
	}
	return reaped, errs
}

// defaultReaperList paginates the OpenSandbox lifecycle API for active
// sandboxes and returns only those bearing the demesne.owner metadata key.
func defaultReaperList(ctx context.Context, cfg Config) ([]opensandbox.SandboxInfo, error) {
	mgr := opensandbox.NewSandboxManager(connectionConfigFor(cfg))
	states := []opensandbox.SandboxState{
		opensandbox.StatePending,
		opensandbox.StateRunning,
		opensandbox.StatePausing,
		opensandbox.StatePaused,
		opensandbox.StateStopping,
	}
	var all []opensandbox.SandboxInfo
	for page := 1; ; page++ {
		resp, err := mgr.ListSandboxInfos(ctx, opensandbox.ListOptions{
			States:   states,
			Page:     page,
			PageSize: 100,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Items...)
		if !resp.Pagination.HasNextPage {
			break
		}
	}
	// The SDK's Metadata filter supports only equality checks and cannot
	// express "key is present", so we filter client-side.
	var owned []opensandbox.SandboxInfo
	for _, info := range all {
		if info.Metadata[metadataDemesneOwner] != "" {
			owned = append(owned, info)
		}
	}
	return owned, nil
}

// defaultReaperDestroy kills a sandbox by ID via the OpenSandbox SDK.
func defaultReaperDestroy(ctx context.Context, cfg Config, sandboxID string) error {
	mgr := opensandbox.NewSandboxManager(connectionConfigFor(cfg))
	return mgr.KillSandbox(ctx, sandboxID)
}

// defaultReaperSidecarRemove removes the demesne sidecar for the given sandbox.
// Idempotent — a missing container is not an error.
func defaultReaperSidecarRemove(ctx context.Context, sandboxID string) error {
	return sidecar.Remove(ctx, sandboxID)
}

// defaultReaperProcessAlive checks whether the process with the given PID and
// starttime is still alive. A read error (process gone) returns false; a
// starttime mismatch also returns false (PID reused by a different process).
func defaultReaperProcessAlive(pid, starttime uint64) bool {
	got, err := readStarttimeTicks(pid)
	if err != nil {
		return false
	}
	return got == starttime
}
