# kubernetes-platform-operator Helm Chart

This chart deploys the Kubernetes Platform Operator and installs the required ApplicationEnvironment CRD.

## Install

```bash
helm upgrade --install kubernetes-platform-operator charts/kubernetes-platform-operator \
  --namespace kubernetes-platform-operator-system \
  --create-namespace
```

## Validate

```bash
helm lint charts/kubernetes-platform-operator
helm template kubernetes-platform-operator charts/kubernetes-platform-operator --namespace kubernetes-platform-operator-system
```

## Values of interest

- `image.repository`: controller image repository
- `image.tag`: controller image tag (defaults to chart appVersion)
- `replicaCount`: operator deployment replica count
- `resources`: pod requests and limits
- `leaderElection.enabled`: enable high-availability leader election
- `metricsService.enabled`: create a Service for metrics scraping
- `serviceMonitor.enabled`: create a Prometheus Operator ServiceMonitor
- `podDisruptionBudget.enabled`: keep operator available during node drains
- `topologySpreadConstraints`: optional anti-concentration scheduling rules
- `networkPolicy.enabled`: enforce egress-only policy for DNS and Kubernetes API access
