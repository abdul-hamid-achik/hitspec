import { createContractView } from './contract.js';
import { createHistoryView } from './history.js';
import { createImportView } from './importexport.js';
import { createMockView } from './mock.js';
import { createRecordView } from './record.js';
import { createSettingsView } from './settings.js';
import { createStressView } from './stress.js';
import { createWelcomeView } from './welcome.js';
import { createWorkspaceView } from './workspace.js';

/** View registry. Order matches the ⌘1…⌘8 accelerators in the native menu. */
export const VIEWS = [
  { id: 'workspace', label: 'Requests', num: '1', icon: 'requests', create: createWorkspaceView },
  { id: 'history', label: 'History', num: '2', icon: 'history', create: createHistoryView },
  { id: 'stress', label: 'Stress', num: '3', icon: 'stress', create: createStressView },
  { id: 'mock', label: 'Mock', num: '4', icon: 'mock', create: createMockView },
  { id: 'record', label: 'Record', num: '5', icon: 'record', create: createRecordView },
  { id: 'import', label: 'Import', num: '6', icon: 'import', create: createImportView },
  { id: 'contract', label: 'Contracts', num: '7', icon: 'contract', create: createContractView },
  { id: 'settings', label: 'Settings', num: '8', icon: 'settings', create: createSettingsView },
];

export const viewById = (id) => VIEWS.find((view) => view.id === id) ?? null;

export { createWelcomeView, createWorkspaceView };
