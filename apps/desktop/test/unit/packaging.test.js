import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';

import { checkFuses, detectBinary, EXPECTED_FUSES, FUSE_NAMES, readFuseWire } from '../../scripts/inspect.mjs';
import { TARGETS, hostTarget, selectTargets, targetsForOs } from '../../scripts/targets.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const builderYml = fs.readFileSync(path.join(root, 'electron-builder.yml'), 'utf8');

/* ---- targets -------------------------------------------------------------- */

test('every target maps to a Go OS/arch and an electron-builder directory id', () => {
  for (const t of TARGETS) {
    assert.match(t.id, /^(mac|win|linux)-(x64|arm64)$/);
    assert.equal(t.file, t.os === 'win' ? 'hitspec.exe' : 'hitspec');
    assert.ok(['darwin', 'windows', 'linux'].includes(t.goos));
    assert.ok(['amd64', 'arm64'].includes(t.goarch));
  }
});

test('selectTargets: host by default, --dist per OS, --all, --targets', () => {
  assert.deepEqual(selectTargets([], 'darwin', 'arm64').map((t) => t.id), ['mac-arm64']);
  assert.deepEqual(selectTargets([], 'win32', 'x64').map((t) => t.id), ['win-x64']);
  assert.deepEqual(selectTargets(['--dist'], 'darwin', 'arm64').map((t) => t.id), ['mac-arm64', 'mac-x64']);
  assert.deepEqual(selectTargets(['--dist'], 'linux', 'x64').map((t) => t.id), ['linux-x64']);
  assert.equal(selectTargets(['--all']).length, TARGETS.length);
  assert.deepEqual(selectTargets(['--targets', 'win-x64,mac-x64']).map((t) => t.id), ['mac-x64', 'win-x64']);
  assert.throws(() => selectTargets(['--targets', 'solaris-sparc']), /unknown target/);
  assert.throws(() => selectTargets([], 'freebsd', 'x64'), /no hitspec Studio target/);
  assert.equal(hostTarget('darwin', 'x64').goarch, 'amd64');
});

test('electron-builder packs a per-target binary and covers every packaged arch', () => {
  // The macros are what stop an x64 build from carrying the arm64 CLI.
  assert.match(builderYml, /from: build\/bin\/\$\{os\}-\$\{arch\}/);
  assert.ok(!/from: build\/bin\s*$/m.test(builderYml), 'no flat host-binary copy');
  assert.match(builderYml, /- hitspec\.exe/);

  // Every os/arch listed in the builder config has a Go build for it.
  const section = (name) => {
    const m = new RegExp(`^${name}:\\n((?:[ \\t].*\\n|\\n)*)`, 'm').exec(builderYml);
    assert.ok(m, `${name} section`);
    return m[1];
  };
  for (const [name, os] of [['mac', 'mac'], ['win', 'win'], ['linux', 'linux']]) {
    const archs = [...section(name).matchAll(/arch: \[([^\]]+)\]/g)].flatMap((m) => m[1].split(',').map((a) => a.trim()));
    assert.ok(archs.length > 0, `${name}: arch list must be explicit`);
    for (const arch of archs) {
      assert.ok(TARGETS.some((t) => t.id === `${os}-${arch}`), `no Go target for ${os}-${arch}`);
    }
    assert.deepEqual(
      [...new Set(archs)].sort(),
      targetsForOs(os).map((t) => t.id.split('-')[1]).sort(),
      `${name}: builder archs and Go targets should match`,
    );
  }
});

test('the hardened fuses are configured', () => {
  const block = /^electronFuses:\n((?:[ \t].*\n)+)/m.exec(builderYml)?.[1] ?? '';
  for (const [key, want] of [
    ['runAsNode', false],
    ['enableNodeOptionsEnvironmentVariable', false],
    ['enableNodeCliInspectArguments', false],
    ['enableEmbeddedAsarIntegrityValidation', true],
    ['onlyLoadAppFromAsar', true],
  ]) {
    assert.match(block, new RegExp(`^  ${key}: ${want}$`, 'm'), key);
  }
  assert.match(builderYml, /^asar: true$/m);
});

