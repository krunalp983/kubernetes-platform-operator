# Architecture Notes

## Custom Resource

`ApplicationEnvironment` represents desired application runtime settings:

- image
- replica count
- container port
- environment variables
- resource requests and limits

## Reconciliation Contract

- environment classification (`dev`, `staging`, `prod`)
- image repository, tag, and pull policy
- service exposure type and service port
- replica count and optional autoscaling policy
- non-secret configuration map entries
3. Set owner references for garbage collection.
4. Update status conditions and observed generation.
5. Emit Kubernetes Events for success/failure.

The reconcile loop is idempotent and safe to retry.

1. Reconcile a ConfigMap `<name>-config` from `spec.config`.
2. Reconcile a Service `<name>-svc` from `spec.service`.
3. Reconcile a Deployment `<name>` from desired image/runtime settings.
4. Reconcile an HPA when autoscaling is enabled, or remove it when disabled.
5. Set owner references for garbage collection.
6. Update status conditions and observed generation.
7. Emit Kubernetes Events for success/failure.

Status condition semantics:

- `Ready=True`: managed Deployment is ready at desired replica count.
- `Progressing=True`: reconciliation is converging to desired state.
- `Degraded=True`: reconciliation failure occurred and requires attention.

The reconcile loop is idempotent and safe to retry.

## Managed Resources

- Deployment
- Service
- ConfigMap
- HorizontalPodAutoscaler (optional)
- Events and Status Conditions

## Operator Runtime

- Leader election enabled for HA controller behavior.
- Liveness/readiness probes exposed on `/healthz` and `/readyz`.
- Prometheus-compatible metrics exposed on port `8080`.

## Security Model

- Operator runs with dedicated ServiceAccount.
- ClusterRole scoped to required resources/verbs only.
- Container hardened with non-root runtime and minimal privileges.
- NetworkPolicy limits default egress to DNS and Kubernetes API.

## GitOps Model

- Argo CD AppProject scopes allowed sources, destinations, and resource kinds.
- Argo CD Application deploys the Helm chart from Git.
- Sync policy enables auto-sync, prune, self-heal, and safe sync options.

## CI Model

- Workflow linting and static checks
- Race-enabled unit tests and integration test scaffold execution
- Kubernetes manifest and Helm validation
- Vulnerability scanning for source and built image
- Release flow publishes versioned multi-arch image artifacts
