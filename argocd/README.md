# Argo CD GitOps Manifests

This directory contains the Argo CD AppProject and Application for deploying
`kubernetes-platform-operator` using GitOps.

## Bootstrap

Before applying, replace these placeholders in the Argo CD manifests:

- `<PERSONAL_GITHUB_USERNAME>`
- `<PERSONAL_GITHUB_REPOSITORY>`

Apply both manifests through kustomize:

```bash
kubectl apply -k argocd
```

## What Is Managed

- Helm chart path: `charts/kubernetes-platform-operator`
- Target namespace: `kubernetes-platform-operator-system`
- Sync policy: automated (prune + self-heal)

## Operational Notes

- Pin immutable image tags in Application Helm values for production.
- Use pull requests to update `targetRevision` or Helm values.
- Rollback by reverting Git commits and letting Argo CD sync.
