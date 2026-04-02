# Why `cub-proc`: Notes from `che/INSTALL.md`

This note captures the conclusion from reviewing the internal `examples-internal/che/INSTALL.md` guide and asks a narrower question:

Would `cub-proc` materially help this install flow?

Short answer:

- yes, this is a strong `cub-proc` candidate
- no, it should probably not be the first `cub-proc` profile
- the main value is bounded-procedure visibility, resumability, and evidence, not hiding the current `./ops` implementation

## What The Install Guide Already Shows

The CHE install flow is not one command. It is a real bounded procedure with:

- AWS authentication and bootstrap
- a temporary Kind-based bootstrap path for AWS infrastructure
- private-cluster access through Tailscale
- ACK controller bootstrap and pivot from Kind to EKS
- prerequisite secret creation in AWS Secrets Manager
- ConfigHub space cloning and unit customization
- grouped ConfigHub unit apply with readiness waits
- Keycloak setup
- ConfigHub worker deployment
- final URL and credential verification

That is exactly the kind of flow that currently lives across shell scripts, AWS state, Kubernetes state, ConfigHub state, worker state, controller readiness, and human memory.

## Why This Validates `cub-proc`

### 1. It needs an answer to "where am I?"

The operator must know which step is complete, which is waiting, and which system is now responsible.

Examples from the install flow:

- AWS bootstrap started but EKS not ready yet
- Tailscale instance created but subnet route not yet approved
- `traefik` ready but NLB hostname not yet captured into later unit config
- Keycloak reachable but ConfigHub not yet applied
- worker manifest applied but worker not yet connected

That is the core `cub-proc` value: one visible operation record instead of shell output plus memory.

### 2. It generates important values that should not live only in terminal scrollback

The guide explicitly says the operator must note and reuse values such as:

- subnet IDs
- security group ID
- OIDC ARN and issuer
- RDS endpoints
- NLB hostname
- service URLs

Those are exactly the kind of resolved outputs that belong in an `Operation` record.

### 3. It has real waits and assertions across multiple systems

This procedure does not just mutate state. It has meaningful post-mutation checks:

- AWS auth valid
- cluster reachable
- ACK controllers healthy
- required secrets exist
- Keycloak reachable
- ConfigHub reachable
- worker connected

Those checks are currently spread across commands, logs, UI pages, and operator judgment. A `cub-proc` record would make them explicit.

### 4. It has real interruption and handoff risk

This is the kind of install where one human starts it, another human or AI may continue it later, and both need to know:

- what already completed
- what values were resolved
- what is still waiting
- what has actually been proven

That is the exact handoff problem `cub-proc` is meant to solve.

## What `cub-proc` Would Help With Most

- tracking "where am I?" across the install
- capturing resolved values instead of relying on terminal memory
- making waits and assertions explicit
- supporting interruption and handoff cleanly

## What `cub-proc` Would Not Magically Fix

`cub-proc` would not remove the need for:

- AWS, Cloudflare, Tailscale, and ConfigHub prerequisites
- the underlying `./ops` commands
- ACK, ESO, Keycloak, and ingress-specific operational knowledge
- environment-specific values and decisions

The point is not to make the install trivial. The point is to give one consistent operational record for a non-trivial install.

## Suggested Procedure Shape

This looks like a strong candidate for either:

```text
che/install
```

or, probably better at first:

```text
che/bootstrap-infra
che/install-platform
che/install-confighub
```

The split form is safer because the current guide combines infrastructure bootstrap, platform bring-up, app install, and worker deployment in one long flow.

## Illustrative `Operation` Fragments

The syntax below is illustrative, not a finalized `cub-proc` spec.

### Early waiting point

````yaml
kind: Operation
procedure: che/install
procedureVersion: v1alpha1
state: waiting
currentStep: tailscale-connect

subjectBinding:
  space: che/acme
  env: acme

resolvedBindings:
  worker: acme-eks-worker
  target: acme-kubernetes
  clusterName: acme-demo

steps:
  - name: aws-login
    state: done
  - name: kind-bootstrap
    state: done
  - name: tailscale-connect
    state: waiting
    message: "Approve subnet route in Tailscale admin, then rerun watch."
  - name: bootstrap-eks
    state: pending
  - name: apply-units
    state: pending
  - name: deploy-worker
    state: pending

outputs:
  subnetIds:
    - subnet-0abc
    - subnet-0def
    - subnet-0123
  oidcIssuer: https://oidc.eks.us-west-2.amazonaws.com/id/EXAMPLE
  keycloakDbEndpoint: keycloak-postgres.abc123.us-west-2.rds.amazonaws.com
  confighubDbEndpoint: confighub-postgres.def456.us-west-2.rds.amazonaws.com
  nlbHostname: abc123.elb.us-west-2.amazonaws.com
  urls:
    keycloak: https://keycloak.acme.example.com
    confighub: https://hub.acme.example.com

assertions:
  - name: aws-auth-valid
    state: pass
  - name: cluster-reachable
    state: pending
  - name: ack-controllers-healthy
    state: pending
  - name: required-secrets-exist
    state: pending
  - name: keycloak-reachable
    state: pending
  - name: confighub-reachable
    state: pending
  - name: worker-connected
    state: pending
````

### Later in the run

````yaml
kind: Operation
procedure: che/install
procedureVersion: v1alpha1
state: running
currentStep: deploy-worker

subjectBinding:
  space: che/acme
  env: acme

resolvedBindings:
  worker: acme-eks-worker
  target: acme-kubernetes
  clusterName: acme-demo

steps:
  - name: aws-login
    state: done
  - name: kind-bootstrap
    state: done
  - name: tailscale-connect
    state: done
  - name: bootstrap-eks
    state: done
  - name: init-secrets
    state: done
  - name: apply-platform-foundations
    state: done
  - name: apply-keycloak
    state: done
  - name: apply-confighub
    state: done
  - name: deploy-worker
    state: running

outputs:
  subnetIds:
    - subnet-0abc
    - subnet-0def
    - subnet-0123
  keycloakDbEndpoint: keycloak-postgres.abc123.us-west-2.rds.amazonaws.com
  confighubDbEndpoint: confighub-postgres.def456.us-west-2.rds.amazonaws.com
  nlbHostname: abc123.elb.us-west-2.elb.amazonaws.com
  urls:
    keycloak: https://keycloak.acme.example.com
    confighub: https://hub.acme.example.com

assertions:
  - name: aws-auth-valid
    state: pass
  - name: cluster-reachable
    state: pass
  - name: ack-controllers-healthy
    state: pass
  - name: required-secrets-exist
    state: pass
  - name: keycloak-reachable
    state: pass
  - name: confighub-reachable
    state: pass
  - name: worker-connected
    state: pending
    message: "Worker manifest applied; waiting for first successful connection to ConfigHub."
````

These fragments illustrate the core distinction:

- steps can be done
- the procedure can still be waiting
- important assertions can still be pending

That is better than treating command completion as proof that the whole install is complete.

## Recommendation

This CHE install flow is good design evidence for `cub-proc`.

It should not block the current examples or become the first profile. It is better treated as:

- a strong later profile once the `cub-proc` model is proven on smaller public procedures
- a useful test case for interruption, resumability, and cross-system evidence

## Related Pages

- [README.md](./README.md)
- [procedure-candidates.md](./procedure-candidates.md)
- [03-cub-proc-prd.md](./03-cub-proc-prd.md)
- [03-cub-proc-rfc.md](./03-cub-proc-rfc.md)
