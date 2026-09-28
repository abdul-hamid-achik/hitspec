import { EventEmitter } from 'node:events';
import { spawn } from 'node:child_process';
import crypto from 'node:crypto';
import net from 'node:net';
import WebSocket from 'ws';

const MAX_LOG_LINES = 1000;
const READY_TIMEOUT_MS = 30_000;
const READY_POLL_MS = 100;
const STOP_GRACE_MS = 4_000;

/** @returns {Promise<number>} an unused TCP port on the loopback interface */
function pickFreePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.once('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const { port } = srv.address();
      srv.close(() => resolve(port));
    });
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/**
 * Owns the `hitspec serve --api-only` child process: lifecycle, auth token,
 * REST proxying and the realtime WebSocket bridge.
 *
 * The token never reaches the renderer — every HTTP call is made here on the
 * renderer's behalf over IPC, which also sidesteps CORS and origin checks on
 * the Go side.
 *
 * Events: `status` ({state, ...}), `event` (WS payload), `log` (line object).
 */
export class Backend extends EventEmitter {
  constructor({ resolveBinary, env = process.env, log = console } = {}) {
    super();
    this.resolveBinary = resolveBinary;
    this.env = env;
    this.log = log;

    this.state = 'idle'; // idle | starting | ready | error | stopped
    this.error = null;
    this.workDir = null;
    this.port = null;
    this.token = null;
    this.version = '';
    this.binary = null;
    this.binarySource = null;

    this.child = null;
    this.ws = null;
    this.wsRetry = 0;
    this.stopping = false;
    this.lines = [];
  }

  /** Serializable snapshot for the renderer's status bar. */
  snapshot() {
    return {
      state: this.state,
      error: this.error,
      workDir: this.workDir,
      port: this.state === 'ready' ? this.port : null,
      version: this.version,
      binary: this.binary,
      binarySource: this.binarySource,
      pid: this.child?.pid ?? null,
    };
  }

  setStatus(patch = {}) {
    Object.assign(this, patch);
    this.emit('status', this.snapshot());
  }

  pushLine(stream, text) {
    for (const raw of text.split(/\r?\n/)) {
      if (!raw.trim()) continue;
      const line = { stream, text: raw, at: new Date().toISOString() };
      try {
        const parsed = JSON.parse(raw);
        if (parsed && typeof parsed === 'object') {
          line.level = parsed.level;
          line.msg = parsed.msg;
          line.fields = Object.fromEntries(
            Object.entries(parsed).filter(([k]) => !['time', 'level', 'msg'].includes(k)),
          );
        }
      } catch {
        // plain text log line
      }
      this.lines.push(line);
      if (this.lines.length > MAX_LOG_LINES) this.lines.splice(0, this.lines.length - MAX_LOG_LINES);
      this.emit('log', line);
    }
  }

  getLogs(limit = 200) {
    return this.lines.slice(Math.max(0, this.lines.length - limit));
  }

