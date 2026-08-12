# Operations Runbook

## Day-2 Commands

```bash
kubectl -n kubernetes-platform-operator-system get deploy,pods
kubectl -n kubernetes-platform-operator-system logs deploy/kubernetes-platform-operator-kubernetes-platform-operator --tail=200
kubectl get applicationenvironments
kubectl describe applicationenvironment <name>
```

## Health Checks

```bash
kubectl -n kubernetes-platform-operator-system port-forward deploy/kubernetes-platform-operator-kubernetes-platform-operator 8080:8080 8081:8081
curl -fsS http://127.0.0.1:8081/healthz
curl -fsS http://127.0.0.1:8081/readyz
```

## ApplicationEnvironment Conditions

- `Ready=True`: desired replicas are reported ready by the managed Deployment.
- `Progressing=True`: reconciliation is still converging to desired state.
- `Degraded=True`: reconciliation detected an error and reported failure details in condition messages.

Inspect conditions quickly:

```bash
kubectl get applicationenvironment -A
kubectl describe applicationenvironment <name>
```

## Common Failure Scenarios

1. RBAC denied errors
- Symptom: reconcile errors on create/update of Deployment or ConfigMap
- Action: validate ClusterRole and ClusterRoleBinding names from chart release

2. Invalid custom resource spec
- Symptom: API server rejects CR create/update
- Action: inspect CRD schema and resource spec fields

3. Image pull failures
- Symptom: operator pod in ImagePullBackOff
- Action: verify image repository, tag, and imagePullSecrets

## Upgrade Procedure

1. Update image tag in chart values or release values file.
2. Run CI validation (tests, Helm lint/template, kubeconform).
3. Deploy using Helm or GitOps sync.
4. Confirm operator pod readiness and reconcile behavior.
5. Confirm existing CR instances report healthy status conditions.

## Rollback Procedure

1. Identify last known-good Helm release revision.
2. Run rollback.
3. Verify deployment availability and controller logs.
4. Validate that existing ApplicationEnvironment instances still reconcile correctly.

## Security Review Checklist

- Non-root runtime user enabled
- Privilege escalation disabled
- Read-only root filesystem enabled
- Linux capabilities dropped
- Least-privilege RBAC verified
- Egress NetworkPolicy allows only DNS and Kubernetes API ports by default
- Trivy and govulncheck clean in CI
