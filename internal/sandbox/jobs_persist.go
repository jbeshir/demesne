package sandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// JobRecord is the on-disk representation of a background job. It is written
// atomically (write to .tmp then rename) so readers always see a complete
// JSON blob, never a partial write.
type JobRecord struct {
	ID          string    `json:"id"`
	Tool        string    `json:"tool"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	RunJobID    string    `json:"run_job_id,omitempty"`
	OutHost     string    `json:"out_host,omitempty"`
	ResultsHost string    `json:"results_host,omitempty"`
	SandboxID   string    `json:"sandbox_id,omitempty"`
	Parent      string    `json:"parent,omitempty"`
	ChildIDs    []string  `json:"child_ids,omitempty"`
	ExitCode    int       `json:"exit_code"`
}

// writeJobRecord atomically writes rec to stateDir/<id>.json by writing to a
// .tmp file first then renaming. This mirrors writeResultsFile in results.go.
// MkdirAll is called before the write so callers need not pre-create stateDir.
func writeJobRecord(stateDir string, rec JobRecord) error {
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		return fmt.Errorf("mkdir jobs state dir: %w", err)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal job record: %w", err)
	}
	path := filepath.Join(stateDir, rec.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write job record tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename job record: %w", err)
	}
	return nil
}

// loadJobs reads all *.json files from stateDir and returns the parsed records.
// Missing or corrupt individual files are logged and skipped. A missing
// stateDir returns a nil slice without error.
func loadJobs(stateDir string) ([]JobRecord, error) {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read jobs state dir: %w", err)
	}
	var recs []JobRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(stateDir, e.Name())
		data, err := os.ReadFile(path) //nolint:gosec // path is runner-composed under stateDir
		if err != nil {
			log.Printf("sandbox: load job record %s: %v", e.Name(), err)
			continue
		}
		var rec JobRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			log.Printf("sandbox: parse job record %s: %v", e.Name(), err)
			continue
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

// reconcileRunning marks any record whose status is "running" as "failed" and
// rewrites it to disk. A process restart means the goroutines are gone; any
// sandbox that was running is in an unknown state and is treated as failed
// (FINDINGS §H8). The updated records slice is returned.
func reconcileRunning(recs []JobRecord, stateDir string, now time.Time) []JobRecord {
	for i := range recs {
		if recs[i].Status != string(JobStatusRunning) {
			continue
		}
		recs[i].Status = string(JobStatusFailed)
		recs[i].UpdatedAt = now
		if err := writeJobRecord(stateDir, recs[i]); err != nil {
			log.Printf("sandbox: reconcile job %s: %v", recs[i].ID, err)
		}
	}
	return recs
}

// processAlive reports whether pid is a live process. syscall.Kill(pid,0)
// delivers no signal: nil or EPERM => the process exists (EPERM means it is
// owned by another user); ESRCH => no such process. Any other errno is
// treated conservatively as alive so the sweep never deletes a live
// instance's records.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	return true
}

// pidFromInstanceDir extracts the owning PID from an instance dir name of the
// form "<pid>-<startnanos>". Returns ok=false for any name that does not
// parse so the caller skips (never deletes) unrecognised entries.
func pidFromInstanceDir(name string) (int, bool) {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		return 0, false
	}
	pid, err := strconv.Atoi(parts[0])
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// sweepStaleInstanceDirs removes sibling instance subdirectories under
// jobsRoot whose owning PID is no longer alive, reclaiming on-disk job
// records left by crashed demesne instances. It never touches ownID, never
// touches a dir whose PID is still alive, and only ever removes job-record
// dirs — container cleanup for crashed instances is handled separately by
// ReapOrphans. Parse failures / unreadable entries are skipped.
func sweepStaleInstanceDirs(jobsRoot, ownID string, alive func(pid int) bool) {
	entries, err := os.ReadDir(jobsRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("sandbox: sweep stale job dirs: read %s: %v", jobsRoot, err)
		}
		return
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ownID {
			continue
		}
		pid, ok := pidFromInstanceDir(e.Name())
		if !ok {
			continue
		}
		if alive(pid) {
			continue
		}
		dir := filepath.Join(jobsRoot, e.Name())
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("sandbox: sweep stale job dir %s: %v", e.Name(), err)
			continue
		}
		log.Printf("sandbox: swept stale job registry dir %s (owner pid %d not alive)", e.Name(), pid)
	}
}

// removeJobRecord deletes <stateDir>/<id>.json; a missing file is not an error.
func removeJobRecord(stateDir, id string) error {
	path := filepath.Join(stateDir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove job record: %w", err)
	}
	return nil
}

// tailFile returns the last maxBytes bytes of the file at path, dropping any
// partial first line that results from seeking to the middle of the file.
// A missing file returns an empty string without error. The path must be
// runner-composed under cfg.OutputRoot.
func tailFile(path string, maxBytes int64) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is runner-composed under job outHost
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("open file for tail: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file for tail: %w", err)
	}
	size := info.Size()
	if size == 0 {
		return "", nil
	}

	start := size - maxBytes
	if start < 0 {
		start = 0
	}

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek file for tail: %w", err)
	}

	buf, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read file tail: %w", err)
	}

	s := string(buf)
	if start > 0 {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
	}
	return s, nil
}
