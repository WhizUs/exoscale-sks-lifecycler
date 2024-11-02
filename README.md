# exoscale-sks-lifecycler

Lifecycle automation of Exoscale SKS cluster nodes.

**Current capabilities:**
- evict nodes, which are not on the desired version
- evict nodes, which are defined via a labelSelector

**Quality-of-life features:**
- before a node is evicted, pods are re-scheduled (in order to maintain high availability for applications):
  - a pod managed by a `Deployment` has its rollout restarted, in order to cause no downtime for **single-replica deployments**
  - a pod managed by a `DaemonSet` is not evicted
  - a pod without an ownerReference is directly evicted

## Getting Started

### Configuration

Configuration is done via environment variables.

#### Required configuration

```
export EXOSCALE_API_KEY=EXO...
export EXOSCALE_API_SECRET=cIzK1...
export EXOSCALE_API_ZONE=at-vie-1
export KUBECONFIG=/path/to/kubeconfig.yaml

export EXOSCALE_SKS_LIFECYCLER_DESIRED_K8S_VERSION=v1.28.7
export EXOSCALE_SKS_LIFECYCLER_SKS_CLUSTER_ID=905ff...
```

#### Optional configuration

`EXOSCALE_SKS_LIFECYCLER_EVICT_NODES_LABELSELECTOR` lets you define nodes, which you want to evict from the cluster, via a labelSelector.
```
export EXOSCALE_SKS_LIFECYCLER_EVICT_NODES_LABELSELECTOR="node.kubernetes.io/instance-type=cpu.extra-large,key2=val2"
```

### Run

> The program loops over all nodes in the cluster, and then exits!

Run directly via `go` from the project's root directory:

```bash
go run main.go nodepool cycle
```

Or use the container image [ghcr.io/whizus/exoscale-sks-lifecycler](https://github.com/WhizUs/exoscale-sks-lifecycler/pkgs/container/exoscale-sks-lifecycler).

## Development

The application is written in Go and uses [spf13/cobra](https://github.com/spf13/cobra) and [spf13/viper](https://github.com/spf13/viper) libraries to provide CLI functionality and simple configuration.

To get started with development, see [CONTRIBUTING.md](CONTRIBUTING.md).
