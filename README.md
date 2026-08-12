# Platform Operator Showcase

[![CI](https://github.com/krunalp983/kubernetes-platform-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/krunalp983/kubernetes-platform-operator/actions/workflows/ci.yaml)
[![Release Image](https://github.com/krunalp983/kubernetes-platform-operator/actions/workflows/release-image.yaml/badge.svg)](https://github.com/krunalp983/kubernetes-platform-operator/actions/workflows/release-image.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Production-grade Platform Engineering repository implementing a Kubernetes Operator for a custom `ApplicationEnvironment` API, with GitOps deployment, Helm packaging, CI validation, and security scanning.

## Architecture

```text
Developer
   |
   v
GitHub Repository
   |
   +--> GitHub Actions (lint, test, build, validate, scan)
   |
   v
Helm Chart + Container Image
   |
   v
Argo CD Application
   |
   v
Kubernetes Cluster
   |
   v
Platform Operator
   |
   v
ApplicationEnvironment Custom Resources
   |
   v
Managed Deployments + Services + ConfigMaps + HPAs
```

## Repository Structure

```text
api/v1alpha1/                 API types for ApplicationEnvironment
cmd/operator/                 Operator entrypoint
internal/controller/          Reconciliation logic and unit tests
config/                       Kubernetes install manifests and CRD
charts/kubernetes-platform-operator/     Production Helm chart
argocd/                       GitOps Application and AppProject
examples/                     Sample custom resource
.github/workflows/            CI and release pipelines
docs/                         Operational and architecture docs
```

## What This Project Demonstrates

- Kubernetes Operator with controller-runtime
- CustomResourceDefinition with schema validation and status subresource
- Idempotent reconciliation of Deployment, Service, ConfigMap, and optional HPA
- Status conditions and Kubernetes Events for observability
- Secure deployment manifests with least-privilege RBAC
- Hardened multi-stage container image
- Helm chart for install and upgrades
- Argo CD GitOps configuration
- GitHub Actions CI/CD with test, validation, and vulnerability scanning

## Local Validation

```bash
go test ./...
go build ./cmd/operator
kubectl kustomize config >/tmp/kubernetes-platform-operator-install.yaml
helm lint charts/kubernetes-platform-operator
helm template kubernetes-platform-operator charts/kubernetes-platform-operator --namespace kubernetes-platform-operator-system
```

## Install with Helm

```bash
helm upgrade --install kubernetes-platform-operator charts/kubernetes-platform-operator \
  --namespace kubernetes-platform-operator-system \
  --create-namespace
```

## Install with Raw Manifests

```bash
kubectl apply -k config
```

## Usage

Apply a custom resource:

```bash
kubectl apply -f examples/applicationenvironment-sample.yaml
```

Check status:

```bash
kubectl get applicationenvironments
kubectl describe applicationenvironment payments-dev
kubectl get deployment payments-dev
kubectl get configmap payments-dev-config -o yaml
```

## Observability

- Logs: reconciliation start/end, errors, and events
- Metrics endpoint: `:8080` (controller-runtime metrics)
- Probes: health at `/healthz`, readiness at `/readyz`
- CR status: `status.conditions` (`Ready`, `Progressing`, `Degraded`), `readyReplicas`, `observedGeneration`

## CI/CD

`CI` workflow performs:

- workflow linting, formatting check, vet, race-enabled unit tests, binary build
- integration test stage scaffold
- kustomize and Helm render validation
- kubeconform schema checks
- govulncheck vulnerability scan
- Trivy filesystem and image scan

`Release Image` workflow builds and publishes multi-arch images to GHCR on version tags.

## Argo CD GitOps

Argo CD manifests:

- `argocd/project-kubernetes-platform-operator.yaml`
- `argocd/application-kubernetes-platform-operator.yaml`
- `argocd/kustomization.yaml`

Apply in Argo CD namespace:

```bash
kubectl apply -k argocd
```

Before applying Argo CD manifests, replace placeholders in repository and image references:

- `<PERSONAL_GITHUB_USERNAME>`
- `<PERSONAL_GITHUB_REPOSITORY>`

Files that include placeholders:

- `argocd/application-kubernetes-platform-operator.yaml`
- `argocd/project-kubernetes-platform-operator.yaml`
- `charts/kubernetes-platform-operator/values.yaml`
- `config/manager/deployment.yaml`
- `Makefile`

## Upgrade Strategy

- Prefer Helm-based upgrades with explicit image tags
- Keep CRD backward compatible for `v1alpha1` resources
- Roll updates using deployment strategy and leader election
- Validate rendering (`helm lint` and `helm template`) before sync

## Rollback Strategy

- Helm rollback:

```bash
helm rollback kubernetes-platform-operator <REVISION> -n kubernetes-platform-operator-system
```

- GitOps rollback: revert the Git commit and let Argo CD sync
- Validate rollback outcome via operator pod readiness and CR status conditions

## Troubleshooting

- Operator not starting:
  - Verify image/tag and pod security constraints
  - Check RBAC binding and service account
- CR not reconciling:
  - Check operator logs and Events on the custom resource
  - Confirm Deployment and ConfigMap ownership references
- Argo CD out of sync:
  - Inspect application health and sync operation logs

See `docs/operations.md` for an operational runbook.

See `docs/production-readiness.md` for the final readiness checklist and known risks.

## License

This project is licensed under the MIT License. See `LICENSE`.
