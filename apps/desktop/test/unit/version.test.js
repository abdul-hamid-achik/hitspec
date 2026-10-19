import assert from 'node:assert/strict';
import { test } from 'node:test';

import { MIN_CLI_VERSION, compareVersions, parseVersion, versionWarning } from '../../src/main/version.js';

test('parseVersion handles release, prefixed, describe and prerelease forms', () => {
  assert.deepEqual(parseVersion('2.20.0'), [2, 20, 0]);
  assert.deepEqual(parseVersion('v2.20.1'), [2, 20, 1]);
  assert.deepEqual(parseVersion('v2.20.0-3-gabc1234-dirty'), [2, 20, 0]);
  assert.deepEqual(parseVersion('2.21.0-rc.1'), [2, 21, 0]);
  for (const bad of ['', 'dev', 'v2', 'f92bac1', null, undefined, '2.20']) assert.equal(parseVersion(bad), null, String(bad));
});

test('compareVersions orders numerically, not lexically', () => {
  assert.ok(compareVersions('2.9.0', '2.20.0') < 0);
  assert.ok(compareVersions('2.20.0', '2.20.0') === 0);
  assert.ok(compareVersions('3.0.0', '2.99.99') > 0);
  assert.equal(compareVersions('dev', '2.20.0'), null);
});

test('versionWarning flags releases older than the minimum only', () => {
  assert.equal(MIN_CLI_VERSION, '2.20.0');
  assert.match(versionWarning('2.19.1'), /v2\.19\.1.*v2\.20\.0/s);
  assert.match(versionWarning('v1.0.0'), /needs v2\.20\.0/);
  assert.equal(versionWarning('2.20.0'), null);
  assert.equal(versionWarning('2.21.0'), null);
  assert.equal(versionWarning('dev'), null, 'source builds are assumed current');
  assert.equal(versionWarning(''), null);
  assert.equal(versionWarning('f92bac1'), null);
});
