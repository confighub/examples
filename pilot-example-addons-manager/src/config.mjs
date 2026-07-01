export const ROOT_URL = new URL("../", import.meta.url);
export const PUBLIC_URL = new URL("../public/", import.meta.url);
export const FIXTURES_URL = new URL("../fixtures/", import.meta.url);
export const DATA_URL = new URL("../data/", import.meta.url);

export function runtimeConfig(env = process.env) {
  const configHubBase = (
    env.CONFIGHUB_BASE ||
    env.VITE_CONFIGHUB_BASE_URL ||
    "https://hub.confighub.com"
  ).replace(/\/+$/, "");
  const oauthClientId = env.OAUTH_CLIENT_ID || env.VITE_OAUTH_CLIENT_ID || "";
  return {
    port: Number(env.PORT || 5173),
    dataMode: env.DATA_MODE || "fixture",
    configHubBase,
    oauthClientId,
    liveBindingsFile: env.LIVE_BINDINGS_FILE || "",
  };
}

export function jsonResponse(payload, status = 200, headers = {}) {
  return {
    status,
    headers: {
      "content-type": "application/json; charset=utf-8",
      ...headers,
    },
    body: JSON.stringify(payload, null, 2) + "\n",
  };
}

export function textResponse(body, status = 200, contentType = "text/plain; charset=utf-8") {
  return {
    status,
    headers: {"content-type": contentType},
    body,
  };
}
