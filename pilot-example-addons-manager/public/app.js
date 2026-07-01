import { completeLoginFromRedirect, startLogin } from "./auth.js";
import { createConfigHubBrowserClient } from "./confighub-api.js";

const state = {
  mode: "fixture",
  appConfig: null,
  session: null,
  browserClient: null,
  inventory: null,
  selection: null,
  detail: null,
  receipt: null,
  bindings: null,
  proofTab: "Revision",
  events: [],
};

const $ = (selector) => document.querySelector(selector);

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

async function localApi(path) {
  const response = await fetch(path);
  const payload = await response.json();
  if (!response.ok && !payload.error) payload.error = response.statusText;
  return payload;
}

function logEvent(text) {
  state.events.unshift({time: new Date().toLocaleTimeString(), text});
  state.events = state.events.slice(0, 12);
  $("#event-count").textContent = String(state.events.length);
  $("#event-log").innerHTML = state.events
    .map((event) => `<li><b>${escapeHtml(event.time)}</b> ${escapeHtml(event.text)}</li>`)
    .join("");
}

function setModeChip(source, warning) {
  const mode = $("#mode");
  mode.textContent = warning ? `${source} with fallback` : source;
  mode.className = `chip ${warning ? "warn" : "ok"}`;
}

function setRunwayCard(id, stateName, title, detail) {
  const card = $(`#${id}`);
  if (!card) return;
  card.className = `runway-card ${stateName}`;
  card.querySelector("b").textContent = title;
  card.querySelector("p").textContent = detail;
}

function renderRunway() {
  const readiness = bindingReadiness();
  const inventoryTotals = state.inventory?.totals;
  const authReady = Boolean(state.session) || state.mode === "fixture";
  const authTitle = state.session
    ? (state.session.displayName || state.session.username || "signed in")
    : state.mode === "browser"
      ? "sign in needed"
      : "fixture operator";
  setRunwayCard(
    "runway-auth",
    authReady ? "ok" : "warn",
    authTitle,
    state.mode === "browser" ? "Browser OAuth reads ConfigHub directly." : "Fixture mode keeps the app runnable offline.",
  );
  setRunwayCard(
    "runway-inventory",
    inventoryTotals?.variants ? "ok" : "warn",
    inventoryTotals?.variants ? `${inventoryTotals.variants} Variants` : "loading",
    inventoryTotals?.addons ? `${inventoryTotals.addons} add-ons / ${inventoryTotals.units} Units` : "Waiting for inventory.",
  );
  setRunwayCard(
    "runway-scope",
    state.detail?.unit ? "ok" : "warn",
    state.detail?.unit?.slug || state.selection?.space || "not selected",
    state.detail?.unit ? `${state.selection?.addon} / ${state.selection?.variant}` : "Choose a Variant and Unit.",
  );
  setRunwayCard(
    "runway-operation",
    readiness.applyReady ? "ok" : "warn",
    readiness.label,
    readiness.note,
  );
}

function isBlockedValue(value) {
  return typeof value === "string" && value.startsWith("blocked:");
}

function cleanBlockedReason(value) {
  return isBlockedValue(value) ? value.slice("blocked:".length).replaceAll("-", " ") : value;
}

function bindingValueReady(value) {
  return Boolean(value) && !isBlockedValue(value);
}

function bindingReadiness() {
  const status = state.bindings?.status || "LIVE_BINDINGS_UNKNOWN";
  const bindings = state.bindings?.bindings || {};
  if (status !== "LIVE_BINDINGS_PRESENT") {
    return {
      label: status === "LIVE_BINDINGS_PLACEHOLDER" ? "replace placeholders" : "bindings missing",
      chip: "warn",
      applyReady: false,
      note: state.bindings?.reason || "Create deployment-local live bindings before live operation.",
    };
  }
  const blockers = [
    bindings.configHub?.objectUrl,
    bindings.approval?.objectId,
    bindings.action?.endpoint,
    bindings.action?.contract?.kind === "ConfigHub-governed-action.v0" ? "" : "blocked:action-contract-missing",
    bindings.runtime?.evidenceSource,
  ].filter(isBlockedValue);
  if (blockers.length) {
    return {
      label: "read-only live surface",
      chip: "warn",
      applyReady: false,
      note: `ConfigHub read proof is connected. Apply remains blocked: ${blockers.map(cleanBlockedReason).join("; ")}.`,
    };
  }
  return {
    label: "live operation bound",
    chip: "ok",
    applyReady: true,
    note: "ConfigHub object, approval, action, proof, and runtime bindings are present.",
  };
}

