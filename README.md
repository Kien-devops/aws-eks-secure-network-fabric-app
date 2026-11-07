# AWS EKS Secure Network Fabric — Workloads & GitOps

Repo hiện chỉ có tài liệu khởi tạo; **chưa có source, Dockerfile, GitOps manifests hoặc pipeline**. Nội dung dưới là contract dự kiến, chưa chứng minh ứng dụng đã triển khai. Lab Console nằm trong repo [hạ tầng](https://github.com/Kien-devops/aws-eks-secure-network-fabric/tree/main/labs).

| Cluster | Workload dự kiến | Contract cần triển khai |
|---|---|---|
| Edge `fabric-edge-eks` | frontend | HTTP 8080, `/health`; ALB HTTPS phía client; gọi Core qua CA nội bộ |
| Edge | auth-service | HTTP 8081, health và JWT contract rõ ràng |
| Core `fabric-core-eks` | order-service | TLS 8082, `/health`; NLB TCP 443 passthrough; gửi SQS bằng Pod Identity |
| Core | inventory-service | HTTP 8083; Redis TLS và Aurora theo contract quyền tối thiểu |
| Core | payment-worker | SQS consumer, idempotency, visibility timeout, retry/DLQ; health không giả định HTTP listener |

Namespace app: `fabric-apps`. DNS Core: `core.fabric.internal` trong private zone `fabric.internal`; certificate backend phải có SAN đúng hostname, frontend tin CA từ `/etc/fabric-ca/ca.crt`. NLB IP targets giữ `preserve_client_ip.enabled=false`; backend NetworkPolicy/SG dựa nguồn NLB theo lab 04.03. TLS bảo vệ truyền tải; MVP không có mTLS hoặc giải mã TLS trên Suricata.

Trước khi chạy lab 04.04, phải cung cấp từng artifact sau; thiếu mục nào thì giữ DRAFT:

1. Source/build contexts và Dockerfile cho từng service; cổng, probes, request/response, cấu hình lỗi/dependency được xác định. Payment xử lý message trùng và chỉ DeleteMessage sau commit thành công.
2. ECR images ghim digest cho từng service; test/scanning reports cùng digest, SBOM và chữ ký được verify. Chưa có admission policy tự chặn unsigned images.
3. Kustomize manifests Edge/Core render được, Service selectors/ports khớp, resources/probes, Pod Identity service accounts và NetworkPolicy tương ứng 03.05. Hai Argo CD quản lý cluster local riêng, app revision ghim commit khi bootstrap.
4. Migration có phiên bản, quyền riêng và backup/restore đã kiểm tra. Git revert không tự hoàn tác schema hoặc dữ liệu.
5. Secrets trong Secrets Manager; SDK hỗ trợ Pod Identity và refresh secret. ConfigMap chỉ chứa endpoint/tên cấu hình, không chứa password/token. CA public certificate có thể mount; private key không đưa vào Git.
6. Smoke tests qua ingress và Edge→Core→data/messaging, controls hai AZ, logs đã che dữ liệu và rollback về digest/commit được duyệt. Probe images dùng GameDay phải có ECR digest riêng, không giả định đã tồn tại.

Cấu trúc dự kiến `services/`, `gitops/edge/`, `gitops/core/`, `policies/` chỉ được tạo khi triển khai thực tế. Chỉnh tài liệu lần này không tạo scaffolding, không deploy hoặc publish.
