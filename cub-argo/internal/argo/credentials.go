package argo

// credentialHelpers are the shell functions a script that writes a credential
// Secret defines after k(). They read ConfigHub with cub's own -o jq, so the
// scripts need no jq of their own.
//
// The Secrets are written with create or replace, never kubectl apply: apply
// keeps a copy of what it applied, token or private key included, in the
// Secret's kubectl.kubernetes.io/last-applied-configuration annotation, and
// replace drops a copy an earlier version left.
func credentialHelpers() []string {
	return []string{
		"# write_secret <namespace> <name>: writes the Secret the YAML on stdin",
		"# describes, with create, or replace when it exists. Not kubectl apply, which",
		"# keeps a copy of what it applies, credentials and all, in the Secret's",
		"# last-applied-configuration annotation; replace drops a copy left there.",
		`write_secret() {`,
		`  local yaml`,
		`  yaml=$(cat)`,
		`  if k -n "$1" get secret "$2" >/dev/null 2>&1; then`,
		`    printf '%s\n' "$yaml" | k replace -f - >/dev/null`,
		`  else`,
		`    printf '%s\n' "$yaml" | k create -f - >/dev/null`,
		`  fi`,
		`  echo "secret/$2 written in $1"`,
		`}`,
		"# live_tokens <account> <description>: the ids of the account's tokens with",
		"# that description that still work. token list shows revoked and expired",
		"# ones too.",
		`live_tokens() {`,
		`  cub serviceaccount token list "$1" -o jq='.[].UserToken | select((.RevokedAt // "") == "" and ((.ExpiresAt // "") == "" or ((.ExpiresAt | sub("\\.[0-9]+"; "")) | fromdateiso8601) > now)) | .Jti + " " + (.Description // "")' | tr -d '"' |`,
		`    while read -r id desc; do if [ "$desc" = "$2" ]; then echo "$id"; fi; done`,
		`}`,
		"# key_ids <account> [description]: the ids of the account's keys, or of those",
		"# with that description.",
		`key_ids() {`,
		`  cub serviceaccount key list "$1" -o jq='.[].UserKey | .Kid + " " + (.Description // "")' | tr -d '"' |`,
		`    while read -r id desc; do if [ $# -lt 2 ] || [ "$desc" = "$2" ]; then echo "$id"; fi; done`,
		`}`,
	}
}
