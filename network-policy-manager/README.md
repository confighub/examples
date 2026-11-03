# NetworkPolicy Manager (moved)

`cub-netpol` is now `cub manage netpol`, the NetworkPolicy command group of
[cub-manage](https://github.com/confighub/cub-manage). cub-manage is one `cub` plugin that
manages Kubernetes configuration across a ConfigHub fleet; each of the manager tools that used to
be here is one of its command groups.

| Before                         | Now                            |
| ------------------------------ | ------------------------------ |
| `cub-netpol <command>`         | `cub manage netpol <command>`  |
| `cub-netpol preflight`         | `cub manage preflight`         |
| `cub-netpol version`           | `cub manage version`           |
| `cub-netpol guardrails status` | `cub manage guardrails status` |

The code, the agent skills and the design are in the
[cub-manage repository](https://github.com/confighub/cub-manage).
