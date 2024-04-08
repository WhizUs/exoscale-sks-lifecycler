# exoscale-sks-lifecycler

Lifecycle automation of Exoscale SKS cluster nodes.

## Getting Started

### Configuration via Environment Variables

| Name                                      | Example Value                  |
|-------------------------------------------|--------------------------------|
| `EXOSCALE_API_KEY`                          | `EXO...`                         |
| `EXOSCALE_API_SECRET`                       | `cIzK1...`                       |
| `EXOSCALE_API_ZONE`                         | `at-vie-1`                       |
| `KUBECONFIG`                                | `/path/to/kubeconfig.yaml`       |
| `EXOSCALE_SKS_LIFECYCLER_DESIRED_K8S_VERSION` | `v1.28.7`                      |
| `EXOSCALE_SKS_LIFECYCLER_SKS_CLUSTER_ID`    | `905ff...`                       |
| `EXOSCALE_SKS_LIFECYCLER_WAIT_FOR_PODS_LABEL_SELECTOR` | `"app in (frontend,backend)"` |

### Run

Run directly via `go` from the project's root directory:

```bash
go run main.go nodepool cycle
```

Or use the container image [ghcr.io/whizus/exoscale-sks-lifecycler](https://github.com/WhizUs/exoscale-sks-lifecycler/pkgs/container/exoscale-sks-lifecycler).

## Development

The application is written in Go and uses [spf13/cobra](https://github.com/spf13/cobra) and [spf13/viper](https://github.com/spf13/viper) libraries to provide CLI functionality and simple configuration.

To get started with development, see [CONTRIBUTING.md](CONTRIBUTING.md).