function resetSelection() {
  state.inventory = null;
  state.selection = null;
  state.detail = null;
  state.receipt = null;
  $("#selection-title").textContent = "Select an add-on Variant";
  $("#selection-subtitle").textContent = "Inventory is grouped by add-on, Variant, space, and Unit.";
  $("#units").innerHTML = `<div class="empty">No Variant selected.</div>`;
  $("#unit-count").textContent = "0";
  renderScope();
  renderDetail();
  renderProof();
  renderRunway();
}

async function loadAppConfig() {
  state.appConfig = await localApi("/app/config");
  $("#auth-base").textContent = state.appConfig.configHubBase || "not set";
  $("#auth-client").textContent = state.appConfig.oauthClientId || "not set";
  renderAuthState();
}

async function loadBindings() {
  state.bindings = await localApi("/app/bindings");
  renderBindings();
}

function renderBindings() {
  const status = state.bindings?.status || "LIVE_BINDINGS_UNKNOWN";
  const readiness = bindingReadiness();
  const bindings = state.bindings?.bindings || {};
  $("#binding-state").textContent = readiness.label;
  $("#binding-state").className = `chip ${readiness.chip}`;
  $("#binding-file").textContent = status === "LIVE_BINDINGS_PRESENT" ? "deployment-local binding loaded" : "data/live-bindings.json";
  const rows = status === "LIVE_BINDINGS_PRESENT"
    ? [
        ["Readiness", readiness.note],
        ["ConfigHub object", bindings.configHub?.objectUrl],
        ["Org / space", [bindings.configHub?.externalOrganizationId || bindings.configHub?.organizationId, bindings.configHub?.space || bindings.configHub?.spaceId].filter(Boolean).join(" / ")],
        ["Approval", bindings.approval?.objectId],
        ["Action", bindings.action?.endpoint],
        ["Proof", bindings.proof?.receiptObjectId],
        ["Runtime", bindings.runtime?.evidenceSource],
        ["Runtime readback", bindings.runtime?.readback],
      ]
    : [
        ["Status", status],
        ["Required file", state.bindings?.requiredFile],
        ["Example file", state.bindings?.exampleFile],
        ["Reason", state.bindings?.reason],
      ];
  $("#binding-grid").innerHTML = rows
    .map(([label, value]) => `<div><span>${escapeHtml(label)}</span><b>${escapeHtml(value || "-")}</b></div>`)
    .join("");
  renderActionContract();
  renderRunway();
  refreshActionState();
}

function actionContract() {
  const bindings = state.bindings?.bindings || {};
  return bindings.action?.contract || {
    kind: "ConfigHub-governed-action.v0",
    operation: "review-addon-version",
    description: "Preview add-on scope and hold Apply until ConfigHub approval, action, controller, and runtime bindings are real.",
  };
}

