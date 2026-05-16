//go:build !linux

package proxies

// setBypassMark is a no-op on non-Linux platforms. Proxies only run
// inside the demesne sidecar (linux/amd64); cross-platform support
// exists so the test suite compiles on dev workstations.
func setBypassMark(_ uintptr) error { return nil }
