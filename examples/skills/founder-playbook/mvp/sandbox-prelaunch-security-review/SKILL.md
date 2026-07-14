---
name: sandbox-prelaunch-security-review
status: alpha
description: "Audit an MVP for pre-launch authentication, data exposure, injection, and dependency risks. Use before real users rely on it; do not use it as compliance certification."
allowed-tools: mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait
---

Run report-only unless fix mode is explicitly authorized. Required capabilities are agents, scripts, waits, and parent-side copies; allowlist enforcement varies by host and is not a safety boundary.

## Procedure

1. Enumerate `/in/*`; identify the single repo matching the launch-supplied label and expected manifest. Fail on zero or multiple matches. Copy it with `cp -a <resolved>/. /workspace/repo` and record the path. In fix mode require `git -C /workspace/repo rev-parse --is-inside-work-tree` and a valid `HEAD`; otherwise disable fix mode and emit only plain-diff drafts.
2. Run deterministic dependency scan only for supported toolchains: Node (`image=node`, `npm audit --json`), Python (`image=python`, `python -m pip install pip-audit==2.7.3 && pip-audit`), or Go (`image=go`, `go install golang.org/x/vuln/cmd/govulncheck@v1.1.4 && govulncheck ./...`). Use `egress=package-managers`. Record exit code, auditor version, and registry/advisory access in `/out/DEPSCAN.txt`; report unknown ecosystems as `UNSUPPORTED/NOT SCANNED`, never clean. A nonzero audit finding exit is a finding only when parsable output exists; other nonzero exits are scan failures.
3. Dispatch auth-session, data-exposure, and input-injection auditors in the background. Each writes `/out/AUDIT-<axis>.md` with `file:line`, severity, exploit sketch, confidence, and human-review flag. Wait while running; require succeeded, exit code zero, and output existence. Retry once as `<name>-r2`; then record the axis as `FAILED/NOT CLEAN` and stop synthesis.
4. In the orchestrator harvest each accepted child file from `/out/child/<name>/` into parent `/out` before synthesis. Synthesize only all accepted axes plus DEPSCAN into `SECURITY-REVIEW.md`; state any unsupported scan or failed axis as a hold condition. Every auth, secret, or data-handling finding is `HUMAN-REVIEW-REQUIRED`.
5. Only in authorized fix mode, draft non-human-review changes sequentially. Keep human-review changes as `/out/drafts/<slug>.patch`. Verify and re-scan only inside this branch, with a fresh verifier; apply the same gate and allow one repair round. Never merge human-review changes.

`SECURITY-REVIEW.md` orders Ship/hold summary, findings by axis, human-review index, fix status, and this exact caveat: “This is an automated pre-launch pass, not a substitute for qualified security review at higher stakes.”
