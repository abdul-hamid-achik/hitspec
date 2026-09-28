import * as actions from '../actions.js';
import { h, clear } from '../dom.js';
import { highlightHttp } from '../highlight.js';
import { btn, card, pill, tabs } from '../ui.js';
import { syncInput } from '../widgets.js';

const KINDS = [
  { id: 'curl', label: 'curl' },
  { id: 'openapi', label: 'OpenAPI' },
  { id: 'insomnia', label: 'Insomnia' },
];

const HINTS = {
  curl: 'Paste one or more curl commands. Multi-line commands with backslash continuations are handled, and each command becomes a separate request.',
  openapi: 'Path to an OpenAPI/Swagger spec inside the workspace, or a public https:// URL. Localhost and private IP addresses are rejected by the server.',
  insomnia: 'Paste an Insomnia v4 collection export (JSON or YAML). Folders become request names and the body is preserved.',
};

export function createImportView(root) {
  const head = h('div', { class: 'view-head' });
  const body = h('div', { class: 'view-body' });
  clear(root);
  root.append(h('div', { class: 'view' }, head, body));

  const sourceArea = h('textarea', {
    class: 'textarea mono',
    rows: 10,
    placeholder: "curl -X POST 'https://api.example.com/login' \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"user\":\"ada\"}'",
    ariaLabel: 'Import source',
  });
  const specInput = h('input', { class: 'input mono', placeholder: 'specs/openapi.yaml or https://…/openapi.json', ariaLabel: 'OpenAPI spec path' });
  const baseUrlInput = h('input', { class: 'input mono', placeholder: '{{baseUrl}}', ariaLabel: 'Base URL override' });
  const preview = h('pre', { class: 'code', style: { maxHeight: '46vh' } });

  sourceArea.addEventListener('input', () => actions.setImportDraft({ input: sourceArea.value }));
  specInput.addEventListener('input', () => actions.setImportDraft({ specPath: specInput.value }));
  baseUrlInput.addEventListener('input', () => actions.setImportDraft({ baseUrl: baseUrlInput.value }));

  function update(s) {
    const draft = s.importDraft;
    syncInput(sourceArea, draft.input ?? '');
    syncInput(specInput, draft.specPath ?? '');
    syncInput(baseUrlInput, draft.baseUrl ?? '');

    clear(head);
    head.append(
      h('h2', { text: 'Import' }),
      h('span', { class: 'sub', text: 'Turn existing definitions into runnable .http files' }),
      h('div', { class: 'spacer' }),
      btn('Import', {
        variant: 'primary',
        busy: Boolean(s.busy.import),
        onClick: () => actions.runImport(),
      }),
    );

    clear(body);
    body.append(
      h('div', { class: 'stack' },
        card('Source',
          h('div', { class: 'stack' },
            tabs(KINDS, draft.kind, (id) => actions.setImportDraft({ kind: id, result: null })),
            h('div', { style: { padding: '12px' } },
              h('p', { class: 'muted', style: { margin: '0 0 10px', fontSize: '11.5px', lineHeight: 1.6 }, text: HINTS[draft.kind] }),
              draft.kind === 'openapi'
                ? h('div', { class: 'stack' },
                    h('div', { class: 'field' }, h('label', { text: 'Spec path or URL' }), specInput),
                    h('div', { class: 'field' }, h('label', { text: 'Base URL (optional)' }), baseUrlInput,
                      h('span', { class: 'muted', style: { fontSize: '10.5px' }, text: 'Used for {{baseUrl}} in the generated file.' })))
                : h('div', { class: 'field' },
                    h('label', { text: draft.kind === 'curl' ? 'curl command(s)' : 'Insomnia export' }),
                    sourceArea))),
          { bodyClass: '' }),

        draft.result
          ? card('Preview',
              h('div', { class: 'stack' },
                h('div', { class: 'row wrap' },
                  pill(`${draft.result.requestCount ?? 0} request(s)`, 'accent'),
                  h('span', { class: 'muted', text: 'Nothing is written until you save it into the workspace.' })),
                preview),
              {
                actions: [
                  btn('Copy', { size: 'sm', onClick: () => actions.copyText(draft.result.content ?? '', 'Imported file copied') }),
                  btn('Save to workspace…', { size: 'sm', variant: 'primary', onClick: () => actions.saveImport() }),
                ],
              })
          : null),
    );

    const content = draft.result?.content ?? '';
    preview.innerHTML = content ? highlightHttp(content) : '';
    if (!content) {
      preview.textContent = 'Run an import to preview the generated .http file.';
      preview.style.opacity = '0.6';
    } else {
      preview.style.opacity = '1';
    }
  }

  return { update };
}
