// Wraps codex so a ChatGPT usage-limit window is survived rather than fatal:
// the wrapper waits for the reset time reported in the failure message — even
// when that message also carries "Upgrade to Pro"/"purchase more credits" upsell
// text — and resumes the same thread. When the message has no parseable reset
// time it falls back to exponential backoff. stdout must stay the verbatim JSONL
// stream so the caller's transcript is uninterrupted across retries.
//
// Usage: node codex-retry.mjs <model> <prompt>
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { StringDecoder } from 'node:string_decoder';

// Retry budget: survive a usage-limit / billing window for up to six hours,
// with the first backoff retry after five minutes (used when the failure
// message carries no explicit reset time). maxTotalWaitSecs is the
// hard ceiling on total time spent waiting across all attempts; maxAttempts
// is high enough that the time budget, not the attempt count, is the binding limit.
const maxAttempts = envInt('RETRY_MAX_ATTEMPTS', 12);
const maxTotalWaitSecs = envInt('RETRY_MAX_TOTAL_WAIT_SECS', 21600);
const resetBufferSecs = envInt('RETRY_RESET_BUFFER_SECS', 30);
const backoffBaseSecs = envInt('RETRY_BACKOFF_BASE_SECS', 300);

const MAX_BACKOFF_SECS = 3600;
const CATALOG_URL = 'http://127.0.0.1:8086/backend-api/codex/models';
const USAGE_LIMIT_PHRASE = "You've hit your usage limit";

const [model, prompt] = process.argv.slice(2);

// envInt reads an integer from the environment; unset, empty or non-integer
// values fall back to fallback.
function envInt(name, fallback) {
  const raw = process.env[name]?.trim();
  return raw !== undefined && /^[-+]?\d+$/.test(raw) ? Number.parseInt(raw, 10) : fallback;
}

