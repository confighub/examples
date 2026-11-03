# Workload Manager (moved)

`cub-workload` is now `cub manage workload`, the workload readiness command group of
[cub-manage](https://github.com/confighub/cub-manage). cub-manage is one `cub` plugin that
manages Kubernetes configuration across a ConfigHub fleet; each of the manager tools that used to
be here is one of its command groups.

| Before                              | Now                               |
| ----------------------------------- | --------------------------------- |
| `cub-workload <command>`            | `cub manage workload <command>`   |
| `cub-workload preflight`            | `cub manage preflight`            |
| `cub-workload version`              | `cub manage version`              |
| `cub-workload guardrails status`    | `cub manage guardrails status`    |
| `cub-workload profile <subcommand>` | `cub manage profile <subcommand>` |

The code, the agent skills and the design are in the
[cub-manage repository](https://github.com/confighub/cub-manage).
