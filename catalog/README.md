# Public example catalog

This small index helps people and agents find a current public guide for a
specific task. It links to the owning repository; it does not copy or execute
its example. Product examples remain with the code and tests that maintain them.
If you are new to the model, start with [What an app looks like](./FIRST_APP.md):
one three-component app, a read-only plan and a visible local edit.

From the repository root:

```bash
node scripts/examples-catalog.mjs list --json
node scripts/examples-catalog.mjs search 'what an app looks like' --json
node scripts/examples-catalog.mjs search 'existing Argo repository' --json
node scripts/examples-catalog.mjs search 'Helm values' --all --json
node scripts/examples-catalog.mjs show argo-beginner-applicationset --json
```

`list` and `search` return JSON with `schema_version` and `entries`. `search`
also returns `query`. Results are sorted by word match, then stable ID; an
empty result is an empty `entries` array. `show` returns `schema_version` and
one `entry`, including candidates. `--tag TAG` filters by an exact tag.
All commands are local, read-only, and require only Node.js. No command fetches
a linked guide or runs its preview.

Default `list` and `search` include entries with all four fields:

```json
{
  "visibility": "public",
  "lifecycle": "maintained",
  "role": "walkthrough",
  "admission": "verified-source"
}
```

`verified-source` means the public source path, guides and local entrypoint
were checked, and the listed local preview or test ran. Local checks may write
temporary artifacts, as `effects` describes. It is **not** a
connected, controller, hardware or runtime certification. Read `evidence` and
`stop_when` on each entry before acting. `--all` includes source-reviewed
entries and advanced references so the user can research them, while their
qualification work remains visible. For example, the promotion data source
needs an authorized ConfigHub setup to test its useful flow; the bounded Helm
change guide already proves a local allowed and blocked edit. The GPU recipe's
preview does not prove an H100 run.

For a selected entry, read `guides.human` and `guides.ai` in the owning
repository, check `source.revision` against its current guide, then run only the
documented `preview.command` with the stated prerequisites. Guide URLs point to
the pinned public source. An optional `tutorial` path resolves from this
repository root and is pinned by the catalog commit.
The source's current instructions take precedence over this index if they have
changed. A source revision drift calls for review and fresh qualification;
it does not renew evidence automatically.

`catalog/examples.json` is the version 1 source for this interface.
`catalog/qualification.json` holds sanitized local output projections and the
exact command and source revision behind admitted entries. The repository
verifier checks the schema, link shapes, local paths, receipt binding,
eligibility and search cases. Adding an entry requires a distinct user task, canonical
public source and revision, lifecycle and role, a deterministic preview,
effects and stop condition, and separate static, connected, controller and
runtime evidence. Use `source-reviewed` until the useful local path is run;
keep private, historical, simulated and unimplemented material out of the
default runnable results.
