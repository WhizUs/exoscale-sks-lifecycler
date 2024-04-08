# exoscale-sks-lifecycler

## Getting Started

Create the following environment variables:
```bash
export EXOSCALE_API_KEY=...
export EXOSCALE_API_SECRET=...
export EXOSCALE_API_ZONE=de-muc-1

export KUBECONFIG=/path/to/kubeconfig.yaml

export EXOSCALE_SKS_LIFECYCLER_DESIRED_K8S_VERSION=v1.28.7
export EXOSCALE_SKS_LIFECYCLER_SKS_CLUSTER_ID=905ff...
export EXOSCALE_SKS_LIFECYCLER_WAIT_FOR_PODS_LABEL_SELECTOR="app in (frontend,backend)"
```
