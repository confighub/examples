const state = {
  config: null,
  workflow: null,
  variants: [],
  selected: null,
  token: null,
  auth: null,
  liveBindings: null,
};

const $ = selector => document.querySelector(selector);

async function loadJson(url, options = {}) {
  const response = await fetch(url, options);
  const data = await response.json();
  if (!response.ok) {
    const message = data.message || data.error || `HTTP ${response.status}`;
    throw new Error(message);
  }
  return data;
}

function headers() {
  const result = {'content-type': 'application/json'};
  if (state.token) {
    result.authorization = `Bearer ${state.token}`;
  }
  return result;
}

function setNote(text) {
  $('#action-note').textContent = text || '';
}

function setAuthState(text) {
  $('#auth-state').textContent = text;
}

function setReadinessCard(id, stateName, title, detail) {
  const card = $(id);
  if (!card) return;
  card.className = `readiness-card ${stateName}`;
  card.querySelector('strong').textContent = title;
  card.querySelector('p').textContent = detail;
}

function renderReadiness() {
  const readiness = bindingReadiness();
  const authMode = state.workflow?.authMode || state.config?.authMode || 'browser-oauth';
  const authReady = authMode !== 'browser-oauth' || Boolean(state.token);
  setReadinessCard(
    '#ready-auth',
    authReady ? 'ok' : 'watch',
    state.token ? 'signed in' : authMode === 'browser-oauth' ? 'sign in needed' : 'fixture auth',
    authMode === 'browser-oauth' ? 'Browser OAuth is required for live ConfigHub reads.' : 'Fixture mode is local review only.',
  );
  setReadinessCard(
    '#ready-inventory',
    state.variants.length ? 'ok' : 'watch',
    state.variants.length ? `${state.variants.length} Variants` : 'loading',
    state.variants.length ? 'Variant rows are ready to inspect.' : 'Waiting for Variant inventory.',
  );
  setReadinessCard(
    '#ready-scope',
    state.selected ? 'ok' : 'watch',
    state.selected ? `${state.selected.variant} / ${state.selected.unit}` : 'not selected',
    state.selected ? state.selected.space : 'Select one Variant before preparing approval.',
  );
  setReadinessCard(
    '#ready-operation',
    readiness.applyReady ? 'ok' : 'watch',
    readiness.label,
    readiness.note,
  );
}

function renderWorkflow() {
  $('#app-title').textContent = state.workflow.app.name;
  $('#job').textContent = state.workflow.scenario.jobToBeDone;
  $('#workflow-steps').innerHTML = state.workflow.workflowSteps
    .map(step => `<li>${escapeHtml(step)}</li>`)
    .join('');
  $('#analysis-cards').innerHTML = (state.workflow.analysisCards || []).map(card => `
    <div class="analysis-card">
      <strong>${escapeHtml(card.title)}</strong>
      <p>${escapeHtml(card.body)}</p>
    </div>
  `).join('');
  $('#binding-requirements').innerHTML = (state.workflow.bindingRequirements || []).map(item => `
    <li>${escapeHtml(item)}</li>
  `).join('');
  renderActionContract();
  renderReadiness();
}

function renderVariants() {
  $('#variant-list').innerHTML = state.variants.map(variant => `
    <button class="variant-row" type="button" data-id="${escapeHtml(variant.id)}">
      <strong>${escapeHtml(variant.app)} / ${escapeHtml(variant.variant)}</strong>
      <span>${escapeHtml(variant.space)} · ${escapeHtml(variant.unit)}</span>
      <span>${escapeHtml(variant.status)} · risk ${escapeHtml(variant.risk)}</span>
    </button>
  `).join('');
  document.querySelectorAll('.variant-row').forEach(button => {
    button.addEventListener('click', () => selectVariant(button.dataset.id));
  });
  if (!state.selected && state.variants[0]) {
    selectVariant(state.variants[0].id);
  }
  renderReadiness();
}

function selectVariant(id) {
  state.selected = state.variants.find(variant => variant.id === id) || state.variants[0];
  if (!state.selected) {
    return;
  }
  $('#variant-title').textContent = `${state.selected.app} / ${state.selected.variant}`;
  $('#variant-status').textContent = state.selected.status;
  const fields = [
    ['Space', state.selected.space],
    ['Unit', state.selected.unit],
    ['Object', state.selected.object],
    ['ConfigHub URL', state.selected.configHubUrl || 'URL not configured yet'],
    ['Next action', state.selected.nextAction],
  ];
  $('#variant-fields').innerHTML = fields.map(([label, value]) => `
    <dt>${escapeHtml(label)}</dt>
    <dd>${escapeHtml(value)}</dd>
  `).join('');
  renderActionContract();
  renderReadiness();
}

