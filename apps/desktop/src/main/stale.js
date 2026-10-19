import { execFile } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Orphan handling for the `hitspec serve --api-only` child.
 *
 * If Electron is killed with SIGKILL (or crashes hard) nothing runs on the
 * way out, and the Go server keeps its port and file watcher. Without a
 * cooperating child (see README "Known limitation") the best we can do is to
 * remember the child in a pidfile and reap it on the next launch.
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

function defaultGetCommand(pid, platform = process.platform) {
  return new Promise((resolve) => {
    if (platform === 'win32') {
      execFile('tasklist', ['/FI', `PID eq ${pid}`, '/FO', 'CSV', '/NH'], { windowsHide: true }, (err, stdout) => {
        if (err) return resolve(null);
        const m = /^"([^"]+)"/.exec(String(stdout).trim());
        resolve(m ? m[1] : null);
      });
      return;
    }
    execFile('ps', ['-p', String(pid), '-o', 'command='], (err, stdout) => {
      if (err) return resolve(null);
      resolve(String(stdout).trim() || null);
    });
  });
}

/**
 * Is `command` (ps output, or an image name on Windows) really a hitspec API
 * server? Guards against PID reuse: we must never signal an unrelated process.
 */
export function looksLikeHitspecServe(command) {
  if (!command) return false;
  if (/^hitspec(\.exe)?$/i.test(command.trim())) return true; // tasklist image name
  return /(^|[\\/\s])hitspec(\.exe)?\s/i.test(command) && /\sserve\s/.test(command) && /--api-only/.test(command);
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
  if (!looksLikeHitspecServe(await getCommand(rec.pid))) {
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
