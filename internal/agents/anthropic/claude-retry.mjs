// Wraps claude so a 5-hour rate-limit exhaustion waits for the reported reset
// time and resumes the same session; stdout must stay the verbatim stream-json
// stream so the caller's transcript is uninterrupted across retries.
//
// Usage: node claude-retry.mjs claude <args...>
import { spawn } from 'node:child_process';
import os from 'node:os';
import { StringDecoder } from 'node:string_decoder';

const MAX_BACKOFF_SECS = 3600;

const maxAttempts = envInt('RETRY_MAX_ATTEMPTS', 6);
const maxTotalWaitSecs = envInt('RETRY_MAX_TOTAL_WAIT_SECS', 39600);
const resetBufferSecs = envInt('RETRY_RESET_BUFFER_SECS', 30);
const backoffBaseSecs = envInt('RETRY_BACKOFF_BASE_SECS', 300);

// envInt reads an integer from the environment; unset, empty or non-integer
// values fall back to fallback.
function envInt(name, fallback) {
  const raw = process.env[name]?.trim();
  return raw !== undefined && /^[-+]?\d+$/.test(raw) ? Number.parseInt(raw, 10) : fallback;
}

function log(message) {
  process.stderr.write(`claude-retry: ${message}\n`);
}

// Signal state: the running child (forwarded to), the pending sleep (cancelled
// on a signal) and whether a signal or a closed stdout ended the run (no retry
// after it).
let currentChild = null;
let cancelSleep = null;
let signalled = false;

// handleSignal forwards a signal to the running child, which ends the retry
// loop, or cuts a pending sleep short with the conventional 128+signo exit
// code. With neither (the run is already over) the exit code stands.
function handleSignal(name) {
  signalled = true;
  if (currentChild) {
    currentChild.kill(name);
  } else if (cancelSleep) {
    process.exitCode = 128 + os.constants.signals[name];
    cancelSleep();
  }
}

// handleStdoutError stops the child when stdout is closed (EPIPE): nobody is
// left to stream to, so the run ends quietly with the child's exit code.
function handleStdoutError() {
  signalled = true;
  currentChild?.kill('SIGTERM');
  currentChild?.stdout.resume();
}

function installHandlers() {
  process.stdout.on('error', handleStdoutError);
  for (const name of ['SIGTERM', 'SIGINT']) {
    process.on(name, () => handleSignal(name));
  }
}

// lineSplitter returns a push/flush pair that feeds each line of the UTF-8
// text in the pushed chunks to onLine. Only newly received text is scanned for
// newlines; the pieces of an unfinished line are joined once, when it ends.
function lineSplitter(onLine) {
  const decoder = new StringDecoder('utf8');
  let pieces = [];
  const feed = (text) => {
    let start = 0;
    for (let end = text.indexOf('\n'); end !== -1; end = text.indexOf('\n', start)) {
      pieces.push(text.slice(start, end));
      onLine(pieces.join(''));
      pieces = [];
      start = end + 1;
    }
    if (start < text.length) pieces.push(text.slice(start));
  };
  return {
    push(chunk) {
      feed(decoder.write(chunk));
    },
    flush() {
      feed(decoder.end());
      onLine(pieces.join(''));
      pieces = [];
    },
  };
}

// parseEvent returns the JSON object on a line, or null for blank lines,
// non-JSON and non-object values (still passed through, just not inspected).
function parseEvent(line) {
  const text = line.trim();
  if (text === '') return null;
  try {
    const value = JSON.parse(text);
    return value !== null && typeof value === 'object' && !Array.isArray(value) ? value : null;
  } catch {
    return null;
  }
}

// runChild runs argv with the given stdin, streams its stdout verbatim to ours
// and hands every parsed event to onEvent. It resolves with the child's exit
// code once all of its stdout has been written.
function runChild(argv, stdin, onEvent) {
  return new Promise((resolve) => {
    const child = spawn(argv[0], argv.slice(1), { stdio: [stdin, 'pipe', 'inherit'] });
    currentChild = child;
    const splitter = lineSplitter((line) => {
      const event = parseEvent(line);
      if (event) onEvent(event);
    });

    child.stdout.on('data', (chunk) => {
      if (!process.stdout.destroyed && !process.stdout.write(chunk)) {
        child.stdout.pause();
        process.stdout.once('drain', () => child.stdout.resume());
      }
      splitter.push(chunk);
    });
    child.stdout.on('end', () => splitter.flush());

    child.on('error', (err) => {
      currentChild = null;
      log(`failed to run ${argv[0]}: ${err.message}`);
      resolve(127);
    });
    child.on('close', (code, signal) => {
      currentChild = null;
      resolve(code ?? 128 + os.constants.signals[signal]);
    });
  });
}