function contractSteps() {
  const bindings = state.bindings?.bindings || {};
  const contract = actionContract();
  const hasBinding = state.bindings?.status === "LIVE_BINDINGS_PRESENT";
  const hasObject = hasBinding && bindingValueReady(bindings.configHub?.objectUrl);
  const hasApproval = hasBinding && bindingValueReady(bindings.approval?.objectId);
  const hasAction = hasBinding && bindingValueReady(bindings.action?.endpoint);
  const hasRuntime = hasBinding && bindingValueReady(bindings.runtime?.evidenceSource);
  const hasUser = Boolean(state.session);
  const selectedScope = [state.selection?.space, state.selection?.unit].filter(Boolean).join(" / ");
  const boundScope = [bindings.configHub?.space, bindings.configHub?.unit].filter(Boolean).join(" / ");
  return [
    {
      label: "User",
      title: hasUser ? "Browser user signed in" : "Browser user required",
      detail: hasUser ? (state.session?.displayName || state.session?.username || "authenticated") : "Sign in before Browser OAuth operations.",
      ready: hasUser,
    },
    {
      label: "Scope",
      title: state.detail?.unit ? "Variant and Unit selected" : "Select a Variant Unit",
      detail: selectedScope || boundScope || "Pick a live add-on Variant and Unit before preparing approval.",
      ready: Boolean(state.detail?.unit || boundScope),
    },
    {
      label: "ConfigHub",
      title: hasObject ? "Object binding present" : "Object binding missing",
      detail: bindings.configHub?.objectUrl || "Bind the ConfigHub object URL used by this app.",
      ready: hasObject,
    },
    {
      label: "Approval",
      title: hasApproval ? "Approval scope bound" : "Approval scope required",
      detail: bindings.approval?.objectId || "Bind a ChangeSet, approval object, or equivalent governed scope.",
      ready: hasApproval,
    },
    {
      label: "Action",
      title: hasAction ? "Action executor bound" : "Action executor required",
      detail: bindings.action?.endpoint || contract.description || "Bind the approved operation endpoint or invocation.",
      ready: hasAction,
    },
    {
      label: "Proof",
      title: hasRuntime ? "Runtime proof bound" : "Controller/runtime proof required",
      detail: bindings.runtime?.evidenceSource || "Bind controller delivery and runtime readback before claiming success.",
      ready: hasRuntime,
    },
  ];
}

function renderActionContract() {
  const container = $("#action-contract");
  if (!container) return;
  const contract = actionContract();
  const readiness = bindingReadiness();
  const contractState = $("#contract-state");
  if (contractState) {
    contractState.textContent = `${contract.operation || "operation"} / ${readiness.label}`;
  }
  container.innerHTML = contractSteps().map((step) => `
    <div class="contract-step ${step.ready ? "pass" : "watch"}">
      <span>${escapeHtml(step.ready ? "bound" : "needed")} · ${escapeHtml(step.label)}</span>
      <b>${escapeHtml(step.title)}</b>
      <p>${escapeHtml(step.detail)}</p>
    </div>
  `).join("");
}

function renderAuthState() {
  const configured = Boolean(state.appConfig?.browserAuthConfigured);
  const signedIn = Boolean(state.session);
  $("#login").disabled = !configured || signedIn;
  $("#call-me").disabled = !signedIn;
  $("#mode-select").querySelector("option[value=browser]").disabled = !configured;
  if (!configured) {
    $("#auth-state").textContent = "Browser OAuth not configured";
    $("#auth-state").className = "chip warn";
  } else if (signedIn) {
    $("#auth-state").textContent = `Signed in / org ${state.session.organizationId || "unknown"}`;
    $("#auth-state").className = "chip ok";
  } else {
    $("#auth-state").textContent = "Ready to sign in";
    $("#auth-state").className = "chip";
  }
  renderActionContract();
  renderRunway();
  refreshActionState();
}

async function completeRedirectIfPresent() {
  try {
    const session = await completeLoginFromRedirect();
    if (session) {
      state.session = session;
      state.browserClient = createConfigHubBrowserClient(session);
      state.mode = "browser";
      $("#mode-select").value = "browser";
      logEvent("Completed browser OAuth sign-in.");
    }
  } catch (error) {
    logEvent(`Sign-in failed: ${error.message || error}`);
  }
  renderAuthState();
}

async function login() {
  try {
    await startLogin({
      baseUrl: state.appConfig.configHubBase,
      clientId: state.appConfig.oauthClientId,
    });
  } catch (error) {
    logEvent(`Sign-in failed: ${error.message || error}`);
  }
}

