import { execFile } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Orphan handling for the `hitspec serve --api-only` child.
 *
 * If Electron is killed with SIGKILL (or crashes hard) nothing runs on the
 * way out. The server is started with `--parent-pid` and exits by itself, but
 * an older CLI ignores that, so the child is also remembered in a pidfile and
 * reaped on the next launch (after verifying its identity).
 */

export function writePidFile(file, record) {
  try {
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, `${JSON.stringify(record)}\n`, 'utf8');
  } catch {
    // best effort: losing the pidfile only loses orphan cleanup
  }
}

export function readPidFile(file) {
  try {
    const rec = JSON.parse(fs.readFileSync(file, 'utf8'));
    return Number.isInteger(rec?.pid) && rec.pid > 1 ? rec : null;
  } catch {
    return null;
  }
}

/** Remove the pidfile, but only if it still describes `pid` (a newer child may own it). */
export function removePidFile(file, pid) {
  try {
    const rec = readPidFile(file);
    if (rec && pid !== undefined && rec.pid !== pid) return;
    fs.rmSync(file, { force: true });
  } catch {
    // already gone
  }
}

function defaultIsAlive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return err?.code === 'EPERM';
  }
}

/**
 * The full command line of `pid`, or null when it cannot be read. Identity
 * checks need the whole command line, not an image name: on Windows
 * `tasklist` only gives "hitspec.exe", which is also a user's `hitspec run`.
 * `run` is injectable for tests.
 */
export function defaultGetCommand(pid, platform = process.platform, run = execFile) {
  return new Promise((resolve) => {
    if (platform === 'win32') {
      // pid comes from a validated integer in the pid file.
      const script = `(Get-CimInstance Win32_Process -Filter 'ProcessId=${Number(pid)}').CommandLine`;
      run('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], { windowsHide: true, timeout: 10000 }, (err, stdout) => {
        if (err) return resolve(null);
        resolve(String(stdout).trim() || null);
      });
      return;
    }
    run('ps', ['-p', String(pid), '-o', 'command='], (err, stdout) => {
      if (err) return resolve(null);
      resolve(String(stdout).trim() || null);
    });
  });
}

/**
 * Is `command` (the process's full command line) really a hitspec API server?
 * Guards against PID reuse: we must never signal an unrelated process, nor
 * another hitspec invocation. The executable may be quoted (Windows).
 */
export function looksLikeHitspecServe(command) {
  if (!command) return false;
  return /(^|[\\/\s"])hitspec(\.exe)?"?\s/i.test(command) && /\sserve\s/.test(command) && /--api-only/.test(command);
}

/** The server we recorded listened on `port`: a reused pid running another server must not match. */
function matchesRecord(command, rec) {
  if (!Number.isInteger(rec?.port)) return true;
  return new RegExp(`--port[= ]${rec.port}(\\s|$)`).test(command);
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/**
 * Kill a stale API server recorded by a previous run.
 * @returns {Promise<{reaped:boolean, pid?:number, reason?:string}>}
 */
export async function reapStaleServer(file, deps = {}) {
  const isAlive = deps.isAlive ?? defaultIsAlive;
  const getCommand = deps.getCommand ?? defaultGetCommand;
  const kill = deps.kill ?? ((pid, sig) => process.kill(pid, sig));
  const graceMs = deps.graceMs ?? 3000;

  const rec = readPidFile(file);
  if (!rec) {
    removePidFile(file);
    return { reaped: false, reason: 'no pidfile' };
  }
  if (rec.pid === process.pid || !isAlive(rec.pid)) {
    removePidFile(file);
    return { reaped: false, pid: rec.pid, reason: 'not running' };
  }
  const command = await getCommand(rec.pid);
  if (!looksLikeHitspecServe(command) || !matchesRecord(command, rec)) {
    removePidFile(file);
    return { reaped: false, pid: rec.pid, reason: 'pid was reused by another program' };
  }

  try {
    kill(rec.pid, 'SIGTERM');
  } catch {
    // raced with its exit
  }
  const deadline = Date.now() + graceMs;
  while (Date.now() < deadline && isAlive(rec.pid)) await sleep(50);
  if (isAlive(rec.pid)) {
    try {
      kill(rec.pid, 'SIGKILL');
    } catch {
      // gone
    }
  }
  removePidFile(file);
  return { reaped: true, pid: rec.pid };
}
