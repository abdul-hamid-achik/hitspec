/**
 * The platforms hitspec Studio ships for. `id` is electron-builder's
 * `${os}-${arch}` macro value; `goos`/`goarch` drive the Go cross-compile.
 * The Go CLI is pure Go (CGO_ENABLED=0, modernc sqlite), so every target can
 * be built from any host.
 */
export const TARGETS = [
  { id: 'mac-arm64', os: 'mac', goos: 'darwin', goarch: 'arm64', file: 'hitspec' },
  { id: 'mac-x64', os: 'mac', goos: 'darwin', goarch: 'amd64', file: 'hitspec' },
  { id: 'win-x64', os: 'win', goos: 'windows', goarch: 'amd64', file: 'hitspec.exe' },
  { id: 'linux-x64', os: 'linux', goos: 'linux', goarch: 'amd64', file: 'hitspec' },
];

const NODE_PLATFORM_TO_OS = { darwin: 'mac', win32: 'win', linux: 'linux' };

/** The target for the machine running this script (undefined for unsupported hosts). */
export function hostTarget(platform = process.platform, arch = process.arch) {
  const os = NODE_PLATFORM_TO_OS[platform];
  return TARGETS.find((t) => t.os === os && t.id.endsWith(`-${arch}`));
}

/** Every target packaged by `electron-builder` on this OS. */
export function targetsForOs(os) {
  return TARGETS.filter((t) => t.os === os);
}

/**
 * Resolve CLI arguments to targets.
 *   (none)            the host target only (development)
 *   --all             every target
 *   --dist            every target for the host OS (what `npm run dist` packages)
 *   --targets a,b     explicit ids
 */
export function selectTargets(argv, platform = process.platform, arch = process.arch) {
  const flag = (name) => argv.includes(name);
  if (flag('--all')) return TARGETS;
  if (flag('--dist')) return targetsForOs(NODE_PLATFORM_TO_OS[platform]);
  const i = argv.indexOf('--targets');
  if (i !== -1) {
    const ids = String(argv[i + 1] ?? '').split(',').map((x) => x.trim()).filter(Boolean);
    const unknown = ids.filter((id) => !TARGETS.some((t) => t.id === id));
    if (!ids.length || unknown.length) {
      throw new Error(`unknown target(s) ${unknown.join(', ') || '(none given)'}; choose from ${TARGETS.map((t) => t.id).join(', ')}`);
    }
    return TARGETS.filter((t) => ids.includes(t.id));
  }
  const host = hostTarget(platform, arch);
  if (!host) throw new Error(`no hitspec Studio target for ${platform}/${arch}`);
  return [host];
}