function renderProof(receipt) {
  const rows = receipt?.checks || state.workflow.proofTabs;
  $('#proof-tabs').innerHTML = rows.map(row => `
    <div class="proof-card">
      <strong>${escapeHtml(row.label || row.id)}</strong>
      <span>${escapeHtml(row.status || row.layer || 'waiting')}</span>
    </div>
  `).join('');
  if (receipt?.liveBindings) {
    renderBindings(receipt.liveBindings);
  }
}

function isBlockedValue(value) {
  return typeof value === 'string' && value.startsWith('blocked:');
}

function cleanBlockedReason(value) {
  return isBlockedValue(value) ? value.slice('blocked:'.length).replaceAll('-', ' ') : value;
}

function bindingValueReady(value) {
  return Boolean(value) && !isBlockedValue(value);
}

function actionContract() {
  return state.liveBindings?.bindings?.action?.contract || state.workflow?.governedAction || {
    kind: 'ConfigHub-governed-action.v0',
    operation: 'operational-action',
    description: 'Preview scope and hold apply until ConfigHub approval, action, controller, and runtime bindings are real.',
  };
}

function contractSteps(bindings = state.liveBindings) {
  const body = bindings?.bindings || {};
  const contract = actionContract();
  const hasBinding = bindings?.status === 'LIVE_BINDINGS_PRESENT';
  const selectedScope = state.selected ? `${state.selected.space} / ${state.selected.unit}` : '';
  const boundScope = [body.configHub?.space, body.configHub?.unit].filter(Boolean).join(' / ');
  return [
    {
      label: 'User',
      title: state.token ? 'Browser user signed in' : 'Browser user required',
      detail: state.token ? 'ConfigHub browser OAuth session is present.' : 'Sign in before Browser OAuth operations.',
      ready: Boolean(state.token) || state.workflow?.authMode !== 'browser-oauth',
    },
    {
      label: 'Scope',
      title: state.selected ? 'Variant and Unit selected' : 'Select a Variant Unit',
      detail: selectedScope || boundScope || 'Pick a live Variant and Unit before preparing approval.',
      ready: Boolean(state.selected || boundScope),
    },
    {
      label: 'ConfigHub',
      title: hasBinding && bindingValueReady(body.configHub?.objectUrl) ? 'Object binding present' : 'Object binding missing',
      detail: body.configHub?.objectUrl || 'Bind the ConfigHub object URL used by this app.',
      ready: hasBinding && bindingValueReady(body.configHub?.objectUrl),
    },
    {
      label: 'Approval',
      title: hasBinding && bindingValueReady(body.approval?.objectId) ? 'Approval scope bound' : 'Approval scope required',
      detail: body.approval?.objectId || 'Bind a ChangeSet, approval object, or equivalent governed scope.',
      ready: hasBinding && bindingValueReady(body.approval?.objectId),
    },
    {
      label: 'Action',
      title: hasBinding && bindingValueReady(body.action?.endpoint) ? 'Action executor bound' : 'Action executor required',
      detail: body.action?.endpoint || contract.description || 'Bind the approved operation endpoint or invocation.',
      ready: hasBinding && bindingValueReady(body.action?.endpoint),
    },
    {
      label: 'Proof',
      title: hasBinding && bindingValueReady(body.runtime?.evidenceSource) ? 'Runtime proof bound' : 'Controller/runtime proof required',
      detail: body.runtime?.evidenceSource || 'Bind controller delivery and runtime readback before claiming success.',
      ready: hasBinding && bindingValueReady(body.runtime?.evidenceSource),
    },
  ];
}

function renderActionContract(bindings = state.liveBindings) {
  const container = $('#action-contract');
  if (!container || !state.workflow) return;
  const contract = actionContract();
  const readiness = bindingReadiness(bindings);
  $('#contract-state').textContent = `${contract.operation || 'operation'} / ${readiness.label}`;
  container.innerHTML = contractSteps(bindings).map(step => `
    <div class="contract-step ${step.ready ? 'pass' : 'watch'}">
      <span>${escapeHtml(step.ready ? 'bound' : 'needed')} · ${escapeHtml(step.label)}</span>
      <strong>${escapeHtml(step.title)}</strong>
      <p>${escapeHtml(step.detail)}</p>
    </div>
  `).join('');
}