// sleep resolves true after secs, or false if a signal cuts it short.
function sleep(secs) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      cancelSleep = null;
      resolve(true);
    }, secs * 1000);
    cancelSleep = () => {
      clearTimeout(timer);
      cancelSleep = null;
      resolve(false);
    };
  });
}

// backoffSecs is base * 2^(attempt-1), capped at MAX_BACKOFF_SECS; a base
// already above the cap is used as-is for the first attempt.
function backoffSecs(attempt) {
  let wait = backoffBaseSecs;
  for (let i = 0; i < attempt - 1; i++) {
    wait *= 2;
    if (wait > MAX_BACKOFF_SECS) return MAX_BACKOFF_SECS;
  }
  return wait;
}

const positiveNumber = (v) => typeof v === 'number' && Number.isFinite(v) && v > 0;

// resetsAtOf returns the unix-seconds reset of the rejected window: the
// top-level resetsAt, else the one for rateLimitType inside unifiedWindows.
// Other windows (e.g. seven_day on a five_hour rejection) are never read.
function resetsAtOf(info) {
  if (positiveNumber(info.resetsAt)) return info.resetsAt;
  const windowReset = info.unifiedWindows?.[info.rateLimitType]?.resetsAt;
  return positiveNumber(windowReset) ? windowReset : null;
}

// resumeArgv rebuilds argv for a resume attempt: --resume <sessionId> goes
// right after the command, the -p/--print value becomes `continue`, and any
// existing resume/continue flags are dropped. It is idempotent.
function resumeArgv(argv, sessionId) {
  const rest = [];
  const tail = argv.slice(1);
  for (let i = 0; i < tail.length; i++) {
    const tok = tail[i];
    if (tok === '--resume' || tok === '-r') {
      i++;
    } else if (tok === '--continue' || tok === '-c') {
      // dropped
    } else if (tok === '-p' || tok === '--print') {
      rest.push(tok);
      if (i + 1 < tail.length) {
        rest.push('continue');
        i++;
      }
    } else {
      rest.push(tok);
    }
  }
  return [argv[0], '--resume', sessionId, ...rest];
}

// waitSecs is how long to wait before resuming: until the rejected window's
// reset plus a buffer, else exponential backoff.
function waitSecs(info, attempt) {
  const resetsAt = resetsAtOf(info);
  if (resetsAt === null) return backoffSecs(attempt);
  const wait = resetsAt - Math.floor(Date.now() / 1000) + resetBufferSecs;
  return wait < 0 ? resetBufferSecs : wait;
}

async function main() {
  let argv = process.argv.slice(2);
  let attempt = 1;
  let totalWaited = 0;
  let sessionId = '';
  let rc = 0;

  while (attempt <= maxAttempts) {
    if (attempt > 1) argv = resumeArgv(argv, sessionId);

    // Per-attempt quota signals: the first rejected rate_limit_event and any
    // result line carrying api_error_status 429.
    let rejected = null;
    let result429 = false;
    rc = await runChild(argv, 'inherit', (event) => {
      if (sessionId === '' && event.subtype === 'init' && typeof event.session_id === 'string') {
        sessionId = event.session_id;
      }
      if (rejected === null && event.type === 'rate_limit_event' && event.rate_limit_info?.status === 'rejected') {
        rejected = event.rate_limit_info;
      }
      if (event.type === 'result' && event.api_error_status === 429) result429 = true;
    });

    if (signalled || rejected === null || !result429) break;

    // Billing exclusion: out_of_credits is fatal, not a retryable quota window.
    if (rejected.overageDisabledReason === 'out_of_credits') {
      log('billing limit (out_of_credits), no retry');
      break;
    }
    if (sessionId === '') {
      log('quota exhausted but no session_id captured; cannot resume');
      break;
    }
    if (attempt >= maxAttempts) {
      log(`quota exhausted and max attempts (${maxAttempts}) reached`);
      break;
    }

    // Cap cumulative waiting under the sandbox command timeout: a reset beyond
    // the remaining budget (e.g. a 7-day window) can't be outlasted, so exit
    // instead of hanging.
    const wait = waitSecs(rejected, attempt);
    if (wait > maxTotalWaitSecs - totalWaited) {
      log('reset is beyond the wait budget; not retrying');
      break;
    }

    log(`quota exhausted, attempt ${attempt}/${maxAttempts}, sleeping ${wait}s until reset, resuming session ${sessionId}`);
    if (!(await sleep(wait))) return;
    totalWaited += wait;
    attempt++;
  }
  process.exitCode = rc;
}

installHandlers();
main().catch((err) => {
  log(err.message);
  process.exitCode = 1;
});
