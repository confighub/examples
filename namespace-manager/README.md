# Namespace Manager (moved)

`cub-namespace` is now `cub manage namespace`, the namespaces command group of
[cub-manage](https://github.com/confighub/cub-manage). cub-manage is one `cub` plugin that
manages Kubernetes configuration across a ConfigHub fleet; each of the manager tools that used to
be here is one of its command groups.

| Before                            | Now                              |
| --------------------------------- | -------------------------------- |
| `cub-namespace <command>`         | `cub manage namespace <command>` |
| `cub-namespace preflight`         | `cub manage preflight`           |
| `cub-namespace version`           | `cub manage version`             |
| `cub-namespace guardrails status` | `cub manage guardrails status`   |

The code, the agent skills and the design are in the
[cub-manage repository](https://github.com/confighub/cub-manage).