function bindingReadiness(bindings = state.liveBindings) {
  const status = bindings?.status || 'LIVE_BINDINGS_UNKNOWN';
  const body = bindings?.bindings || {};
  if (status !== 'LIVE_BINDINGS_PRESENT') {
    return {
      label: status === 'LIVE_BINDINGS_PLACEHOLDER' ? 'replace placeholders' : 'bindings missing',
      applyReady: false,
      note: status === 'LIVE_BINDINGS_PLACEHOLDER'
        ? 'The live binding file still contains placeholder values. Replace them with verified ConfigHub objects or explicit blocked: reasons.'
        : 'Live bindings are missing. Copy data/live-bindings.example.json to data/live-bindings.json and fill in the real ConfigHub objects before live operation.',
    };
  }
  const blockers = [
    body.configHub?.objectUrl,
    body.approval?.objectId,
    body.action?.contract?.kind === 'ConfigHub-governed-action.v0' ? '' : 'blocked:governed-action-contract-missing',
    body.action?.endpoint,
    body.runtime?.evidenceSource,
  ].filter(isBlockedValue);
  if (blockers.length) {
    return {
      label: 'read-only live surface',
      applyReady: false,
      note: `ConfigHub read proof is connected. Apply remains blocked: ${blockers.map(cleanBlockedReason).join('; ')}.`,
    };
  }
  return {
    label: 'live operation bound',
    applyReady: true,
    note: 'ConfigHub object, approval, action, proof, and runtime bindings are present.',
  };
}

function renderBindings(bindings) {
  state.liveBindings = bindings;
  const status = bindings?.status || 'LIVE_BINDINGS_UNKNOWN';
  const readiness = bindingReadiness(bindings);
  $('#bindings-status').textContent = status === 'LIVE_BINDINGS_PRESENT'
    ? readiness.note
    : readiness.note;
  renderActionContract(bindings);
  renderReadiness();
  refreshButtons();
}

function liveActionAllowed() {
  if (state.workflow.authMode !== 'browser-oauth') {
    return true;
  }
  return Boolean(state.token);
}

function refreshButtons() {
  const allowed = liveActionAllowed();
  const readiness = bindingReadiness();
  $('#preview').disabled = !state.selected || !allowed;
  $('#approve').disabled = !state.selected || !allowed;
  $('#apply').disabled = true;
  $('#apply').title = readiness.applyReady
    ? 'Apply is still disabled in this starter app until a scenario-specific executor is implemented.'
    : readiness.note;
  $('#sign-in').style.display = state.workflow.authMode === 'browser-oauth' ? 'inline-flex' : 'none';
  if (state.workflow.authMode === 'browser-oauth' && !state.token) {
    setNote('Sign in to ConfigHub before preview, approval, or apply.');
  } else if (state.selected) {
    setNote(readiness.note);
  }
  renderReadiness();
}

async function refresh() {
  const [workflow, variants, receipt] = await Promise.all([
    loadJson('/api/workflow'),
    loadJson('/api/variants'),
    loadJson('/api/receipt'),
  ]);
  state.workflow = workflow;
  state.variants = variants.variants;
  renderWorkflow();
  renderVariants();
  renderProof(receipt);
  refreshButtons();
}

async function postAction(path) {
  if (!state.selected) {
    setNote('Select a Variant first.');
    return;
  }
  try {
    const result = await loadJson(path, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({variantId: state.selected.id}),
    });
    setNote(result.message || result.nextGate || result.status);
  } catch (error) {
    setNote(error.message);
  }
}

async function setupAuth() {
  state.config = await loadJson('/app/config');
  if (state.config.authMode !== 'browser-oauth') {
    setAuthState('fixture auth');
    return;
  }
  state.auth = await import('./auth.js');
  const callbackToken = await state.auth.completeSignIn(state.config);
  state.token = callbackToken || state.auth.getAccessToken();
  setAuthState(state.token ? 'signed in' : 'sign-in required');
  renderActionContract();
  renderReadiness();
  $('#sign-in').addEventListener('click', () => state.auth.startSignIn(state.config));
  if (state.token && state.config.configHubBaseUrl) {
    try {
      await state.auth.configHubFetch(state.config, '/api/me');
      setAuthState('ConfigHub connected');
    } catch {
      setAuthState('signed in');
    }
  }
}

function escapeHtml(value) {
  return String(value ?? '').replace(/[&<>"']/g, char => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }[char]));
}

$('#refresh').addEventListener('click', refresh);
$('#proof-refresh').addEventListener('click', async () => renderProof(await loadJson('/api/receipt')));
$('#bindings-refresh').addEventListener('click', async () => renderBindings(await loadJson('/api/bindings')));
$('#preview').addEventListener('click', () => postAction('/api/preview'));
$('#approve').addEventListener('click', () => postAction('/api/approval'));
$('#apply').addEventListener('click', () => postAction('/api/apply'));

const startup = await Promise.allSettled([setupAuth(), refresh()]);
for (const result of startup) {
  if (result.status === 'rejected') {
    setNote(result.reason?.message || String(result.reason));
  }
}
renderReadiness();
