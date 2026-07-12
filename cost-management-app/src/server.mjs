import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { extname, join, normalize } from 'node:path';
import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { readRuntimeConfig } from './config.mjs';
import { reviewPacketStatus } from './executor.mjs';
import { classifyLiveBindings } from './live-bindings.mjs';
import { readWorkflow } from './workflow.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const publicDir = join(root, 'public');

const contentTypes = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml; charset=utf-8',
};

function sendJson(res, status, value) {
  res.writeHead(status, {'content-type': 'application/json; charset=utf-8'});
  res.end(JSON.stringify(value, null, 2));
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let data = '';
    req.on('data', chunk => { data += chunk; });
    req.on('end', () => {
      if (!data) {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(data));
      } catch (error) {
        reject(error);
      }
    });
  });
}

function hasBearer(req) {
  const auth = req.headers.authorization || '';
  return auth.startsWith('Bearer ');
}

async function readLiveBindings() {
  let raw;
  try {
    raw = await readFile(join(root, 'data', 'live-bindings.json'), 'utf8');
  } catch (error) {
    if (error && error.code !== 'ENOENT') {
      return {status: 'LIVE_BINDINGS_UNREADABLE', verdict: 'ERROR', reason: 'LIVE_BINDINGS_UNREADABLE', bindings: null};
    }
    return {
      ...classifyLiveBindings(null),
      bindings: null,
      requiredFile: 'data/live-bindings.json',
      exampleFile: 'data/live-bindings.example.json',
    };
  }
  let bindings;
  try {
    bindings = JSON.parse(raw);
  } catch {
    return {status: 'LIVE_BINDINGS_UNPARSEABLE', verdict: 'ERROR', reason: 'LIVE_BINDINGS_UNPARSEABLE', bindings: null};
  }
  return {...classifyLiveBindings(bindings), bindings};
}

async function serveStatic(req, res, url) {
  const requested = url.pathname === '/' || url.pathname === '/callback'
    ? '/index.html'
    : url.pathname;
  const safePath = normalize(requested).replace(/^\.{2,}/, '');
  const filePath = join(publicDir, safePath);
  if (!filePath.startsWith(publicDir)) {
    sendJson(res, 403, {error: 'FORBIDDEN'});
    return;
  }
  try {
    const body = await readFile(filePath);
    res.writeHead(200, {'content-type': contentTypes[extname(filePath)] || 'application/octet-stream'});
    res.end(body);
  } catch {
    sendJson(res, 404, {error: 'NOT_FOUND'});
  }
}

function authBlocked(req, workflow) {
  return workflow.authMode === 'browser-oauth' && !hasBearer(req);
}

export function createAppServer() {
  return createServer(async (req, res) => {
    const url = new URL(req.url || '/', 'http://localhost');
    const workflow = await readWorkflow();
    const runtime = readRuntimeConfig();
    workflow.authMode = runtime.authMode;

    try {
      if (req.method === 'GET' && url.pathname === '/app/config') {
        sendJson(res, 200, {
          authMode: runtime.authMode,
          configHubBaseUrl: runtime.configHubBaseUrl,
          oauthClientId: runtime.oauthClientId,
          callbackPath: '/',
          workflowStatus: workflow.status,
        });
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/workflow') {
        sendJson(res, 200, workflow);
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/cost-findings') {
        try {
          const report = JSON.parse(await readFile(join(root, 'data', 'cost-findings.json'), 'utf8'));
          sendJson(res, 200, report);
        } catch {
          sendJson(res, 404, {
            error: 'COST_SWEEP_NOT_RUN',
            message: 'No findings exist for this deployment yet.',
          });
        }
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/variants') {
        sendJson(res, 200, {variants: workflow.variants});
        return;
      }

      if (req.method === 'GET' && url.pathname.startsWith('/api/variants/')) {
        const id = decodeURIComponent(url.pathname.split('/').pop() || '');
        const variant = workflow.variants.find(item => item.id === id);
        sendJson(res, variant ? 200 : 404, variant || {error: 'VARIANT_NOT_FOUND'});
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/receipt') {
        const liveBindings = await readLiveBindings();
        const reviewPacket = reviewPacketStatus();
        sendJson(res, 200, {
          verdict: 'WATCH',
          reason: !liveBindings.reviewReady
            ? liveBindings.reason
            : (reviewPacket.reason === 'LOCAL_REVIEW_RECORDED'
              ? 'EXECUTION_CONFIRMATION_REQUIRED'
              : (reviewPacket.reason === 'CONFIG_REVISION_COMMITTED' || reviewPacket.recordedOutcome?.reason === 'CONFIG_REVISION_COMMITTED'
                ? 'CONFIG_REVISION_COMMITTED_DELIVERY_PENDING'
                : reviewPacket.reason)),
          status: 'WAITING_FOR_LIVE_PROOF',
          app: workflow.app.name,
          checks: workflow.proofTabs.map(tab => ({id: tab.id, label: tab.label, status: tab.status})),
          liveBindings,
          reviewPacket,
          stopRules: workflow.stopRules,
        });
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/bindings') {
        sendJson(res, 200, await readLiveBindings());
        return;
      }

      if (req.method === 'POST' && url.pathname === '/api/preview') {
        if (authBlocked(req, workflow)) {
          sendJson(res, 401, {error: 'SIGN_IN_REQUIRED'});
          return;
        }
        await readBody(req);
        sendJson(res, 200, {
          verdict: 'WATCH',
          reason: 'EXACT_FINDING_REQUIRED',
          status: 'PREVIEW_HANDOFF_REQUIRED',
          message: 'The browser cannot create a mutation preview from Variant scope alone. Use the CLI findings route to select one exact finding and generate its dry-run diff.',
          nextCommand: 'node cli.mjs findings --json',
        });
        return;
      }

      if (req.method === 'POST' && url.pathname === '/api/review') {
        if (authBlocked(req, workflow)) {
          sendJson(res, 401, {error: 'SIGN_IN_REQUIRED'});
          return;
        }
        await readBody(req);
        sendJson(res, 200, {
          verdict: 'BLOCK',
          reason: 'STORED_PREVIEW_REQUIRED',
          status: 'REVIEW_HANDOFF_REQUIRED',
          message: 'The browser does not record local review evidence from Variant scope. Create and inspect an exact CLI preview first.',
          nextCommand: 'node cli.mjs preview --finding <finding-id> --json',
        });
        return;
      }

      if (req.method === 'POST' && url.pathname === '/api/apply') {
        if (authBlocked(req, workflow)) {
          sendJson(res, 401, {error: 'SIGN_IN_REQUIRED'});
          return;
        }
        const liveBindings = await readLiveBindings();
        if (!liveBindings.reviewReady) {
          sendJson(res, 409, {
            error: 'LIVE_BINDINGS_REQUIRED',
            reason: liveBindings.reason,
            message: 'Resolve the exact-review authority gaps before checking the commit gate.',
            liveBindings,
          });
          return;
        }
        sendJson(res, 409, {
          error: 'BROWSER_EXACT_REVIEW_REQUIRED',
          message: 'The browser does not yet hand an exact finding-owned review packet to the shared executor. Use the CLI review and explicitly confirmed commit path; do not substitute a Variant-only browser action.',
          nextCommand: 'node cli.mjs findings --json',
        });
        return;
      }

      if (req.method === 'GET') {
        await serveStatic(req, res, url);
        return;
      }

      sendJson(res, 405, {error: 'METHOD_NOT_ALLOWED'});
    } catch (error) {
      sendJson(res, 500, {error: 'SERVER_ERROR', message: String(error.message || error)});
    }
  });
}
