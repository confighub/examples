import fs from "node:fs/promises";
import http from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { jsonResponse, PUBLIC_URL, runtimeConfig, textResponse } from "./config.mjs";
import {
  fixtureDetail,
  fixtureInventory,
  fixtureMe,
  fixtureReceipt,
  fixtureRevisions,
  fixtureSpaces,
  fixtureUnit,
  fixtureUnitData,
  fixtureUnits,
} from "./fixtures.mjs";
import {
  liveDetail,
  liveInventory,
  liveMe,
  liveReceipt,
  liveRevisions,
  liveSpaces,
  liveUnit,
  liveUnitData,
  liveUnits,
} from "./confighub.mjs";
import { buildApprovalScope, WORKFLOW } from "./workflow.mjs";

const PUBLIC_DIR = fileURLToPath(PUBLIC_URL);

const CONTENT_TYPES = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
  ".txt": "text/plain; charset=utf-8",
};

function send(res, response) {
  res.writeHead(response.status, response.headers);
  res.end(response.body);
}

function queryParam(url, name, fallback = "") {
  return url.searchParams.get(name) || fallback;
}

function modeFrom(url, config) {
  return queryParam(url, "mode", config.dataMode);
}

function isError(payload) {
  return payload && typeof payload === "object" && payload.error;
}

async function withDataMode(url, config, liveFn, fixtureFn) {
  const mode = modeFrom(url, config);
  if (mode === "fixture") return {source: "fixture", payload: await fixtureFn()};
  if (mode === "live") return {source: "live", payload: await liveFn()};
  let livePayload;
  try {
    livePayload = await liveFn();
  } catch (error) {
    livePayload = {error: String(error.message || error)};
  }
  if (!isError(livePayload)) return {source: "live", payload: livePayload};
  const fixturePayload = await fixtureFn();
  return {
    source: "fixture",
    warning: livePayload.error,
    payload: fixturePayload,
  };
}

async function handleApi(req, url, config) {
  if (req.method === "POST") {
    if (url.pathname === "/api/approvals" || url.pathname === "/api/apply") {
      return jsonResponse(
        {
          error: "read-only sample app: live mutation requires a governed approval and apply path",
          blocked: true,
        },
        405,
      );
    }
    return jsonResponse({error: "unsupported write route"}, 405);
  }

  if (url.pathname === "/api/workflow") {
    return jsonResponse(WORKFLOW);
  }

  if (url.pathname === "/api/me") {
    const result = await withDataMode(url, config, () => liveMe(config), fixtureMe);
    return jsonResponse({...result.payload, source: result.source, warning: result.warning});
  }

  if (url.pathname === "/api/spaces") {
    const result = await withDataMode(url, config, liveSpaces, fixtureSpaces);
    return jsonResponse({...result.payload, source: result.source, warning: result.warning});
  }

  if (url.pathname === "/api/inventory") {
    const result = await withDataMode(url, config, liveInventory, fixtureInventory);
    return jsonResponse({...result.payload, source: result.source, warning: result.warning});
  }

  if (url.pathname === "/api/units") {
    const space = queryParam(url, "space");
    const result = await withDataMode(url, config, () => liveUnits(space), () => fixtureUnits(space));
    return jsonResponse({items: result.payload.items || result.payload.Items || result.payload, source: result.source, warning: result.warning});
  }

  if (url.pathname === "/api/unit") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    const result = await withDataMode(url, config, () => liveUnit(space, unit), () => fixtureUnit(space, unit));
    return jsonResponse({item: result.payload.Unit || result.payload, source: result.source, warning: result.warning});
  }

  if (url.pathname === "/api/revisions") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    const result = await withDataMode(url, config, () => liveRevisions(space, unit), () => fixtureRevisions(space, unit));
    return jsonResponse({items: result.payload.items || result.payload.Items || result.payload, source: result.source, warning: result.warning});
  }

  if (url.pathname === "/api/unitdata") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    const result = await withDataMode(url, config, () => liveUnitData(space, unit), () => fixtureUnitData(space, unit));
    return jsonResponse({text: result.payload.text || result.payload || "", source: result.source, warning: result.warning});
  }

  if (url.pathname === "/api/detail") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    const result = await withDataMode(url, config, () => liveDetail(space, unit), () => fixtureDetail(space, unit));
    return jsonResponse({...result.payload, source: result.source, warning: result.warning});
  }

  if (url.pathname === "/api/proposal") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    const result = await withDataMode(url, config, () => liveDetail(space, unit), () => fixtureDetail(space, unit));
    return jsonResponse({
      source: result.source,
      warning: result.warning,
      approvalScope: buildApprovalScope(result.payload),
      previewOnly: true,
    });
  }

  if (url.pathname === "/api/receipt") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    const result = await withDataMode(url, config, () => liveReceipt(space, unit), () => fixtureReceipt(space, unit));
    return jsonResponse({...result.payload, source: result.source, warning: result.warning});
  }

  return jsonResponse({error: "unknown API route"}, 404);
}

async function serveStatic(url) {
  const requested = url.pathname === "/" ? "index.html" : url.pathname.replace(/^\/+/, "");
  const resolved = path.resolve(PUBLIC_DIR, requested);
  if (!resolved.startsWith(PUBLIC_DIR)) return textResponse("not found", 404);
  try {
    const body = await fs.readFile(resolved);
    return {
      status: 200,
      headers: {"content-type": CONTENT_TYPES[path.extname(resolved)] || "application/octet-stream"},
      body,
    };
  } catch {
    return textResponse("not found", 404);
  }
}

export function createAppServer(options = {}) {
  const config = {...runtimeConfig(), ...options};
  return http.createServer(async (req, res) => {
    const url = new URL(req.url || "/", `http://${req.headers.host || "localhost"}`);
    try {
      if (url.pathname.startsWith("/api/")) {
        send(res, await handleApi(req, url, config));
        return;
      }
      send(res, await serveStatic(url));
    } catch (error) {
      send(res, jsonResponse({error: String(error.message || error)}, 500));
    }
  });
}

export function listen(server, port) {
  return new Promise((resolve) => {
    server.listen(port, "127.0.0.1", () => resolve(server.address()));
  });
}

export async function main() {
  const config = runtimeConfig();
  const server = createAppServer(config);
  await listen(server, config.port);
  console.log(`Add-on Manager sample app: http://localhost:${config.port}`);
  console.log(`data mode: ${config.dataMode}`);
  return server;
}
