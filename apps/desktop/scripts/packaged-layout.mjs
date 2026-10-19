import fs from 'node:fs';
import path from 'node:path';

/**
 * Where electron-builder `--dir` leaves each unpacked app and its bundled CLI.
 * `arch` is the CPU the directory was built for (electron-builder names the
 * mac x64 directory plain "mac").
 */
export function packagedApps(releaseDir) {
  const found = [];
  const add = (dir, platform, arch, exe, resources, fuseHost = exe) => {
    const root = path.join(releaseDir, dir);
    if (!fs.existsSync(root)) return;
    found.push({
      dir,
      platform,
      arch,
      executable: path.join(root, ...exe),
      // The fuse wire is embedded in the Electron Framework binary on macOS and
      // in the executable itself elsewhere.
      fuseHost: path.join(root, ...fuseHost),
      cli: path.join(root, ...resources, platform === 'win32' ? 'hitspec.exe' : 'hitspec'),
    });
  };
  const macApp = 'hitspec Studio.app';
  const macFramework = [macApp, 'Contents', 'Frameworks', 'Electron Framework.framework', 'Versions', 'A', 'Electron Framework'];
  add('mac-arm64', 'darwin', 'arm64', [macApp, 'Contents', 'MacOS', 'hitspec Studio'], [macApp, 'Contents', 'Resources'], macFramework);
  add('mac', 'darwin', 'x64', [macApp, 'Contents', 'MacOS', 'hitspec Studio'], [macApp, 'Contents', 'Resources'], macFramework);
  add('win-unpacked', 'win32', 'x64', ['hitspec Studio.exe'], ['resources']);
  add('linux-unpacked', 'linux', 'x64', ['hitspec-studio'], ['resources']);
  return found;
}
