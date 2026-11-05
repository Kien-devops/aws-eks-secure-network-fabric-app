# AWS EKS Secure Network Fabric (Workloads & GitOps)

Application microservices and GitOps deployment configurations for Edge and Core EKS clusters.

## Microservices Topology
- **Edge EKS (Public/Ingress boundary):**
  - \rontend\: Next.js / Web UI facing ALB + AWS WAF
  - \uth-service\: User authentication & JWT validation
- **Core EKS (Private data boundary):**
  - \order-service\: Order processing & business transactions
  - \inventory-service\: Stock management & Redis cache
  - \payment-worker\: SQS consumer & asynchronous transaction settlement

## Structure
- \services/\: Source code and container definitions for Edge & Core services
- \gitops/\: Argo CD application manifests, Helm charts, and Kustomize overlays
  - \edge/\: Manifests targeting Edge EKS cluster
  - \core/\: Manifests targeting Core EKS cluster
- \policies/\: Kubernetes NetworkPolicy specifications (default deny, explicit inter-service egress/ingress)
