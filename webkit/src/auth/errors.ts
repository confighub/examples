// The two ways the browser-direct flow fails that a raw IdP message does not explain.
// Both are configuration mistakes with a specific fix, and both otherwise surface as a
// wall of OAuth text that reads like a bug in the app.

/**
 * A plain-language explanation of an auth error, or undefined when the message already
 * says what to do. `appName` names the client to register in the suggested command.
 */
export function explainAuthError(message: string, appName: string): string | undefined {
  if (/not a member of the organization that owns this app/i.test(message)) {
    return (
      "You signed in to a different organization than the one that owns this app's client " +
      'id. Sign in with the organization you registered the client in — or register a ' +
      `client in the organization you just used:  cub oauthclient create ${appName} ` +
      '--redirect-uri <this origin>'
    );
  }
  if (/redirect_uri/i.test(message)) {
    return 'The origin serving this app is not a registered redirect URI for this client id.';
  }
  return undefined;
}
