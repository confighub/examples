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

function appConfig(req, config) {
  const origin = `http://${req.headers.host || `localhost:${config.port}`}`;
  return {
    app: "Add-on Manager",
    configHubBase: config.configHubBase,
    oauthClientId: config.oauthClientId,
    browserAuthConfigured: Boolean(config.configHubBase && config.oauthClientId),
    redirectUri: `${origin}/`,
    dataMode: config.dataMode,
  };
}

async function handleApp(req, url, config) {
  if (url.pathname === "/app/config") {
    return jsonResponse(appConfig(req, config));
  }
  return jsonResponse({error: "unknown app route"}, 404);
}

async function handleApi(req, url) {
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
    return jsonResponse({...await fixtureMe(), source: "fixture"});
  }

  if (url.pathname === "/api/spaces") {
    return jsonResponse({...await fixtureSpaces(), source: "fixture"});
  }

  if (url.pathname === "/api/inventory") {
    return jsonResponse({...await fixtureInventory(), source: "fixture"});
  }

  if (url.pathname === "/api/units") {
    const space = queryParam(url, "space");
    return jsonResponse({items: await fixtureUnits(space), source: "fixture"});
  }

  if (url.pathname === "/api/unit") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    return jsonResponse({item: await fixtureUnit(space, unit), source: "fixture"});
  }

  if (url.pathname === "/api/revisions") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    return jsonResponse({items: await fixtureRevisions(space, unit), source: "fixture"});
  }

  if (url.pathname === "/api/unitdata") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    return jsonResponse({text: await fixtureUnitData(space, unit), source: "fixture"});
  }

  if (url.pathname === "/api/detail") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    return jsonResponse({...await fixtureDetail(space, unit), source: "fixture"});
  }

  if (url.pathname === "/api/proposal") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    const detail = await fixtureDetail(space, unit);
    return jsonResponse({
      source: "fixture",
      approvalScope: buildApprovalScope(detail),
      previewOnly: true,
    });
  }

  if (url.pathname === "/api/receipt") {
    const space = queryParam(url, "space");
    const unit = queryParam(url, "unit");
    return jsonResponse({...await fixtureReceipt(space, unit), source: "fixture"});
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
        send(res, await handleApi(req, url));
        return;
      }
      if (url.pathname.startsWith("/app/")) {
        send(res, await handleApp(req, url, config));
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
  console.log(`fixture mode: local sample data`);
  console.log(`browser OAuth configured: ${Boolean(config.oauthClientId)}`);
  return server;
}