async function callMe() {
  if (!state.browserClient) return;
  try {
    const payload = await state.browserClient.me();
    $("#identity").textContent = payload.DisplayName || payload.Username || payload.user || "authenticated browser user";
    $("#identity").className = "chip ok";
    logEvent("Browser token called ConfigHub /api/me successfully.");
  } catch (error) {
    $("#identity").textContent = "/api/me failed";
    $("#identity").className = "chip bad";
    logEvent(`/api/me failed: ${error.message || error}`);
  }
}

async function loadIdentity() {
  const element = $("#identity");
  if (state.mode === "browser") {
    if (!state.browserClient) {
      element.textContent = "sign in for browser mode";
      element.className = "chip warn";
      return;
    }
    await callMe();
    return;
  }

  const identity = await localApi("/api/me");
  if (identity.error) {
    element.textContent = identity.error;
    element.className = "chip warn";
    return;
  }
  const body = identity.body || identity;
  element.textContent = body.DisplayName || body.Username || body.user || "sample user";
  element.className = "chip ok";
  renderRunway();
}

async function loadWorkflow() {
  const workflow = await localApi("/api/workflow");
  $("#workflow-summary").textContent = workflow.summary || "";
  $("#workflow-steps").innerHTML = (workflow.steps || [])
    .map((step, index) => `
      <li>
        <span>Step ${index + 1}</span>
        <b>${escapeHtml(step.label)}</b>
        <p>${escapeHtml(step.purpose)}</p>
      </li>
    `)
    .join("");
  $("#proof-tabs").innerHTML = (workflow.proofTabs || [])
    .map((name) => `<button class="tab ${name === state.proofTab ? "active" : ""}" data-tab="${escapeHtml(name)}">${escapeHtml(name)}</button>`)
    .join("");
  document.querySelectorAll(".tab").forEach((tab) => {
    tab.addEventListener("click", () => {
      state.proofTab = tab.dataset.tab;
      renderProof();
    });
  });
}

async function readInventory() {
  if (state.mode === "browser") {
    if (!state.browserClient) throw new Error("Sign in before loading browser OAuth inventory.");
    return state.browserClient.inventory();
  }
  return localApi("/api/inventory");
}

function emptyInventoryMessage() {
  if (state.mode === "browser") {
    return `
      <div class="empty-state">
        <b>No add-on Variants found in this ConfigHub org yet.</b>
        <p>The browser sign-in and ConfigHub read path can be working even when the org has no spaces named like <code>helm-&lt;addon&gt;-&lt;variant&gt;</code> and no add-on Units yet.</p>
        <p>Next proof: create or import add-on Units, then refresh Browser OAuth inventory.</p>
      </div>
    `;
  }
  return `<div class="empty-state"><b>No add-on Variants found.</b><p>Use fixture mode for bundled sample data, or connect a ConfigHub org that has add-on spaces and Units.</p></div>`;
}

async function loadInventory() {
  resetSelection();
  $("#inventory").innerHTML = `<div class="unit-table empty">Loading inventory...</div>`;
  try {
    const inventory = await readInventory();
    if (inventory.error) throw new Error(inventory.error);
    state.inventory = inventory;
    setModeChip(inventory.source || "fixture", inventory.warning);
    $("#inventory-count").textContent = `${inventory.totals.addons} add-ons / ${inventory.totals.variants} Variants`;
    if (!(inventory.addons || []).length) {
      $("#inventory").innerHTML = emptyInventoryMessage();
      logEvent(`Loaded 0 Variants from ${inventory.source || "fixture"} data.`);
      renderRunway();
      return;
    }
    $("#inventory").innerHTML = (inventory.addons || [])
      .map((group) => `
        <section>
          <div class="addon-heading">${escapeHtml(group.addon)} <span class="muted">${group.variants.length} Variant${group.variants.length === 1 ? "" : "s"}</span></div>
          ${group.variants.map((variant) => `
            <button class="variant-row" data-addon="${escapeHtml(group.addon)}" data-space="${escapeHtml(variant.slug)}">
              <span class="row-title">${escapeHtml(variant.variant)}</span>
              <span class="row-meta">${escapeHtml(variant.slug)} / ${variant.unitCount} Units</span>
            </button>
          `).join("")}
        </section>
      `)
      .join("");
    document.querySelectorAll(".variant-row").forEach((row) => row.addEventListener("click", () => selectVariant(row)));
    logEvent(`Loaded ${inventory.totals.variants} Variants from ${inventory.source || "fixture"} data.`);
    renderRunway();
  } catch (error) {
    $("#inventory").innerHTML = `<div class="unit-table empty">${escapeHtml(error.message || error)}</div>`;
    setModeChip(state.mode, true);
    renderRunway();
  }
}

