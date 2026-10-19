/**
 * Pure decision helpers for protecting unsaved editor changes. Kept free of
 * DOM/state imports so they can be unit tested.
 */

/**
 * What `openFile` should do about an unsaved buffer.
 *  - 'proceed': nothing to protect
 *  - 'skip':    an automatic reload of the file being edited; never clobber it
 *  - 'confirm': a different file was requested; ask before discarding
 */
export function openFileDecision({ dirty, currentPath, targetPath, keepSelection = false, force = false }) {
  if (!dirty || force) return 'proceed';
  if (targetPath === currentPath) return keepSelection ? 'skip' : 'proceed';
  return 'confirm';
}

/**
 * Has the file changed on disk since this buffer last synced with it?
 * `known` is the disk content seen at open/save time, `current` a fresh read.
 */
export function diskChanged(known, current) {
  return typeof known === 'string' && typeof current === 'string' && known !== current;
}
