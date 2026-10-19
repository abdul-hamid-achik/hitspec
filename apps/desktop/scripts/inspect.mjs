/**
 * Pure binary inspection helpers used by the packaging checks (and unit
 * tested with synthetic buffers).
 */

/** Which CPU/OS is this executable for? Returns {format, arch} or null. */
export function detectBinary(buf) {
  if (!buf || buf.length < 64) return null;

  // ELF: 7f 45 4c 46, e_machine (u16 LE at 18)
  if (buf.readUInt32BE(0) === 0x7f454c46) {
    const machine = buf.readUInt16LE(18);
    const arch = { 0x3e: 'x64', 0xb7: 'arm64', 0x03: 'ia32', 0x28: 'arm' }[machine];
    return { format: 'elf', arch: arch ?? `unknown(0x${machine.toString(16)})` };
  }

  // PE: "MZ", e_lfanew (u32 LE at 0x3c) -> "PE\0\0", Machine (u16 LE)
  if (buf[0] === 0x4d && buf[1] === 0x5a) {
    const pe = buf.readUInt32LE(0x3c);
    if (pe + 6 > buf.length || buf.readUInt32LE(pe) !== 0x00004550) return null;
    const machine = buf.readUInt16LE(pe + 4);
    const arch = { 0x8664: 'x64', 0xaa64: 'arm64', 0x14c: 'ia32' }[machine];
    return { format: 'pe', arch: arch ?? `unknown(0x${machine.toString(16)})` };
  }

  // Mach-O 64-bit little endian (cf fa ed fe): cputype u32 LE at 4
  if (buf.readUInt32LE(0) === 0xfeedfacf) {
    const cpu = buf.readUInt32LE(4);
    const arch = { 0x0100000c: 'arm64', 0x01000007: 'x64' }[cpu];
    return { format: 'macho', arch: arch ?? `unknown(0x${cpu.toString(16)})` };
  }
  // Fat/universal binary: report as such so callers can reject or accept it.
  if (buf.readUInt32BE(0) === 0xcafebabe) return { format: 'macho', arch: 'universal' };

  return null;
}

const SENTINEL = Buffer.from('dL7pKGdnNz796PbbjQWNKmHXBZaB9tsX');
export const FUSE_NAMES = [
  'RunAsNode',
  'EnableCookieEncryption',
  'EnableNodeOptionsEnvironmentVariable',
  'EnableNodeCliInspectArguments',
  'EnableEmbeddedAsarIntegrityValidation',
  'OnlyLoadAppFromAsar',
  'LoadBrowserProcessSpecificV8Snapshot',
  'GrantFileProtocolExtraPrivileges',
];

/**
 * Read the Electron fuse wire from an executable (the layout @electron/fuses
 * writes: sentinel, version byte, length byte, one byte per fuse where '1'
 * is enabled, '0' disabled and 'r' removed).
 * @returns {Record<string, boolean|null>|null} null when no wire is present
 */
export function readFuseWire(buf) {
  const at = buf.indexOf(SENTINEL);
  if (at === -1) return null;
  const base = at + SENTINEL.length;
  const length = buf[base + 1];
  const out = {};
  for (let i = 0; i < Math.min(length, FUSE_NAMES.length); i++) {
    const b = buf[base + 2 + i];
    out[FUSE_NAMES[i]] = b === 0x31 ? true : b === 0x30 ? false : null;
  }
  return out;
}

/** The fuse state a release build must have. */
export const EXPECTED_FUSES = {
  RunAsNode: false,
  EnableNodeOptionsEnvironmentVariable: false,
  EnableNodeCliInspectArguments: false,
  EnableEmbeddedAsarIntegrityValidation: true,
  OnlyLoadAppFromAsar: true,
  GrantFileProtocolExtraPrivileges: false,
};

/** @returns {string[]} human readable mismatches (empty when the wire is as expected) */
export function checkFuses(wire, expected = EXPECTED_FUSES) {
  if (!wire) return ['no Electron fuse wire found in the executable'];
  return Object.entries(expected)
    .filter(([name, want]) => wire[name] !== want)
    .map(([name, want]) => `${name} should be ${want ? 'enabled' : 'disabled'} but is ${wire[name] === null ? 'removed/unknown' : wire[name] ? 'enabled' : 'disabled'}`);
}
