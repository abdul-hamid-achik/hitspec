import * as actions from '../actions.js';
import { h, clear, appendNodes } from '../dom.js';
import { formatDuration } from '../format.js';
import { btn, card, empty, pill, stat, statGrid, table } from '../ui.js';
import { filePicker, syncInput } from '../widgets.js';

export function createContractView(root) {
  const head = h('div', { class: 'view-head' });
  const body = h('div', { class: 'view-body' });
  clear(root);
  root.append(h('div', { class: 'view' }, head, body));

  const providerInput = h('input', { class: 'input mono', placeholder: 'http://localhost:8080', ariaLabel: 'Provider base URL' });
  const stateHandlerInput = h('input', { class: 'input mono', placeholder: 'optional provider state handler path', ariaLabel: 'State handler' });
  let picker = null;
  let pickerFiles = null;

  const syncForm = () =>
    actions.setContractForm({
      providerUrl: providerInput.value.trim(),
      stateHandler: stateHandlerInput.value.trim(),
      files: picker?.selection() ?? [],
    });

  providerInput.addEventListener('input', syncForm);
  stateHandlerInput.addEventListener('input', syncForm);

  function update(s) {
    const form = s.contract.form;
    syncInput(providerInput, form.providerUrl ?? '');
    syncInput(stateHandlerInput, form.stateHandler ?? '');

    if (!picker || pickerFiles !== s.files) {
      pickerFiles = s.files;
      picker = filePicker({
        files: s.files ?? [],
        selected: form.files ?? [],
        onChange: (files) => actions.setContractForm({ files }),
        height: '150px',
      });
    }

    clear(head);
    head.append(
      h('h2', { text: 'Contract testing' }),
      h('span', { class: 'sub', text: 'Replay provider states against a live implementation' }),
      h('div', { class: 'spacer' }),
      btn('Verify', { variant: 'primary', busy: Boolean(s.busy.contract), onClick: () => { syncForm(); actions.verifyContracts(); } }),
    );

    clear(body);
    if (!s.files?.length) {
      body.append(empty({ glyph: '⚖', title: 'No files to verify', message: 'Open a workspace containing .http files with provider states.' }));
      return;
    }

    appendNodes(
      body,
      card('Provider',
        h('div', { class: 'stack' },
          h('div', { class: 'row wrap' },
            h('div', { class: 'field grow' }, h('label', { text: 'Provider base URL' }), providerInput),
            h('div', { class: 'field grow' }, h('label', { text: 'State handler (optional)' }), stateHandlerInput)),
          picker.el,
          h('div', { class: 'muted', style: { fontSize: '11.5px' }, text: 'Leaving the file list empty verifies every .http file in the workspace.' })),
        { bodyClass: 'card-body' }),

      s.contract.results
        ? card('Results',
            s.contract.results.length
              ? h('div', { class: 'stack' }, s.contract.results.map((result) => resultCard(result)))
              : h('div', { class: 'muted', text: 'Verification produced no results.' }),
            { bodyClass: 'card-body' })
        : null,
    );
  }

  return { update };
}

function resultCard(result) {
  const interactions = result.results ?? [];
  return h('div', { class: 'card' },
    h('div', { class: 'card-head' },
      h('h3', { class: 'mono', text: result.file }),
      h('div', { class: 'spacer' }),
      pill(`${result.passed} passed`, 'success'),
      result.failed ? pill(`${result.failed} failed`, 'danger') : null,
      result.skipped ? pill(`${result.skipped} skipped`) : null,
      pill(formatDuration(result.duration))),
    statGrid([
      stat('Interactions', String(interactions.length)),
      stat('Passed', String(result.passed), 'success'),
      stat('Failed', String(result.failed), result.failed ? 'danger' : ''),
    ]),
    interactions.length
      ? table({
          columns: [
            {
              label: '',
              width: '34px',
              render: (row) => h('span', { class: row.passed ? 'success' : 'danger', text: row.passed ? '✓' : '✕' }),
            },
            { label: 'Interaction', class: 'ell', render: (row) => row.name },
            { label: 'Provider', class: 'mono ell', render: (row) => row.provider || '—' },
            { label: 'State', class: 'mono ell', render: (row) => row.state || '—' },
            { label: 'Duration', class: 'num', width: '92px', render: (row) => formatDuration(row.duration) },
            { label: 'Error', class: 'ell', render: (row) => (row.error ? h('span', { class: 'danger mono', text: row.error }) : h('span', { class: 'muted', text: '—' })) },
          ],
          rows: interactions,
        })
      : null);
}
