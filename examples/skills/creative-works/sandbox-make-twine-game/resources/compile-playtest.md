# Compile and playtest contract

Run Tweego from `/workspace/story` with `image=twine`; require exit code 0 and `/workspace/story/index.html`. Run the browser harness with the same image and require `playtest-report.json`, a start screenshot, one screenshot per ending, no console errors, and every planned passage reachable. Write failures as structured report entries.