  /**
   * Start (or restart) the API server for a workspace directory.
   * @param {string} workDir
   * @param {{watch?:boolean, readOnly?:boolean, allowShell?:boolean, env?:string, logLevel?:string, childEnv?:Record<string,string>}} [options]
   *   `options.env` selects the hitspec environment (--env); `options.childEnv`
   *   is merged into the child process environment (the API token always wins).
   */
  async start(workDir, options = {}) {
    if (!workDir) throw new Error('workDir is required');
    await this.stop();

    this.stopping = false;
    this.lines = [];
    this.error = null;
    this.workDir = workDir;
    this.version = '';
    this.setStatus({ state: 'starting' });

    let resolved;
    if (typeof this.resolveBinary === 'function') {
      resolved = await this.resolveBinary();
    }
    if (!resolved || resolved.error) {
      const message = resolved?.error ?? 'hitspec binary resolver unavailable';
      this.setStatus({ state: 'error', error: message });
      throw new Error(message);
    }

    this.binary = resolved.path;
    this.binarySource = resolved.source;
    this.token = crypto.randomBytes(24).toString('base64url');

    const attempt = async (retriesLeft) => {
      const port = await pickFreePort();
      this.port = port;

      const args = [
        'serve',
        '--api-only',
        '--host', '127.0.0.1',
        '--port', String(port),
        '--api-token', this.token,
        '--log-format', 'json',
        '--log-level', options.logLevel ?? 'info',
      ];
      if (options.watch === false) args.push('--watch=false');
      if (options.readOnly) args.push('--read-only');
      if (options.allowShell) args.push('--allow-shell');
      if (options.env) args.push('--env', options.env);
      args.push(workDir);

      const child = spawn(this.binary, args, {
        cwd: workDir,
        env: { ...process.env, ...(options.childEnv ?? {}), HITSPEC_API_TOKEN: this.token },
        stdio: ['ignore', 'pipe', 'pipe'],
        windowsHide: true,
      });
      this.child = child;

      child.stdout.setEncoding('utf8');
      child.stderr.setEncoding('utf8');
      child.stdout.on('data', (d) => this.pushLine('stdout', d));
      child.stderr.on('data', (d) => this.pushLine('stderr', d));

      const exited = new Promise((resolve) => {
        child.once('exit', (code, signal) => resolve({ code, signal }));
      });

      try {
        await this.waitReady(exited);
      } catch (err) {
        this.killChild(child);
        await exited;
        this.child = null;
        const msg = String(err?.message ?? err);
        if (retriesLeft > 0 && /address already in use|bind/i.test(msg)) {
          return attempt(retriesLeft - 1);
        }
        throw err;
      }

      return child;
    };

    let child;
    try {
      child = await attempt(3);
    } catch (err) {
      const message = err?.message ? String(err.message) : String(err);
      this.child = null;
      this.setStatus({ state: 'error', error: message });
      throw new Error(message);
    }

    child.once('exit', (code, signal) => {
      if (this.child !== child) return;
      this.child = null;
      this.closeSocket();
      if (this.stopping) {
        this.setStatus({ state: 'stopped' });
      } else {
        this.setStatus({
          state: 'error',
          error: `hitspec API server exited unexpectedly (code ${code ?? 'null'}, signal ${signal ?? 'none'})`,
        });
      }
    });

    const info = await this.request({ method: 'GET', path: '/system/info' }).catch(() => null);
    this.version = info?.data?.version ?? '';
    this.connectSocket();
    this.setStatus({ state: 'ready', error: null });
    return this.snapshot();
  }

  async waitReady(exited) {
    const deadline = Date.now() + READY_TIMEOUT_MS;
    let lastError = 'server did not become ready';

    while (Date.now() < deadline) {
      const done = await Promise.race([exited.then((r) => r), sleep(READY_POLL_MS).then(() => null)]);
      if (done) {
        throw new Error(
          `hitspec API server exited before becoming ready (code ${done.code ?? 'null'}, signal ${done.signal ?? 'none'}). ` +
          `Last output:\n${this.getLogs(15).map((l) => l.text).join('\n') || '(no output)'}`,
        );
      }
      try {
        const res = await fetch(this.url('/system/info'), {
          headers: { Authorization: `Bearer ${this.token}` },
          signal: AbortSignal.timeout(2000),
        });
        if (res.ok) return;
        lastError = `readiness probe returned HTTP ${res.status}`;
      } catch (err) {
        lastError = err?.message ? String(err.message) : String(err);
      }
      await sleep(READY_POLL_MS);
    }
    throw new Error(`timed out waiting for hitspec API server: ${lastError}`);
  }

  url(pathname, query) {
    const u = new URL(`/api/v1${pathname}`, `http://127.0.0.1:${this.port}`);
    if (query) {
      for (const [k, v] of Object.entries(query)) {
        if (v === undefined || v === null || v === '') continue;
        u.searchParams.set(k, String(v));
      }
    }
    return u.toString();
  }

