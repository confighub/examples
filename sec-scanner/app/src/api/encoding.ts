// UTF-8-safe base64 helpers. atob/btoa alone operate on Latin-1 and corrupt
// multibyte characters, which YAML config data may legally contain.

export function b64decodeUtf8(encoded: string): string {
  const bytes = Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

export function b64encodeUtf8(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
}
