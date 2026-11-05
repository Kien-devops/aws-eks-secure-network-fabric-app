# Agent Directives: AWS EKS Secure Network Fabric

## 1. Phạm vi & Mục tiêu
- **Dự án:** `aws-eks-secure-network-fabric`
- **Region mục tiêu:** `ap-southeast-1` (Singapore), triển khai multi-AZ (`ap-southeast-1a`, `ap-southeast-1b`).
- **Phân tách 2 Repo:**
  - `aws-eks-secure-network-fabric`: Hạ tầng mạng AWS, Terraform, TGW Route Domains, GWLB + Suricata, AWS Network Firewall, EKS Clusters, GameDay Scenarios.
  - `aws-eks-secure-network-fabric-app`: Microservices (`frontend`, `auth-service`, `order-service`, `inventory-service`, `payment-worker`), Argo CD manifests, Kustomize overlays, Kubernetes NetworkPolicy.

---

## 2. Bắt buộc Tự động Sử dụng Danh mục MCP Tools (Đã Cấp Quyền "Allow")

Agent phải tự động ưu tiên và kích hoạt ngay các công cụ MCP dưới đây:

### 2.1 Nhóm AWS Cloud
- **`aws-api-mcp/call_aws`:** Bắt buộc 100% khi thao tác với AWS Cloud (describe, create, update, delete, verify).
- **`aws-api-mcp/suggest_aws_commands`:** Bắt buộc khi phân tích, tối ưu cú pháp lệnh AWS CLI.
- **`aws-mcp/aws__run_script`:** Bắt buộc khi chạy script automation AWS.
- **`aws-mcp/aws__search_documentation`:** Bắt buộc khi tra cứu tài liệu AWS.
- **`kubernetes-mcp-server`:** Bắt buộc khi quản lý Kubernetes objects trên EKS.

### 2.2 Nhóm Code Review Graph & Codebase Memory
- **`code-review-graph/get_architecture_overview_tool`:** Bắt buộc khi truy vấn kiến trúc tổng quan.
- **`code-review-graph/query_graph_tool`:** Bắt buộc khi truy vấn quan hệ module/file/flow.
- **`code-review-graph/list_graph_stats_tool`:** Bắt buộc khi kiểm tra số lượng node/edge đồ thị mã nguồn.
- **`code-review-graph/find_large_functions_tool`:** Bắt buộc khi rà soát hàm/khối lệnh phức tạp.
- **`codebase-memory-mcp`:** Bắt buộc duy trì index và truy vấn kiến trúc sâu qua `index_repository`, `get_architecture`, `search_graph`, `trace_path`.

---

## 3. Hành động Tự chủ & Dứt khoát (Autonomous Action)
- Khi nhận lệnh từ người dùng: Tự giác thực thi ngay qua MCP tương ứng mà không hỏi xin phép lại.
- Chỉ dùng `run_command` cho local git và packaging.

---

## 4. An toàn & Bảo mật
- **Zero Fake Data:** Bằng chứng kiểm thử phải lấy từ raw runtime logs/output thực tế kèm timestamp.
- **Redaction:** Che giấu AWS Account ID, token bí mật, password trước khi lưu vào git repository.
- **FinOps:** Hủy bỏ tài nguyên theo quy trình Reverse Cleanup đảm bảo $0.00 lingering cost sau bài lab.

---

## 5. Bỏ qua (Ignore) Toàn bộ Third-Party Skills Ngoài Luồng
- Bỏ qua hoàn toàn các third-party agent skills (như Pentest, Bug Bounty, SEO, Marketing...).
- Chỉ tập trung vào các MCP tools đã được phê duyệt và giải pháp hạ tầng AWS Network Fabric theo `spec.md`.
