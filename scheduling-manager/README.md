# Scheduling Manager (moved)

`cub-scheduling` is now `cub manage scheduling`, the workload placement command group of
[cub-manage](https://github.com/confighub/cub-manage). cub-manage is one `cub` plugin that
manages Kubernetes configuration across a ConfigHub fleet; each of the manager tools that used to
be here is one of its command groups.

| Before                                | Now                               |
| ------------------------------------- | --------------------------------- |
| `cub-scheduling <command>`            | `cub manage scheduling <command>` |
| `cub-scheduling preflight`            | `cub manage preflight`            |
| `cub-scheduling version`              | `cub manage version`              |
| `cub-scheduling guardrails status`    | `cub manage guardrails status`    |
| `cub-scheduling profile <subcommand>` | `cub manage profile <subcommand>` |

The code, the agent skills and the design are in the
[cub-manage repository](https://github.com/confighub/cub-manage).