function findVariant(space) {
  for (const group of state.inventory?.addons || []) {
    const variant = group.variants.find((item) => item.slug === space);
    if (variant) return {group, variant};
  }
  return null;
}

function renderScope(scope = {}) {
  const rows = [
    ["Add-on", scope.addon],
    ["Variant", scope.variant],
    ["Space", scope.space],
    ["Unit", scope.unit],
  ];
  $("#scope-grid").innerHTML = rows
    .map(([label, value]) => `<div class="scope-item"><span>${label}</span><b>${escapeHtml(value || "-")}</b></div>`)
    .join("");
}

function selectVariant(row) {
  document.querySelectorAll(".variant-row").forEach((item) => item.classList.remove("selected"));
  row.classList.add("selected");
  const found = findVariant(row.dataset.space);
  state.selection = {
    addon: row.dataset.addon,
    space: row.dataset.space,
    variant: found?.variant.variant || "default",
    spaceRecord: found?.variant || null,
    units: found?.variant.units || [],
  };
  state.detail = null;
  state.receipt = null;
  $("#selection-title").textContent = `${state.selection.addon} / ${state.selection.variant}`;
  $("#selection-subtitle").textContent = state.selection.space;
  renderScope(state.selection);
  renderUnits();
  renderDetail();
  renderActionContract();
  renderProof();
  renderRunway();
  logEvent(`Selected Variant ${state.selection.variant} for ${state.selection.addon}.`);
}

function renderUnits() {
  const units = state.selection?.units || [];
  $("#unit-count").textContent = String(units.length);
  if (!units.length) {
    $("#units").innerHTML = `<div class="empty">No Units returned for this Variant.</div>`;
    return;
  }
  $("#units").innerHTML = `
    <table>
      <thead><tr><th>Unit</th><th>Component</th><th>Version</th><th>Head</th></tr></thead>
      <tbody>
        ${units.map((unit) => `
          <tr>
            <td><button class="unit-row" data-unit="${escapeHtml(unit.slug)}"><span class="row-title">${escapeHtml(unit.slug)}</span><span class="row-meta">${escapeHtml(unit.variant || state.selection.variant)}</span></button></td>
            <td>${escapeHtml(unit.component || "-")}</td>
            <td>${escapeHtml(unit.chartVersion || "-")}</td>
            <td>${escapeHtml(unit.headRevision || "-")}</td>
          </tr>
        `).join("")}
      </tbody>
    </table>
  `;
  document.querySelectorAll(".unit-row").forEach((row) => row.addEventListener("click", () => selectUnit(row)));
}

async function readDetail(unit) {
  if (state.mode === "browser") {
    return state.browserClient.detail(state.selection.spaceRecord, unit);
  }
  return localApi(`/api/detail?space=${encodeURIComponent(state.selection.space)}&unit=${encodeURIComponent(unit.slug)}`);
}

async function readReceipt(detail) {
  if (state.mode === "browser") {
    return state.browserClient.receipt(detail);
  }
  return localApi(`/api/receipt?space=${encodeURIComponent(state.selection.space)}&unit=${encodeURIComponent(detail.unit.slug)}`);
}

