package sandbox

import (
	"context"
	"errors"
	"fmt"
	"testing"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sandbox IDs used across reaper orchestrator tests. Extracted as constants
// to satisfy the goconst linter (>= 3 occurrences each).
const (
	reaperTestSbA    = "sb-a"
	reaperTestSbB    = "sb-b"
	reaperTestSbDead = "sb-dead"
)

// saveReaperSeams saves all four reaper seam vars and registers t.Cleanup to
// restore them, preventing cross-test contamination of package-level state.
func saveReaperSeams(t *testing.T) {
	t.Helper()
	prevList, prevDestroy, prevSidecar, prevAlive :=
		reaperListFn, reaperDestroyFn, reaperSidecarRemoveFn, reaperProcessAliveFn
	t.Cleanup(func() {
		reaperListFn = prevList
		reaperDestroyFn = prevDestroy
		reaperSidecarRemoveFn = prevSidecar
		reaperProcessAliveFn = prevAlive
	})
}

// makeOwnedInfo builds a SandboxInfo bearing the given demesne.owner label.
func makeOwnedInfo(id, ownerLabel string) opensandbox.SandboxInfo {
	return opensandbox.SandboxInfo{
		ID:       id,
		Metadata: map[string]string{metadataDemesneOwner: ownerLabel},
	}
}

// makeUnlabelledInfo builds a SandboxInfo with no demesne.owner metadata.
func makeUnlabelledInfo(id string) opensandbox.SandboxInfo {
	return opensandbox.SandboxInfo{ID: id}
}

func TestClassifyOwner(t *testing.T) {
	const (
		boot  = "boot-abc-123"
		other = "boot-xyz-456"
	)
	sameBootLabel := fmt.Sprintf("%s_1234_567", boot)
	diffBootLabel := fmt.Sprintf("%s_1234_567", other)

	tests := []struct {
		name        string
		label       string
		currentBoot string
		alive       bool
		want        ownerVerdict
	}{
		{
			name:        "alive-same-boot",
			label:       sameBootLabel,
			currentBoot: boot,
			alive:       true,
			want:        verdictKeep,
		},
		{
			name:        "dead-same-boot",
			label:       sameBootLabel,
			currentBoot: boot,
			alive:       false,
			want:        verdictReap,
		},
		{
			name:        "different-boot",
			label:       diffBootLabel,
			currentBoot: boot,
			// processAlive is irrelevant: different bootID → verdictReap immediately.
			want: verdictReap,
		},
		{
			name:        "pid-reuse",
			label:       sameBootLabel,
			currentBoot: boot,
			// processAlive returns false because the new process has a different
			// starttime; the seam folds the mismatch into a single false signal.
			alive: false,
			want:  verdictReap,
		},
		{
			name:        "empty-label",
			label:       "",
			currentBoot: boot,
			want:        verdictSkip,
		},
		{
			name:        "malformed-wrong-part-count",
			label:       "boot-abc_1234", // two parts, not three
			currentBoot: boot,
			want:        verdictSkip,
		},
		{
			name:        "malformed-non-numeric-pid",
			label:       fmt.Sprintf("%s_notapid_567", boot),
			currentBoot: boot,
			want:        verdictSkip,
		},
		{
			name:        "malformed-non-numeric-starttime",
			label:       fmt.Sprintf("%s_1234_notanumber", boot),
			currentBoot: boot,
			want:        verdictSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOwner(tt.label, tt.currentBoot, func(_, _ uint64) bool {
				return tt.alive
			})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReapOrphans(t *testing.T) {
	// Read the real boot_id to construct labels whose bootID matches the
	// running system (needed for the "alive" sandbox path through classifyOwner).
	currentBoot, err := readBootID()
	require.NoError(t, err, "reading /proc/sys/kernel/random/boot_id")

	// deadLabel produces a label with a different bootID → verdictReap via
	// boot mismatch, without ever consulting processAlive.
	deadLabel := func(n int) string {
		return fmt.Sprintf("dead-boot-reaper-test_%d_100", n)
	}

	// aliveLabel matches the current bootID; classifyOwner reaches processAlive,
	// which the seam returns true for (alivePID, aliveStarttime).
	const alivePID, aliveStarttime = uint64(99997), uint64(99998)
	aliveLabel := fmt.Sprintf("%s_%d_%d", currentBoot, alivePID, aliveStarttime)

	tests := []struct {
		name string

		infos   []opensandbox.SandboxInfo
		listErr error

		// destroyErrors maps sandbox ID → error returned by destroyFn.
		destroyErrors map[string]error
		// sidecarErrors maps sandbox ID → error returned by sidecarRemoveFn.
		sidecarErrors map[string]error

		wantReaped   int
		wantErrCount int

		// wantDestroyed is the set of sandbox IDs passed to destroyFn (order-insensitive).
		wantDestroyed []string
		// wantSidecar is the set of sandbox IDs passed to sidecarRemoveFn.
		wantSidecar []string
		// destroyNeverCalled asserts that destroyFn was not invoked at all.
		destroyNeverCalled bool
	}{
		{
			name:               "enumerate-error",
			listErr:            errors.New("api unreachable"),
			wantReaped:         0,
			wantErrCount:       1,
			destroyNeverCalled: true,
		},
		{
			name: "two-dead-candidates",
			infos: []opensandbox.SandboxInfo{
				makeOwnedInfo(reaperTestSbA, deadLabel(1)),
				makeOwnedInfo(reaperTestSbB, deadLabel(2)),
			},
			wantReaped:    2,
			wantErrCount:  0,
			wantDestroyed: []string{reaperTestSbA, reaperTestSbB},
			wantSidecar:   []string{reaperTestSbA, reaperTestSbB},
		},
		{
			name: "one-dead-one-alive",
			infos: []opensandbox.SandboxInfo{
				makeOwnedInfo(reaperTestSbDead, deadLabel(1)),
				makeOwnedInfo("sb-alive", aliveLabel),
			},
			wantReaped:    1,
			wantErrCount:  0,
			wantDestroyed: []string{reaperTestSbDead},
			wantSidecar:   []string{reaperTestSbDead},
		},
		{
			name: "one-dead-one-missing-label-one-malformed",
			infos: []opensandbox.SandboxInfo{
				makeOwnedInfo(reaperTestSbDead, deadLabel(1)),
				makeUnlabelledInfo("sb-no-label"),
				makeOwnedInfo("sb-malformed", "boot_only_two_too_many"),
			},
			wantReaped:    1,
			wantErrCount:  0,
			wantDestroyed: []string{reaperTestSbDead},
			wantSidecar:   []string{reaperTestSbDead},
		},
		{
			name: "destroy-error-on-one",
			infos: []opensandbox.SandboxInfo{
				makeOwnedInfo(reaperTestSbA, deadLabel(1)),
				makeOwnedInfo(reaperTestSbB, deadLabel(2)),
			},
			destroyErrors: map[string]error{reaperTestSbA: errors.New("kill rejected")},
			wantReaped:    1,
			wantErrCount:  1,
			// Both are attempted even when one fails.
			wantDestroyed: []string{reaperTestSbA, reaperTestSbB},
			wantSidecar:   []string{reaperTestSbA, reaperTestSbB},
		},
		{
			name: "sidecar-remove-error-destroy-succeeds",
			infos: []opensandbox.SandboxInfo{
				makeOwnedInfo(reaperTestSbA, deadLabel(1)),
			},
			sidecarErrors: map[string]error{reaperTestSbA: errors.New("container not found")},
			// reaped still counts because the destroy succeeded.
			wantReaped:    1,
			wantErrCount:  1,
			wantDestroyed: []string{reaperTestSbA},
			wantSidecar:   []string{reaperTestSbA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveReaperSeams(t)

			var destroyedIDs []string
			var sidecarIDs []string

			reaperListFn = func(_ context.Context, _ Config) ([]opensandbox.SandboxInfo, error) {
				return tt.infos, tt.listErr
			}
			reaperDestroyFn = func(_ context.Context, _ Config, id string) error {
				destroyedIDs = append(destroyedIDs, id)
				if e, ok := tt.destroyErrors[id]; ok {
					return e
				}
				return nil
			}
			reaperSidecarRemoveFn = func(_ context.Context, id string) error {
				sidecarIDs = append(sidecarIDs, id)
				if e, ok := tt.sidecarErrors[id]; ok {
					return e
				}
				return nil
			}
			reaperProcessAliveFn = func(pid, starttime uint64) bool {
				return pid == alivePID && starttime == aliveStarttime
			}

			reaped, errs := ReapOrphans(context.Background(), Config{})

			assert.Equal(t, tt.wantReaped, reaped)
			assert.Len(t, errs, tt.wantErrCount)

			if tt.destroyNeverCalled {
				assert.Empty(t, destroyedIDs, "destroyFn must not be called on enumeration failure")
				assert.Empty(t, sidecarIDs, "sidecarRemoveFn must not be called on enumeration failure")
			}
			if tt.wantDestroyed != nil {
				assert.ElementsMatch(t, tt.wantDestroyed, destroyedIDs)
			}
			if tt.wantSidecar != nil {
				assert.ElementsMatch(t, tt.wantSidecar, sidecarIDs)
			}
		})
	}
}
