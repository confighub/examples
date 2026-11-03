# Autoscale Manager (moved)

`cub-autoscale` is now `cub manage autoscale`, the autoscaling command group of
[cub-manage](https://github.com/confighub/cub-manage). cub-manage is one `cub` plugin that
manages Kubernetes configuration across a ConfigHub fleet; each of the manager tools that used to
be here is one of its command groups.

| Before                               | Now                               |
| ------------------------------------ | --------------------------------- |
| `cub-autoscale <command>`            | `cub manage autoscale <command>`  |
| `cub-autoscale preflight`            | `cub manage preflight`            |
| `cub-autoscale version`              | `cub manage version`              |
| `cub-autoscale guardrails status`    | `cub manage guardrails status`    |
| `cub-autoscale profile <subcommand>` | `cub manage profile <subcommand>` |

The code, the agent skills and the design are in the
[cub-manage repository](https://github.com/confighub/cub-manage).