async function selectUnit(row) {
  document.querySelectorAll(".unit-row").forEach((item) => item.classList.remove("selected"));
  row.classList.add("selected");
  const unit = state.selection.units.find((item) => item.slug === row.dataset.unit);
  state.selection.unit = unit.slug;
  state.selection.unitRecord = unit;
  renderScope(state.selection);
  try {
    state.detail = await readDetail(unit);
    state.receipt = await readReceipt(state.detail);
    $("#unit-source").textContent = state.detail.source || state.mode;
    renderDetail();
    enableButtons();
    renderActionContract();
    renderProof();
    renderRunway();
    logEvent(`Previewed Unit ${unit.slug}.`);
  } catch (error) {
    $("#detail").innerHTML = `<div class="empty">${escapeHtml(error.message || error)}</div>`;
    logEvent(`Unit preview failed: ${error.message || error}`);
  }
}

function renderDetail() {
  if (!state.detail?.unit) {
    $("#detail").innerHTML = `<div class="empty">No Unit selected.</div>`;
    $("#unit-source").textContent = "-";
    disableButtons();
    renderActionContract();
    renderRunway();
    return;
  }
  const detail = state.detail;
  const unit = detail.unit;
  const parsed = detail.parsedData || {};
  $("#detail").innerHTML = `
    <div class="detail-grid">
      <div>Unit</div><div><code>${escapeHtml(unit.slug)}</code></div>
      <div>Variant</div><div>${escapeHtml(unit.variant || state.selection.variant)}</div>
      <div>Head / applied / live</div><div>${escapeHtml(unit.headRevision || "?")} / ${escapeHtml(unit.appliedRevision || "?")} / ${escapeHtml(unit.liveRevision || "?")}</div>
      <div>Chart</div><div>${escapeHtml(parsed.chart || "-")}</div>
      <div>App version</div><div>${escapeHtml(parsed.appVersion || "-")}</div>
      <div>Image</div><div>${parsed.image ? `<code>${escapeHtml(parsed.image)}</code>` : "-"}</div>
      <div>Replicas</div><div>${escapeHtml(parsed.replicas || "-")}</div>
    </div>
    ${(detail.warnings || []).map((warning) => `<div class="warning">${escapeHtml(warning)}</div>`).join("")}
  `;
  renderActionContract();
  renderRunway();
}

function disableButtons() {
  for (const id of ["preview-config", "prepare-scope", "apply", "export-receipt"]) {
    $(`#${id}`).disabled = true;
  }
  refreshActionState();
}

function enableButtons() {
  $("#preview-config").disabled = false;
  $("#prepare-scope").disabled = false;
  $("#export-receipt").disabled = false;
  refreshActionState();
}

function refreshActionState() {
  const readiness = bindingReadiness();
  const hasUnit = Boolean(state.detail?.unit);
  const actionNote = $("#action-note");
  if (actionNote) {
    actionNote.textContent = hasUnit
      ? readiness.note
      : "Select a Unit to preview config, prepare approval scope, and inspect proof.";
  }
  const apply = $("#apply");
  if (apply) {
    apply.disabled = true;
    apply.title = readiness.applyReady
      ? "Apply is still disabled in this sample until a scenario-specific executor is implemented."
      : readiness.note;
  }
  const proofState = $("#proof-state");
  if (proofState) proofState.textContent = readiness.label;
  renderRunway();
}

