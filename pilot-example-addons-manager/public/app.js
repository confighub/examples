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
}

async function loadAppConfig() {
  state.appConfig = await localApi("/app/config");
  $("#auth-base").textContent = state.appConfig.configHubBase || "not set";
  $("#auth-client").textContent = state.appConfig.oauthClientId || "not set";
  renderAuthState();
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

async function loadInventory() {
  resetSelection();
  $("#inventory").innerHTML = `<div class="unit-table empty">Loading inventory...</div>`;
  try {
    const inventory = await readInventory();
    if (inventory.error) throw new Error(inventory.error);
    state.inventory = inventory;
    setModeChip(inventory.source || "fixture", inventory.warning);
    $("#inventory-count").textContent = `${inventory.totals.addons} add-ons / ${inventory.totals.variants} Variants`;
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
  } catch (error) {
    $("#inventory").innerHTML = `<div class="unit-table empty">${escapeHtml(error.message || error)}</div>`;
    setModeChip(state.mode, true);
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
  renderProof();
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
    renderProof();
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
}

function disableButtons() {
  for (const id of ["preview-config", "prepare-scope", "apply", "export-receipt"]) {
    $(`#${id}`).disabled = true;
  }
}

function enableButtons() {
  $("#preview-config").disabled = false;
  $("#prepare-scope").disabled = false;
  $("#export-receipt").disabled = false;
  $("#apply").disabled = true;
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
    body.innerHTML = `<b>${escapeHtml(proof.gate)}</b><p class="muted">Apply stays disabled until approval, write path, controller proof, and runtime proof are connected.</p>`;
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
  await loadAppConfig();
  await completeRedirectIfPresent();
  await loadWorkflow();
  await loadIdentity();
  await loadInventory();
}

boot();