  /**
   * Proxy a REST call to the Go server.
   * @param {{method?:string, path:string, body?:unknown, raw?:boolean, query?:Record<string,unknown>, accept?:string}} req
   * @returns {Promise<{ok:boolean, status:number, data:unknown, text:string, error?:string}>}
   */
  async request(req) {
    if (this.state !== 'ready') {
      return { ok: false, status: 0, data: null, text: '', error: `backend is ${this.state}` };
    }
    const method = (req.method ?? 'GET').toUpperCase();
    const headers = { Authorization: `Bearer ${this.token}` };
    let body;
    if (req.body !== undefined && req.body !== null) {
      if (req.raw || typeof req.body === 'string') {
        body = String(req.body);
        headers['Content-Type'] = req.accept ?? 'text/plain; charset=utf-8';
      } else {
        body = JSON.stringify(req.body);
        headers['Content-Type'] = 'application/json';
      }
    }

    let res;
    try {
      res = await fetch(this.url(req.path, req.query), { method, headers, body, signal: req.timeout ? AbortSignal.timeout(req.timeout) : undefined });
    } catch (err) {
      return { ok: false, status: 0, data: null, text: '', error: err?.message ?? String(err) };
    }

    const text = await res.text();
    let data = null;
    const contentType = res.headers.get('content-type') ?? '';
    if (text) {
      if (req.raw || contentType.includes('text/plain')) {
        data = text;
      } else if (contentType.includes('application/json')) {
        try {
          data = JSON.parse(text);
        } catch {
          data = text;
        }
      } else {
        try {
          data = JSON.parse(text);
        } catch {
          data = text;
        }
      }
    }

    const out = { ok: res.ok, status: res.status, data, text };
    if (!res.ok) {
      out.error =
        (data && typeof data === 'object' && (data.message || data.error)) ||
        text ||
        `HTTP ${res.status}`;
    }
    return out;
  }

  connectSocket() {
    this.closeSocket();
    if (this.state !== 'ready' && this.state !== 'starting') return;

    const u = new URL(`/api/v1/ws`, `http://127.0.0.1:${this.port}`);
    u.searchParams.set('token', this.token);

    let ws;
    try {
      ws = new WebSocket(u.toString(), { handshakeTimeout: 5000 });
    } catch (err) {
      this.pushLine('stderr', `websocket error: ${err?.message ?? err}`);
      return;
    }
    this.ws = ws;

    ws.on('message', (buf) => {
      const text = buf.toString();
      try {
        const msg = JSON.parse(text);
        this.emit('event', msg);
      } catch {
        this.emit('event', { type: 'raw', payload: text, timestamp: new Date().toISOString() });
      }
    });
    ws.on('open', () => {
      this.wsRetry = 0;
      this.emit('socket', { connected: true });
    });
    ws.on('error', (err) => {
      this.pushLine('stderr', `websocket error: ${err?.message ?? err}`);
    });
    ws.on('close', () => {
      this.emit('socket', { connected: false });
      if (this.ws !== ws) return;
      this.ws = null;
      if (this.stopping || !this.child) return;
      const delay = Math.min(5000, 250 * 2 ** this.wsRetry++);
      setTimeout(() => {
        if (!this.stopping && this.child) this.connectSocket();
      }, delay);
    });
  }

  closeSocket() {
    const ws = this.ws;
    this.ws = null;
    if (!ws) return;
    try {
      ws.removeAllListeners('close');
      ws.terminate();
    } catch {
      // already closed
    }
  }

  killChild(child) {
    try {
      if (child.exitCode === null && child.signalCode === null) child.kill('SIGTERM');
    } catch {
      // already gone
    }
  }

  async stop() {
    this.stopping = true;
    this.closeSocket();
    const child = this.child;
    this.child = null;
    if (!child) {
      if (this.state !== 'idle' && this.state !== 'error') this.setStatus({ state: 'stopped' });
      return;
    }
    this.killChild(child);
    const exited = new Promise((resolve) => child.once('exit', resolve));
    const timedOut = await Promise.race([exited.then(() => false), sleep(STOP_GRACE_MS).then(() => true)]);
    if (timedOut) {
      try {
        child.kill('SIGKILL');
      } catch {
        // already gone
      }
      await Promise.race([exited, sleep(1000)]);
    }
    this.setStatus({ state: 'stopped' });
  }
}