function log(message) {
  process.stderr.write(`codex-retry: ${message}\n`);
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
  installHandlers();
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

// writeCodexConfig installs CODEX_HOME/config.toml: the source config with
// model_catalog_json prepended. It must be a top-level key, so it has to land
// before the first `[table]` header (`[model_providers.demesne]`) in the
// copied config — a bare `key = value` appended after that header would be
// parsed as belonging to that table instead. Prepending guarantees top-level
// placement regardless of the file's internal structure.
function writeCodexConfig(codexHome, catalogPath) {
  const configPath = process.env.CODEX_CONFIG_PATH || '/in/.agent/config.toml';
  const source = fs.readFileSync(configPath, 'utf8');
  fs.writeFileSync(path.join(codexHome, 'config.toml'), `model_catalog_json = "${catalogPath}"\n${source}`);
}

// fetchModelCatalog downloads and validates the model catalog, installing it
// at catalogPath. It returns false (after logging) on any failure.
function fetchModelCatalog(catalogPath) {
  const key = process.env.DEMESNE_OPENAI_AGENT_KEY;
  if (!key) {
    log('DEMESNE_OPENAI_AGENT_KEY is required to fetch Codex model catalog');
    return false;
  }

  const tmpCatalog = `${catalogPath}.tmp-${process.pid}`;
  const discard = () => fs.rmSync(tmpCatalog, { force: true });
  // The sidecar proxy supplies the Codex version, User-Agent and
  // client_version query, so none are sent from here.
  const curl = spawnSync(
    'curl',
    ['-sS', '-H', `Authorization: Bearer ${key}`, '-o', tmpCatalog, '-w', '%{http_code}', CATALOG_URL],
    { stdio: ['ignore', 'pipe', 'inherit'], encoding: 'utf8' },
  );
  if (curl.error || curl.status !== 0) {
    discard();
    log(`failed to fetch Codex model catalog from sidecar (curl exit ${curl.status ?? 127})`);
    return false;
  }

  const status = curl.stdout.trim();
  if (!/^2\d\d$/.test(status)) {
    log(`failed to fetch Codex model catalog from sidecar: HTTP ${status}`);
    const head = fs.readFileSync(tmpCatalog, 'utf8').split('\n').slice(0, 5).join('\n');
    if (head.trim() !== '') {
      process.stderr.write(`${head}\n`);
    }
    discard();
    return false;
  }

  if (!hasModels(tmpCatalog)) {
    discard();
    log('sidecar returned unusable Codex model catalog JSON; expected object with non-empty models array');
    return false;
  }
  fs.renameSync(tmpCatalog, catalogPath);
  return true;
}

function hasModels(file) {
  try {
    const body = JSON.parse(fs.readFileSync(file, 'utf8'));
    return Array.isArray(body?.models) && body.models.length > 0;
  } catch {
    return false;
  }
}

const MONTHS = ['jan', 'feb', 'mar', 'apr', 'may', 'jun', 'jul', 'aug', 'sep', 'oct', 'nov', 'dec'];

// hourOfDay converts a 12-hour clock reading to 24-hour time.
function hourOfDay(hour, meridiem) {
  return (Number.parseInt(hour, 10) % 12) + (meridiem.toUpperCase() === 'PM' ? 12 : 0);
}

// parseResetTime parses the "try again at" text as local time, returning an
// epoch in seconds or null. Accepted forms are a bare time ("1:00 AM", meaning
// today) and a full date ("Jan 1, 2999 1:00 AM"), after ordinal suffixes
// ("1st") are stripped; anything else is left to Date.parse.
function parseResetTime(text) {
  const cleaned = text.replace(/(\d+)(st|nd|rd|th)/g, '$1').trim();

  let match = /^(\d{1,2}):(\d{2})\s*(AM|PM)$/i.exec(cleaned);
  if (match) {
    const date = new Date();
    date.setHours(hourOfDay(match[1], match[3]), Number.parseInt(match[2], 10), 0, 0);
    return Math.floor(date.getTime() / 1000);
  }

  match = /^([A-Za-z]{3,}) (\d{1,2}),? (\d{4}),? (\d{1,2}):(\d{2})\s*(AM|PM)$/.exec(cleaned);
  const month = match ? MONTHS.indexOf(match[1].slice(0, 3).toLowerCase()) : -1;
  if (match && month >= 0) {
    const date = new Date(
      Number.parseInt(match[3], 10),
      month,
      Number.parseInt(match[2], 10),
      hourOfDay(match[4], match[6]),
      Number.parseInt(match[5], 10),
    );
    return Math.floor(date.getTime() / 1000);
  }

  const parsed = Date.parse(cleaned);
  return Number.isNaN(parsed) ? null : Math.floor(parsed / 1000);
}

// resetWait returns the seconds to wait for the reset time in a usage-limit
// message (plus the buffer, at least the buffer), or null when the message has
// no parseable reset time.
function resetWait(message) {
  const match = /[Tt]ry again at ([^".]*)/.exec(message);
  if (!match) {
    return null;
  }
  const resetEpoch = parseResetTime(match[1]);
  if (resetEpoch === null) {
    return null;
  }
  const now = Math.floor(Date.now() / 1000);
  return Math.max(resetEpoch - now + resetBufferSecs, resetBufferSecs);
}

async function main() {
  const codexHome = path.join(process.cwd(), '.codex');
  const catalogPath = path.join(codexHome, 'model-catalog.json');
  process.env.CODEX_HOME = codexHome;
  fs.mkdirSync(codexHome, { recursive: true });
  writeCodexConfig(codexHome, catalogPath);
  if (!fetchModelCatalog(catalogPath)) {
    process.exitCode = 1;
    return;
  }

  // Installed only now: the catalog fetch blocks the event loop, and until
  // there is something to forward to a signal should just terminate us.
  for (const name of ['SIGTERM', 'SIGINT']) {
    process.on(name, () => handleSignal(name));
  }

  let totalWaited = 0;
  let threadId = '';
  let rc = 0;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    let quotaMessage = null;
    const onEvent = (event) => {
      if (!threadId && event.type === 'thread.started' && typeof event.thread_id === 'string') {
        threadId = event.thread_id;
      }
      const message = event.error?.message;
      if (!quotaMessage && event.type === 'turn.failed' && typeof message === 'string' && message.includes(USAGE_LIMIT_PHRASE)) {
        quotaMessage = message;
      }
    };

    const args =
      attempt === 1
        ? ['exec', '--json', '-s', 'danger-full-access', '--skip-git-repo-check', '-C', process.cwd(), '-m', model, '--', prompt]
        : ['exec', 'resume', threadId, '--json', '-m', model, '--skip-git-repo-check', 'continue'];
    rc = await runChild(['codex', ...args], 'ignore', onEvent);

    if (signalled || quotaMessage === null) {
      break;
    }
    if (!threadId) {
      log('usage limit hit but no thread_id captured; cannot resume');
      break;
    }
    if (attempt >= maxAttempts) {
      log(`usage limit hit and max attempts (${maxAttempts}) reached; giving up`);
      break;
    }

    // Prefer the reset time the failure message reports; ChatGPT usage-limit
    // messages routinely carry "Upgrade to Pro"/"purchase more credits" upsell
    // text alongside a real "try again at <time>", so honour the reset rather
    // than treating the upsell as fatal. With no parseable reset time, fall
    // back to exponential backoff instead of giving up.
    let wait = resetWait(quotaMessage);
    let reason = 'reported reset';
    if (wait === null) {
      wait = backoffSecs(attempt);
      reason = 'exponential backoff (no reset time in message)';
    }

    const remaining = maxTotalWaitSecs - totalWaited;
    if (wait > remaining) {
      log(`next wait of ${wait}s is beyond the wait budget (${remaining}s remaining); giving up`);
      break;
    }

    log(`usage limit hit, attempt ${attempt}/${maxAttempts}, sleeping ${wait}s (${reason}), resuming thread ${threadId}`);
    if (!(await sleep(wait))) return;
    totalWaited += wait;
  }

  process.exitCode = rc;
}

main().catch((err) => {
  log(err.message);
  process.exitCode = 1;
});
