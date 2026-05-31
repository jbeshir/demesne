package sandbox

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// metadataDemesneOwner is the sandbox metadata key that records which
	// demesne-mcp instance created the sandbox. Used by the startup reaper
	// to enumerate and claim ownership of orphaned sandboxes.
	metadataDemesneOwner = "demesne.owner"

	// bootIDPath is the procfs file holding the per-boot UUID.
	bootIDPath = "/proc/sys/kernel/random/boot_id"
)

// parseStarttimeFromStat extracts field 22 (starttime, clock ticks since
// boot) from the raw contents of /proc/<pid>/stat.
//
// The comm field (field 2) is enclosed in parentheses but may itself
// contain '(', ')', and spaces — naive split mis-parses it. We find
// the LAST ')' in the line to reliably locate the end of comm, then
// index from there: field 3 is index 0, so field 22 is index 19.
func parseStarttimeFromStat(data []byte) (uint64, error) {
	line := strings.TrimRight(string(data), "\n")

	// LAST ')' ends the comm field even when comm contains '(' or ')'.
	lastParen := strings.LastIndex(line, ")")
	if lastParen < 0 {
		return 0, fmt.Errorf("proc/stat: no closing ')' found")
	}

	// fields[0] == field 3 (state), fields[19] == field 22 (starttime).
	rest := strings.TrimSpace(line[lastParen+1:])
	fields := strings.Fields(rest)
	if len(fields) < 20 {
		return 0, fmt.Errorf("proc/stat: too few fields after comm (%d)", len(fields))
	}

	ticks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("proc/stat: parsing starttime: %w", err)
	}
	return ticks, nil
}

// readStarttimeTicks reads /proc/<pid>/stat and delegates to the pure
// parseStarttimeFromStat helper.
func readStarttimeTicks(pid uint64) (uint64, error) {
	data, err := os.ReadFile("/proc/" + strconv.FormatUint(pid, 10) + "/stat")
	if err != nil {
		return 0, err
	}
	return parseStarttimeFromStat(data)
}

// readBootID returns the trimmed contents of bootIDPath.
func readBootID() (string, error) {
	data, err := os.ReadFile(bootIDPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ComputeOwner returns a per-instance owner identity string of the form
// "<bootID>_<pid>_<starttime>". The three parts together are unique across
// concurrent and sequential demesne-mcp instances:
//   - bootID guards against reuse across host reboots.
//   - pid identifies the process within a boot.
//   - starttime (clock ticks since boot) is PID-reuse-safe: a recycled PID
//     will have a different starttime, so two processes that share a PID but
//     not a starttime are distinguishable.
//
// The parts are joined with '_' (not ':') because this string is stored as an
// OpenSandbox sandbox-metadata label value, which is restricted to
// alphanumerics, '-', '_', and '.'. boot_id is a hyphenated UUID containing no
// underscores, so '_' splits the three parts unambiguously.
func ComputeOwner() (string, error) {
	boot, err := readBootID()
	if err != nil {
		return "", fmt.Errorf("read boot_id: %w", err)
	}
	pid := os.Getpid()
	if pid < 0 {
		return "", fmt.Errorf("read pid: got negative PID %d", pid)
	}
	st, err := readStarttimeTicks(uint64(pid))
	if err != nil {
		return "", fmt.Errorf("read starttime: %w", err)
	}
	return fmt.Sprintf("%s_%d_%d", boot, pid, st), nil
}
