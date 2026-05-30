# Persistent session — create → exec → upload → exec → download → destroy

This example walks through the full lifecycle of a persistent sandbox: create it, install a package, upload a data file, run analysis, download the result, then destroy the sandbox. It uses six separate JSON-RPC calls that must be issued in order.

## The six steps

| Step | File | Tool | What it does |
|------|------|------|--------------|
| 1 | `01-create.json` | `sandbox_create` | Create a Python sandbox; returns `sandbox_id` and `output_dir`. |
| 2 | `02-exec-install.json` | `sandbox_exec` | Install pandas inside the sandbox. |
| 3 | `03-upload.json` | `sandbox_upload` | Copy a CSV file from the host into the sandbox at `/data.csv`. |
| 4 | `04-exec-analyse.json` | `sandbox_exec` | Read the CSV with pandas, print `.describe()`, write `results.json`. |
| 5 | `05-download.json` | `sandbox_download` | Copy `/results.json` from the sandbox to the host `output_dir`. |
| 6 | `06-destroy.json` | `sandbox_destroy` | Kill the sandbox container. |

## Key concepts

- **`sandbox_id` threads through every call.** The `sandbox_id` returned by `sandbox_create` in step 1 must be substituted into every subsequent request. The JSON files use the placeholder string `"SANDBOX_ID_HERE"` — you must replace it with the actual ID from step 1's response before issuing steps 2–6.
- **TTL refreshes on each `sandbox_exec`.** The sandbox has a 24-hour time-to-live from creation, and each `sandbox_exec` call resets the TTL to 24 hours from the time of that call. As long as you keep issuing commands, the sandbox stays alive.
- **`output_dir` is preserved after `sandbox_destroy`.** The host output directory returned by `sandbox_create` is not removed when the sandbox is destroyed. Files written to `/out` inside the sandbox, and any files retrieved via `sandbox_download`, remain accessible on the host.
- **Threading `sandbox_id` through.** `run.sh` extracts the `sandbox_id` from step 1's response and threads it through steps 2–6 with `jq`. The placeholder `"SANDBOX_ID_HERE"` documented in the bullet above is what `run.sh` substitutes — a hand-driven client can substitute it the same way.

## Setup

Create a sample CSV file to upload:

```bash
mkdir -p /tmp/demesne-example
printf 'name,value\nalpha,10\nbeta,20\ngamma,30\n' > /tmp/demesne-example/data.csv
```

## Run it

```bash
bash run.sh
```

`run.sh` uses `jq` to extract the `sandbox_id` from step 1's response and substitute it automatically into the subsequent calls.

## See also

- [`../../docs/reference/tools/sandbox_create.md`](../../docs/reference/tools/sandbox_create.md)
- [`../../docs/reference/tools/sandbox_exec.md`](../../docs/reference/tools/sandbox_exec.md)
- [`../../docs/reference/tools/sandbox_upload.md`](../../docs/reference/tools/sandbox_upload.md)
- [`../../docs/reference/tools/sandbox_download.md`](../../docs/reference/tools/sandbox_download.md)
- [`../../docs/reference/tools/sandbox_destroy.md`](../../docs/reference/tools/sandbox_destroy.md)
