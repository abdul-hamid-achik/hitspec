/**
 * CLI version gate. The desktop app relies on the hardening shipped in
 * hitspec v2.20.0 (JSON-only bodies, Host/Origin allowlist), so older servers
 * are reported to the user instead of failing in obscure ways.
 */

export const MIN_CLI_VERSION = '2.20.0';

/**
 * Parse "2.20.0", "v2.20.0", "2.20.0-3-gabc1234" or "2.21.0-rc.1".
 * @returns {[number, number, number]|null} null for "dev", commit hashes, empty.
 */
export function parseVersion(text) {
  const m = /^v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$/.exec(String(text ?? '').trim());
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
}

/** @returns {number|null} negative/0/positive like a comparator; null if either side is unparseable */
export function compareVersions(a, b) {
  const pa = parseVersion(a);
  const pb = parseVersion(b);
  if (!pa || !pb) return null;
  for (let i = 0; i < 3; i++) {
    if (pa[i] !== pb[i]) return pa[i] - pb[i];
  }
  return 0;
}

/**
 * A user-facing warning when `version` is a release older than `min`.
 * Unparseable versions (local `dev` builds, bare commit hashes) are assumed
 * current: they come from a source checkout, which is newer by definition.
 * @returns {string|null}
 */
export function versionWarning(version, min = MIN_CLI_VERSION) {
  const cmp = compareVersions(version, min);
  if (cmp === null || cmp >= 0) return null;
  return (
    `The hitspec CLI is v${parseVersion(version).join('.')}, but hitspec Studio needs v${min} or newer ` +
    `(the API server must enforce JSON content types and Host/Origin checks). ` +
    `Upgrade the CLI ("brew upgrade hitspec" or rebuild with "task build") and restart the API server.`
  );
}