/* ---- binary + fuse inspection ------------------------------------------------ */

test('detectBinary reads Mach-O, PE and ELF architectures', () => {
  const macho = (cpu) => { const b = Buffer.alloc(64); b.writeUInt32LE(0xfeedfacf, 0); b.writeUInt32LE(cpu, 4); return b; };
  assert.deepEqual(detectBinary(macho(0x0100000c)), { format: 'macho', arch: 'arm64' });
  assert.deepEqual(detectBinary(macho(0x01000007)), { format: 'macho', arch: 'x64' });

  const pe = (machine) => {
    const b = Buffer.alloc(256);
    b[0] = 0x4d; b[1] = 0x5a; b.writeUInt32LE(0x80, 0x3c);
    b.writeUInt32LE(0x00004550, 0x80); b.writeUInt16LE(machine, 0x84);
    return b;
  };
  assert.deepEqual(detectBinary(pe(0x8664)), { format: 'pe', arch: 'x64' });
  assert.deepEqual(detectBinary(pe(0xaa64)), { format: 'pe', arch: 'arm64' });

  const elf = (machine) => { const b = Buffer.alloc(64); b.writeUInt32BE(0x7f454c46, 0); b.writeUInt16LE(machine, 18); return b; };
  assert.deepEqual(detectBinary(elf(0x3e)), { format: 'elf', arch: 'x64' });
  assert.deepEqual(detectBinary(elf(0xb7)), { format: 'elf', arch: 'arm64' });

  assert.equal(detectBinary(Buffer.from('#!/bin/sh\n'.padEnd(80, ' '))), null);
  assert.equal(detectBinary(Buffer.alloc(8)), null);
});

function fuseBuffer(bytes) {
  return Buffer.concat([
    Buffer.alloc(32, 7),
    Buffer.from('dL7pKGdnNz796PbbjQWNKmHXBZaB9tsX'),
    Buffer.from([1, bytes.length]),
    Buffer.from(bytes),
    Buffer.alloc(32, 7),
  ]);
}

test('readFuseWire/checkFuses report the state of each fuse', () => {
  const good = '00001110'.split('').map((c) => c.charCodeAt(0)); // see EXPECTED_FUSES order
  // order: RunAsNode, Cookie, NodeOptions, NodeCliInspect, AsarIntegrity, OnlyAsar, V8Snapshot, FileProtocol
  const wire = readFuseWire(fuseBuffer(good));
  assert.equal(Object.keys(wire).length, FUSE_NAMES.length);
  assert.equal(wire.RunAsNode, false);
  assert.equal(wire.EnableEmbeddedAsarIntegrityValidation, true);
  assert.deepEqual(checkFuses(wire), []);

  const stock = readFuseWire(fuseBuffer('10110001'.split('').map((c) => c.charCodeAt(0)))); // RunAsNode on, inspect on, ...
  const problems = checkFuses(stock);
  assert.ok(problems.some((p) => /^RunAsNode should be disabled/.test(p)));
  assert.ok(problems.some((p) => /EnableNodeCliInspectArguments should be disabled/.test(p)));
  assert.ok(problems.some((p) => /EnableEmbeddedAsarIntegrityValidation should be enabled/.test(p)));
  assert.ok(problems.some((p) => /OnlyLoadAppFromAsar should be enabled/.test(p)));

  assert.equal(readFuseWire(Buffer.alloc(100)), null);
  assert.deepEqual(checkFuses(null), ['no Electron fuse wire found in the executable']);
  assert.deepEqual(Object.keys(EXPECTED_FUSES).filter((k) => !FUSE_NAMES.includes(k)), []);
});
