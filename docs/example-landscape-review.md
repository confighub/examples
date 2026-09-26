# Example landscape and maintenance review

**Maintainer record.** For the public example directory, use the
[main README](../README.md). This audit supports decisions about ownership,
consolidation and qualification; it is not another user starting point.

Reviewed 2026-09-26. This is a source inventory and a maintenance recommendation,
not a new runnable catalog or a certification of the linked examples.

## Decision

Use `confighub/examples` as the place to find a useful ConfigHub learning journey.
Keep product-specific examples beside the product that tests them, and link to
that canonical home. Move an example only when this repository will own its
instructions, dependencies and verification after the move. More copied examples
would create more stale instructions without helping a user finish a task.

The highest-value additions to discovery are: fix an existing chart; trace a
configuration field back to its source; manage ordinary application settings;
and inspect whether a specific release reached its target. Platform and GPU
journeys should build on those entry points, with their additional prerequisites
and proof limits visible.

## What is already here beyond GitOps

These are families, not equal units of maintenance or live proof. Follow each
example's README and contract for its actual effects and prerequisites.

| User need | Current paths |
| --- | --- |
| Layer an application across environments | [global-app-layer](../global-app-layer/README.md): single-component, frontend-postgres, realistic-app, gpu-eks-h100-training and enterprise-rag-blueprint. A GPU-shaped configuration is not evidence of an H100 run. |
| Change app settings and understand platform ownership | [spring-platform](../spring-platform/README.md), with the current source-generation product path in cub-gen below. |
| Promote and govern changes | [promotion-demo-data](../promotion-demo-data/README.md), its `verify/` companion, [promoter](../promoter/README.md), and [initiatives-demo](../initiatives-demo/README.md). |
| Control security, permissions and cost | [sec-scanner](../sec-scanner/README.md), [rbac-manager](../rbac-manager/README.md), [cost-estimator](../cost-estimator/README.md), plus rbac-manager-for-agents, rbac-manager-over-redis and redis-platform-with-rbac-guardrails. |
| Operate a fleet | workload-manager, namespace-manager, network-policy-manager, scheduling-manager, autoscale-manager, observability-manager and eks-manager; linked from the [main README](../README.md). |
| Build useful operational applications | [configboard](../configboard/README.md), [fleet-ql](../fleet-ql/README.md), [pilot-example-addons-manager](../pilot-example-addons-manager/README.md) and [cost-management-app](../cost-management-app/README.md). |
| Extend or transform configuration | [custom-workers](../custom-workers/), [k8s-to-score](../k8s-to-score/README.md) and [prompts for an existing repository](../prompts/README.md). |

`managerkit` and `webkit` are shared implementation libraries, not additional
user journeys. The [GitOps index](../gitops/README.md) remains the detailed map
for Argo CD and Flux; this review does not replace it.

## Examples worth tracking in their existing homes

Sources below are pinned to the reviewed revision. Use the owning repository's
current instructions before running anything. “Index in place” is a recommended
maintenance disposition, not a statement that this repository tests that source.
No linked example was executed during this audit.

