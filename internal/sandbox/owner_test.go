package sandbox

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// openSandboxLabelValue matches OpenSandbox's metadata-label-value rule: 1-63
// characters, starting and ending with an alphanumeric, otherwise only
// alphanumerics, '-', '_', and '.'. ComputeOwner's output is stored as such a
// label, so it must satisfy this; a ':' separator (the original bug) does not.
var openSandboxLabelValue = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]{0,61}[A-Za-z0-9])?$`)

func TestComputeOwnerIsValidLabelValue(t *testing.T) {
	owner, err := ComputeOwner()
	if err != nil {
		t.Fatalf("ComputeOwner() error = %v", err)
	}
	if !openSandboxLabelValue.MatchString(owner) {
		t.Errorf("ComputeOwner() = %q is not a valid OpenSandbox metadata label value", owner)
	}
	if len(owner) > 63 {
		t.Errorf("ComputeOwner() = %q has length %d, exceeds the 63-char label limit", owner, len(owner))
	}
}

// buildStatLine constructs a synthetic /proc/pid/stat line with the given
// comm and starttime. Fields 3-21 are zero placeholders; field 22 is
// starttime (index 19 after the last ')').
func buildStatLine(pid int, comm string, starttime uint64) []byte {
	// 18 zero placeholders occupy indices 1-18; starttime is index 19.
	placeholders := strings.Repeat(" 0", 18)
	return []byte(fmt.Sprintf("%d (%s) S%s %d", pid, comm, placeholders, starttime))
}

func TestParseStarttimeFromStat(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    uint64
		wantErr bool
	}{
		{
			name: "simple comm",
			data: buildStatLine(1234, "myproc", 9876),
			want: 9876,
		},
		{
			name: "comm with spaces",
			data: buildStatLine(1234, "my proc name", 5555),
			want: 5555,
		},
		{
			// comm = "weird ) (name" — the LAST ')' in the line is the one
			// closing (name), so the parser correctly skips the entire comm.
			// This is the canonical parenthesis-in-comm gotcha.
			name: "comm with embedded open and close parens",
			data: buildStatLine(1234, "weird ) (name", 7777),
			want: 7777,
		},
		{
			name:    "missing closing paren",
			data:    []byte("1234 (noclose S 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 5678"),
			wantErr: true,
		},
		{
			name:    "too few fields after comm",
			data:    []byte("1234 (myproc) S 0 0"),
			wantErr: true,
		},
		{
			name:    "non-numeric starttime",
			data:    []byte("1234 (myproc) S 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 notanumber"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStarttimeFromStat(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseStarttimeFromStat() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseStarttimeFromStat() = %d, want %d", got, tt.want)
			}
		})
	}
}