function renderProof() {
  document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("active", tab.dataset.tab === state.proofTab));
  const body = $("#proof-body");
  const receipt = state.receipt;
  if (!receipt) {
    body.textContent = "Select a Unit to see proof.";
    return;
  }
  const proof = receipt.proof || {};
  const scope = receipt.scope || {};
  const tab = state.proofTab;
  if (tab === "Revision") {
    body.innerHTML = `<b>${escapeHtml(proof.revision)}</b><pre>${escapeHtml(JSON.stringify(state.detail.revisions || [], null, 2))}</pre>`;
  } else if (tab === "Approval") {
    body.innerHTML = `<b>${escapeHtml(proof.approval)}</b><pre>${escapeHtml(JSON.stringify(scope, null, 2))}</pre>`;
  } else if (tab === "Gate") {
    const readiness = bindingReadiness();
    body.innerHTML = `<b>${escapeHtml(proof.gate)}</b><p class="muted">${escapeHtml(readiness.note)}</p><pre>${escapeHtml(JSON.stringify({contract: actionContract(), bindings: state.bindings || {}}, null, 2))}</pre>`;
  } else if (tab === "Controller") {
    body.innerHTML = `<b>${escapeHtml(proof.controller)}</b><p class="muted">No Argo, Flux, or other controller readback is wired in this standalone sample.</p>`;
  } else if (tab === "Runtime") {
    body.innerHTML = `<b>${escapeHtml(proof.runtime)}</b><p class="muted">No cluster runtime readback is wired in this standalone sample.</p>`;
  } else {
    body.innerHTML = `<b>${escapeHtml(proof.receipt)}</b><pre>${escapeHtml(JSON.stringify(receipt, null, 2))}</pre>`;
  }
}

function previewConfig() {
  if (!state.detail) return;
  $("#proof-body").innerHTML = `<pre>${escapeHtml(state.detail.unitData || "No Unit data returned.")}</pre>`;
  logEvent("Opened current Unit data preview.");
}

async function prepareScope() {
  if (!state.detail) return;
  const proposal = state.mode === "browser"
    ? state.browserClient.proposal(state.detail)
    : await localApi(`/api/proposal?space=${encodeURIComponent(state.selection.space)}&unit=${encodeURIComponent(state.selection.unit)}`);
  state.proofTab = "Approval";
  renderProof();
  $("#proof-body").innerHTML = `<pre>${escapeHtml(JSON.stringify(proposal.approvalScope, null, 2))}</pre>`;
  logEvent("Prepared approval scope preview.");
}

function exportReceipt() {
  if (!state.receipt) return;
  const blob = new Blob([JSON.stringify(state.receipt, null, 2) + "\n"], {type: "application/json"});
  const link = document.createElement("a");
  link.href = URL.createObjectURL(blob);
  link.download = `${state.selection.addon}-${state.selection.variant}-${state.selection.unit}-receipt.json`;
  link.click();
  URL.revokeObjectURL(link.href);
  logEvent("Exported local receipt preview.");
}

function wireControls() {
  $("#refresh").addEventListener("click", loadInventory);
  $("#mode-select").addEventListener("change", async (event) => {
    state.mode = event.target.value;
    if (state.mode === "browser" && !state.browserClient) {
      logEvent("Sign in before loading browser OAuth inventory.");
    }
    await loadIdentity();
    await loadInventory();
  });
  $("#login").addEventListener("click", login);
  $("#call-me").addEventListener("click", callMe);
  $("#preview-config").addEventListener("click", previewConfig);
  $("#prepare-scope").addEventListener("click", prepareScope);
  $("#export-receipt").addEventListener("click", exportReceipt);
  $("#apply").addEventListener("click", () => logEvent("Apply is blocked in this sample."));
}

async function boot() {
  wireControls();
  disableButtons();
  renderScope();
  renderRunway();
  const startup = await Promise.allSettled([
    loadAppConfig(),
    loadBindings(),
    loadWorkflow(),
  ]);
  for (const result of startup) {
    if (result.status === "rejected") logEvent(`Startup check failed: ${result.reason?.message || result.reason}`);
  }
  await completeRedirectIfPresent();
  const liveReads = await Promise.allSettled([loadIdentity(), loadInventory()]);
  for (const result of liveReads) {
    if (result.status === "rejected") logEvent(`Live read failed: ${result.reason?.message || result.reason}`);
  }
  renderRunway();
}

boot();