| User problem | Source at reviewed revision | Recommendation and evidence limit |
| --- | --- | --- |
| Repair and review existing Helm/config changes | [confighub/cub-workshop/examples](https://github.com/confighub/cub-workshop/tree/044f0a560657c9cef07e9320ee6f8911f8881220/examples) | **Index in place.** Adapt, Match and stack CI fixtures stay with the plugin that executes them. The stack-CI README separates local checks from deployment and rollback proof. |
| Trace a Helm field to source and its owner | [confighub/cub-gen/examples/helm-paas](https://github.com/confighub/cub-gen/tree/fa70ffad379765d8052f6873ccdb51b8edf4b3e7/examples/helm-paas) | **Index in place.** Documented local ownership checks and a connected runtime wrapper. Umbrella charts, fan-out and external values have explicit gaps; inspect the exact route before presenting it. |
| Change a Spring app while respecting platform ownership | [confighub/cub-gen/examples/springboot-paas](https://github.com/confighub/cub-gen/tree/fa70ffad379765d8052f6873ccdb51b8edf4b3e7/examples/springboot-paas) | **Index in place.** Successor to the old configuration write-API example. Local, connected and runtime paths have different evidence. The README says refresh preview is client-side and server-side merge support is not implemented. |
| Manage app settings without a Kubernetes platform | [confighub/cub-gen/examples/just-apps-no-platform-config](https://github.com/confighub/cub-gen/tree/fa70ffad379765d8052f6873ccdb51b8edf4b3e7/examples/just-apps-no-platform-config) | **Index in place.** Provider/channel configuration and source provenance. The README explicitly lacks provider-runtime proof; the rendered representation alone does not establish a provider update. |
| Inspect non-Kubernetes application configuration | [confighub/cub-scout/examples/app-config-rtmsg](https://github.com/confighub/cub-scout/tree/adf328e246b57f08f2d75d3307dac5987e63dcb8/examples/app-config-rtmsg) | **Index in place.** Real-time messaging configuration illustrates an additional non-GitOps family. Keep its inspection behavior with Scout; qualify any connected claim separately. |
| Help an agent inspect infrastructure | [confighub/cub-scout/examples/ai-integration](https://github.com/confighub/cub-scout/tree/adf328e246b57f08f2d75d3307dac5987e63dcb8/examples/ai-integration) | **Index in place.** Offline CLI and MCP integration examples. A useful entry point for agent users, separate from a live cluster or ConfigHub import claim. |
| Propose an import from a saved bundle | [confighub/cub-scout/examples/import-from-bundle](https://github.com/confighub/cub-scout/tree/adf328e246b57f08f2d75d3307dac5987e63dcb8/examples/import-from-bundle) | **Index in place.** Committed expected proposal output; no cluster needed for the dry run. A proposal does not establish a successful import. |
| Check whether an exact OCI release reached its target | [confighub/cub-scout/examples/oci-release-check](https://github.com/confighub/cub-scout/tree/adf328e246b57f08f2d75d3307dac5987e63dcb8/examples/oci-release-check) | **Index in place.** Version-specific release and running-image checks with deterministic test references. Synthetic digest fixtures are not pullable images; application success remains outside the check. |
| Make a small change to an OCI configuration without an account | [confighub/helm-expt/examples/anonymous-oci-transform](https://github.com/confighub/helm-expt/tree/a2b11cea8a26ef0ca95377c2dd746179ffdba799/examples/anonymous-oci-transform) | **Index in place.** Retained before/after hashes, five-object output and proof-summary link. No cluster or server used; external Secret requirement remains visible. |
| Apply the same stack checks in pull requests | [confighub/cub-workshop/examples/stack-ci](https://github.com/confighub/cub-workshop/tree/044f0a560657c9cef07e9320ee6f8911f8881220/examples/stack-ci) | **Index in place.** Copyable workflow tied to a pinned plugin. Local authored/rendered components work without a ConfigHub account; OCI sources have additional tool and registry requirements. |
| Build a browser application using the ConfigHub SDK | [confighub/js-sdk/examples/space-browser](https://github.com/confighub/js-sdk/tree/499bd223bb3562c15bdf390b8e5f2b49e3e218de/examples/space-browser) | **Index in place.** Plain browser sample; the sibling space-browser-rtk shows Redux integration. Keep both next to SDK changes and authentication documentation. |
| Adopt a Kubara platform | [confighub/kubara-confighub/examples/mini-kubara](https://github.com/confighub/kubara-confighub/tree/a5bbe1fd20838d162e53cd8802293420c7ae53b5/examples/mini-kubara) | **Advanced reference.** This is the recorded successor of the old mini-kubara example. Keep the integration, receipts and version requirements together; do not confuse it with cub-gen's explicitly Kubara-like pattern. |
| Govern delivery across multiple clusters | [confighub/sveltos-confighub/](https://github.com/confighub/sveltos-confighub/tree/1a6958a7347638960bb779b71bbdcfb183ca51d9/) | **Advanced reference.** Scenario locks, verifier and receipts belong to the integration repo. Its documented addon-controller version is a material prerequisite, not a generic Sveltos compatibility claim. |
| Explore EKS inference infrastructure | [confighub/eks-inference/](https://github.com/confighub/eks-inference/tree/a015b65811a9dd92f8c452d11401bb8b97b06c62/) | **Advanced reference.** Separate the configuration-only sandbox from cloud provisioning. Actual EKS/GPU runs require credentials, capacity, cost controls and teardown. |
| Seed a synthetic fleet for product exploration | [confighub/cub-demo/](https://github.com/confighub/cub-demo/tree/213336c7f8ec3f9602cec10b7424e7c422f4af95/) | **Tool reference.** A scenario seeder with substantial org quotas, not a small tutorial. Keep scenario execution and tests with the plugin. |
| Reuse a representative multi-service workload | [confighub/cubbychat/](https://github.com/confighub/cubbychat/tree/e9e76a076924d95897c3ede7a0f21cec523c4f6f/) | **Companion code.** App source belongs here. Its README points to a historical global-app path; use the current global-app-layer index when choosing a ConfigHub walkthrough instead of copying the app. |

Additional source-generation families remain under
[cub-gen/examples](https://github.com/confighub/cub-gen/tree/fa70ffad379765d8052f6873ccdb51b8edf4b3e7/examples): Score, Backstage,
OpenChoreo, agent-fleet configuration, operations workflows and Swamp automation.
They broaden the inventory beyond Kubernetes charts, but their source/rendered
configuration contracts must not be presented as proof of execution by those
external systems. `demo/` is a script collection and `live-reconcile/` is a
product proof harness, not two more independent tutorials.

## Do not revive old paths or promote outlines as working examples

- [Consolidation PR #242](https://github.com/confighub/examples/pull/242) records
  the moves and deletions; [#244](https://github.com/confighub/examples/pull/244)
  cleans up remaining references. The
  [historical inventory](https://github.com/confighub/examples/blob/4ff37f7e284be78c11da69baf48b967ba9a5830f/incubator/README.md)
  is useful history, not a current directory map.
- The write-API example now belongs to cub-gen's `springboot-paas`; `cub-proc`
  moved to cub-gen's experimental examples; mini-kubara moved to
  kubara-confighub. Deleted Scout adaptations should generally link back to
  Scout rather than be recreated here.
- Scout's `onboard-existing-argocd` and `onboard-existing-flux` READMEs explicitly
  say **planned**, with setup/verify/cleanup scripts still to be added. Its
  `rm-demos-argocd` says **simulation**. None should be listed as a working
  takeover or live-delivery demonstration.
- Scout's older Argo import guide says `cub gitops import` was removed. A source
  that documents the old path is useful history, not an instruction to revive
  the command. Confirm the current upload/import boundary for any adaptation.
- `confighub/flux-bridge` explicitly says its protocol was removed and the
  integration is no longer functional. Keep it out of recommended starts.
- Catalog retained fixtures, source trees, render outputs and refusal corpora
  stay in helm-expt alongside their generators and evidence. Their quantity is
  not a count of runnable user journeys.

## Which excluded material is worth updating?

Exclusion from today's runnable list is not a decision to discard the user need.
Keep the following outcomes, while replacing stale instructions in their owning
repositories. These are qualification decisions; no runtime status changes here.

| Material | Decision | Useful outcome and condition for admission |
| --- | --- | --- |
| Scout's planned Argo and Flux onboarding outlines | Keep the goals; update and implement against current product behavior | Explain an existing app, identify source and delivery authority, and prepare a reviewed handoff. Start with one common shape per controller. A successful upload is not proof that the controller changed source. |
| Scout's old import demos | Retain diagnostic fixtures; replace obsolete command paths | Preserve useful discovery and render cases. Merge the user walkthrough into the current onboarding path instead of maintaining a second importer. |
| Scout's simulated Argo demonstrations | Preserve as labelled historical/design references; extract useful acceptance questions | Rebuild only questions covered by a selected real journey. Simulated transcripts stay out of default runnable-example retrieval. |
| Obsolete flux-bridge and legacy integration copies | Flag for archival review; point to supported successors | Do not revive a removed protocol. Preserve history where useful and verify the replacement link before retirement. |
| Detector, ownership, hook and refusal fixtures | Keep with product tests | These are valuable regression and agent-evaluation inputs. Label them as fixtures, not user walkthroughs or proof of a live deployment. |
| The old mini-kubara and source-generation copies | Use the successors recorded in #242 | Preserve any unique acceptance requirements in the current canonical homes; do not restore deleted copies. |

Before removing anything, identify its successor or record why none is needed,
check incoming links and tests, and preserve relevant historical evidence. This
review flags archival candidates; it does not delete or archive repositories.

## A discoverable corpus for Pilot and Workshop

The next deliverable should be a small shared example index that points to
canonical guides and artifacts. Keep one human-readable front door here; do not
require a user to understand the inventory's internal categories before choosing
a useful task. Agent retrieval should start with the user's problem and return
the same maintained guide a person can follow.

Proposed minimum entry contract:

- Stable ID, plain-language user task, canonical source URL/path and pinned revision.
- Source visibility, accepting maintainer and lifecycle status: candidate,
  maintained, needs-refresh, superseded or historical.
- Human guide and AI guide; supported tool versions, prerequisites, effects,
  credentials/cost requirements and cleanup.
- Deterministic command or workflow, expected useful artifact, and explicit
  refusal/stop conditions. Do not invent an executable command for an outline.
- Evidence references, separately stating static, connected, controller and
  runtime coverage, with last verified date and known gaps.
- Successor reference when superseded; role as walkthrough, test fixture or
  historical reference. A fixture passing is not a user journey passing.

Default runnable retrieval should select explicitly admitted maintained entries
with public, verified source references and prerequisites the agent can explain.
Private records stay access-controlled; historical, simulated and unimplemented
material requires an explicit research/test query. A maintained local-only
example remains useful: its limits must travel with the result. Drift in pinned
sources or supported commands triggers review, not automatic proof renewal.

Acceptance for this corpus: given an existing-chart, Argo/Flux, app-configuration,
platform or GPU request, Pilot and Workshop can find the appropriate guide,
explain its prerequisites, produce its declared artifact using supported steps,
and stop honestly at missing authority or proof. Use a fresh agent and the same
manual path. **This PR defines the direction; it does not implement ingestion or
claim either tool already consumes this review.**

## Small follow-up queue

No migration is approved by this review. These are the next qualification tasks,
ordered by user benefit; repository maintainers still need to accept ownership.

| Priority | Outcome for a user | Minimal follow-up and completion evidence |
| --- | --- | --- |
| 1 | Find the right path for an existing chart or config problem | Use the Workshop and cub-gen links above; try the selected current instructions in a clean environment and record the first useful artifact, exact versions and where the user stops. Fix that path in its owning repo rather than fork it. |
| 2 | See that ordinary app settings are also configuration | Qualify the app-only provider example and Scout messaging example; retain one small success and one refusal, distinguishing local configuration inspection from a provider-runtime result. |
| 3 | Know whether a reviewed release reached its target | Qualify Scout's OCI check for one supported target and keep unknowns explicit. Preserve its provider tests and receipts in Scout. |
| 4 | Build a browser integration without inventing auth | Qualify the plain SDK sample, then link the RTK variation. Record required OAuth registration and safe credential handling; no SDK copy in this repo. |
| 5 | Progress to platform or GPU operation | Use the existing layered recipes and dedicated Kubara/Sveltos/EKS repositories. Run only the declared environment; retain receipts for configuration, controller observation and application health separately. |

Before a candidate becomes a maintained runnable example here, require:

1. A distinct user outcome and a check against existing examples and open PRs.
2. An accepting maintainer and one canonical home; preserve license/attribution.
3. A self-contained or explicitly versioned input, documented tools, credentials,
   cost, effects and cleanup; no private or environment-specific data.
4. Human and AI entry points and machine-readable contracts under
   [EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md).
5. A local verifier, expected useful output and a failure/refusal case, plus
   separately recorded connected/controller/runtime proof for claims that need it.
6. A link or redirect from the previous home, with duplicate scripts removed
   only after the successor works.

The current [repository verifier](../scripts/verify.sh) lists 19 AI-guide examples,
with two explicit contract/explain exemptions. That is coverage of a particular
check, not the total number of examples or a live qualification count. It also
runs local addons-manager tests. A green verifier does not prove every linked
external example or a deployment.

Open PRs [#124](https://github.com/confighub/examples/pull/124) (onboarding),
[#141](https://github.com/confighub/examples/pull/141) (Spring proof docs) and
[#146](https://github.com/confighub/examples/pull/146) (Ctrlplane mapper) were
checked for overlap. This review adds a reference link rather than rewriting
onboarding or duplicating their proposed examples. Open status is a dated
observation, not a request to merge them.

## Public repository coverage

All 27 repositories visible as public across the two reviewed organizations
received default-branch tree and root-README triage where present. Deeper
inspection sampled example READMEs, contracts, scripts and test/receipt paths.
No returned tree was truncated. This is not a line-by-line code audit, a license
clearance or a run of the examples. Repositories without a recommended migration
still appear below so “not selected” is distinguishable from “not reviewed.”

| Repository at reviewed revision | Disposition |
| --- | --- |
| [confighub/CRDs-catalog@e3b5c395](https://github.com/confighub/CRDs-catalog/tree/e3b5c395adf9786ece082a1e38114684cc1e4a8d/) | Aggregated CRD schema corpus for external validators, not a worked user workflow. |
| [confighub/argobot@448f2e0c](https://github.com/confighub/argobot/tree/448f2e0c50f45afb38b6c36e1da62203784a7e60/) | Product daemon; tests are implementation evidence, with no self-contained learning scenario. |
| [confighub/confighub-skills@e380b058](https://github.com/confighub/confighub-skills/tree/e380b0589f1ebee66f314676663baaffae8e4228/) | Agent skill distribution and eval fixtures belong with the evolving skills product. |
| [confighub/cub-che@6377db4c](https://github.com/confighub/cub-che/tree/6377db4c823ea3c55d931634ebd48607551382e4/) | Enterprise self-hosted installer needs Docker/kind, GHCR credentials and full stack. |
| [confighub/cub-commander@7ffc9c5f](https://github.com/confighub/cub-commander/tree/7ffc9c5fa04027e1df19132d09b14364bbba395e/) | CLI product docs/examples depend on interactive product surfaces. |
| [confighub/cub-demo@213336c7](https://github.com/confighub/cub-demo/tree/213336c7f8ec3f9602cec10b7424e7c422f4af95/) | Large synthetic-org seeder is a product demo/scale fixture, not tutorial material. |
| [confighub/cub-gen@fa70ffad](https://github.com/confighub/cub-gen/tree/fa70ffad379765d8052f6873ccdb51b8edf4b3e7/) | Index selected source-generation journeys; keep generators and their fixtures together. |
| [confighub/cub-helm@0ebc4bf7](https://github.com/confighub/cub-helm/tree/0ebc4bf7ac64f083d31fedf93fb84777c87837c5/) | Chart plugin workflow and its guide belong with the plugin. |
| [confighub/cub-scout@adf328e2](https://github.com/confighub/cub-scout/tree/adf328e246b57f08f2d75d3307dac5987e63dcb8/) | Index selected diagnostic journeys; keep detector fixtures, simulations and planned onboarding distinct. |
| [confighub/cub-server@88039619](https://github.com/confighub/cub-server/tree/88039619335d9a19ad26cd3bc99a3a017dd297b9/) | Self-hosted installer carries evaluation-license constraints and substantial prerequisites. |
| [confighub/cub-workshop@044f0a56](https://github.com/confighub/cub-workshop/tree/044f0a560657c9cef07e9320ee6f8911f8881220/) | Index user-facing plugin examples; retain CLI and CI fixtures with the plugin. |
| [confighub/cubbychat@e9e76a07](https://github.com/confighub/cubbychat/tree/e9e76a076924d95897c3ede7a0f21cec523c4f6f/) | Companion workload; its historical global-app link needs reconciliation with the current global-app-layer entry. |
| [confighub/eks-inference@a015b658](https://github.com/confighub/eks-inference/tree/a015b65811a9dd92f8c452d11401bb8b97b06c62/) | Useful AI infrastructure story with free config-only route, but actual EKS/GPU path is high cost and setup. |
| [confighub/examples@2de38909](https://github.com/confighub/examples/tree/2de389098e32179da7a5b008fdc0443a2508d080/) | Canonical learning examples; preserve existing GitOps index and maintenance contracts. |
| [confighub/flux-bridge@8e85e01d](https://github.com/confighub/flux-bridge/tree/8e85e01d538776f3aaf963e95387cae80d2eb0d8/) | README says protocol removed and integration no longer functional; point users to current GitOps guide. |
| [confighub/helm-expt@a2b11cea](https://github.com/confighub/helm-expt/tree/a2b11cea8a26ef0ca95377c2dd746179ffdba799/) | Index selected guides; retain Catalog source and proof corpus with generators. |
| [confighub/homebrew-tap@73ee344d](https://github.com/confighub/homebrew-tap/tree/73ee344d52458f4182a9ffc1b5214eecf77b210b/) | Release packaging/formula metadata only. |
| [confighub/installer@1613d8cb](https://github.com/confighub/installer/tree/1613d8cb7822240662739f0d165a75132dd9f174/) | Installer framework starter fixtures are product samples; no distinct ConfigHub user story identified. |
| [confighub/js-sdk@499bd223](https://github.com/confighub/js-sdk/tree/499bd223bb3562c15bdf390b8e5f2b49e3e218de/) | Plain and RTK browser examples are useful developer material; maintain in SDK repo and index. |
| [confighub/kubara-confighub@a5bbe1fd](https://github.com/confighub/kubara-confighub/tree/a5bbe1fd20838d162e53cd8802293420c7ae53b5/) | Distinct recorded Kubara adoption/governance flow; retain source repo and link stable user docs. |
| [confighub/kubara-confighub-argo3@e5bd66a4](https://github.com/confighub/kubara-confighub-argo3/tree/e5bd66a43946b60f41fbff3f5ba530a73d8b430f/) | Legacy bridge-worker walkthrough appears superseded by newer integration. |
| [confighub/schema-catalog@0805fb32](https://github.com/confighub/schema-catalog/tree/0805fb3286b7e47b8ac6f612dd1466c525312b15/) | Machine-readable supplemental schema corpus. |
| [confighub/sdk@648a6e84](https://github.com/confighub/sdk/tree/648a6e842ba7035cfe3f99dc2ff65e3893434324/) | Large SDK/CLI monorepo; examples primarily developer docs and tests. |
| [confighub/sveltos-confighub@1a6958a7](https://github.com/confighub/sveltos-confighub/tree/1a6958a7347638960bb779b71bbdcfb183ca51d9/) | Strong multi-cluster governance story with offline verify and receipts; maintain in place with caveats. |
| [confighubai/ai-iac-ex@8d1e6749](https://github.com/confighubai/ai-iac-ex/tree/8d1e6749ec9f4190b5c602cadfac306a70abfee9/) | Historical model-output evaluation; no current ConfigHub user journey selected. |
| [confighubai/confighub-patterns@19c98b6f](https://github.com/confighubai/confighub-patterns/tree/19c98b6f158e8d3b84f33a582aa21702cf526db1/) | Pattern/schema corpus; reference data rather than a runnable user example. |
| [confighubai/cub-scout-ui@3879f156](https://github.com/confighubai/cub-scout-ui/tree/3879f15638ccd206ba9ddcaf3461a95036a279ea/) | UI reference; keep product-specific examples with the UI. |
