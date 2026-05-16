//go:build linux

package proxies

import "syscall"

// soMarkBypass is the SO_MARK value the OpenSandbox egress sidecar
// accepts unconditionally in its nftables egress chain
// (`meta mark 0x00000001 accept`). All demesne proxies set this on
// every outbound socket so they can reach their upstream services
// without those services being in the sandbox's egress allowlist —
// the same mechanism OpenSandbox's own egress proxy uses on itself.
//
// Requires CAP_NET_ADMIN; the sidecar container is launched with
// `--cap-add=NET_ADMIN` so setsockopt(SO_MARK) succeeds. The agent's
// container runs without that capability and without the mark, so
// the daddr filter still applies to its traffic.
const soMarkBypass = 1

// setBypassMark applies SO_MARK to the given file descriptor. Errors
// (including EPERM when CAP_NET_ADMIN is missing) propagate to the
// caller; the proxy must refuse to dial rather than silently fail to
// mark its packets, because an unmarked socket can't bypass the
// sandbox egress filter and would leak the demesne bypass guarantee.
//
// The fd comes from syscall.RawConn.Control which only ever yields
// values in the int range on Linux, so the conversion is safe.
func setBypassMark(fd uintptr) error {
	//nolint:gosec // fd is always within int range
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, soMarkBypass)
}
