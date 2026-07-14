# Playtest contract

Provide five named scenarios. Each supplies start state, actions, expected visible state, and expected completion state. The harness writes `playtest-report.json` with scenario name, pass/fail, error, and screenshot path. Capture `start.png` and one PNG for every distinct ending. Fail when any scenario, required screenshot, or build artifact is absent.
