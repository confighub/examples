# RBAC Manager for Agents (moved)

`cub-rbac` is now `cub manage rbac`, the Kubernetes RBAC command group of
[cub-manage](https://github.com/confighub/cub-manage). cub-manage is one `cub` plugin that
manages Kubernetes configuration across a ConfigHub fleet; each of the manager tools that used to
be here is one of its command groups.

| Before                       | Now                            |
| ---------------------------- | ------------------------------ |
| `cub-rbac <command>`         | `cub manage rbac <command>`    |
| `cub-rbac preflight`         | `cub manage preflight`         |
| `cub-rbac version`           | `cub manage version`           |
| `cub-rbac guardrails status` | `cub manage guardrails status` |

The code, the agent skills and the design are in the
[cub-manage repository](https://github.com/confighub/cub-manage).
