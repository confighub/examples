# Observability Manager (moved)

`cub-observability` is now `cub manage observability`, the observability command group of
[cub-manage](https://github.com/confighub/cub-manage). cub-manage is one `cub` plugin that
manages Kubernetes configuration across a ConfigHub fleet; each of the manager tools that used to
be here is one of its command groups.

| Before                                   | Now                                  |
| ---------------------------------------- | ------------------------------------ |
| `cub-observability <command>`            | `cub manage observability <command>` |
| `cub-observability preflight`            | `cub manage preflight`               |
| `cub-observability version`              | `cub manage version`                 |
| `cub-observability guardrails status`    | `cub manage guardrails status`       |
| `cub-observability profile <subcommand>` | `cub manage profile <subcommand>`    |

The code, the agent skills and the design are in the
[cub-manage repository](https://github.com/confighub/cub-manage).
