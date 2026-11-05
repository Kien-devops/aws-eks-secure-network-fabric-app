# Agent Directives: AWS EKS Secure Network Fabric

## 1. Phạm vi & Mục tiêu
- **Dự án:** `aws-eks-secure-network-fabric`
- **Region mục tiêu:** `ap-southeast-1` (Singapore), triển khai multi-AZ (`ap-southeast-1a`, `ap-southeast-1b`).
- **Phân tách 2 Repo:**
  - `aws-eks-secure-network-fabric`: Hạ tầng mạng AWS, Terraform, TGW Route Domains, GWLB + Suricata, AWS Network Firewall, EKS Clusters, GameDay Scenarios.
  - `aws-eks-secure-network-fabric-app`: Microservices (`frontend`, `auth-service`, `order-service`, `inventory-service`, `payment-worker`), Argo CD manifests, Kustomize overlays, Kubernetes NetworkPolicy.

---

## 2. Bắt buộc Sử dụng AWS MCP khi Triển khai hoặc Truy vấn
- **Nguyên tắc bắt buộc:**
  - Bất cứ khi nào nhận yêu cầu triển khai, kiểm tra, tạo, sửa hoặc dọn dẹp tài nguyên AWS, Agent **BẮT BUỘC PHẢI SỬ DỤNG AWS MCP** (`aws-api-mcp` -> `call_aws`).
  - Đối với các thao tác trong Kubernetes/EKS: Sử dụng `kubernetes-mcp-server`.
  - Không viết terminal shell hay script python boto3 thay thế cho MCP.
  - Khi người dùng bảo "triển khai X", Agent tự giác thực thi ngay qua AWS MCP mà không cần hỏi lại câu hỏi xác nhận thừa thãi.

---

## 3. Tích hợp Codebase Memory & Code Review Graph
- **Codebase Memory MCP (`codebase-memory-mcp`):**
  - Indexing: `index_repository(repo_path="...", persistence=true)`.
  - Tra cứu cấu trúc & phân tích: `get_architecture`, `search_graph`, `query_graph`.
- **Code Review Graph MCP (`code-review-graph`):**
  - Build & update graph: `build_or_update_graph_tool(repo_root="...")`.
  - Phân tích rủi ro & bán kính ảnh hưởng: `get_impact_radius_tool`.

---

## 4. An toàn & Bảo mật
- **Không giả mạo kết quả (Zero Fake Data):** Bằng chứng kiểm thử phải lấy từ raw runtime logs/output thực tế kèm timestamp.
- **Redaction:** Che giấu AWS Account ID, token bí mật, password trước khi lưu vào git repository.
- **FinOps:** Hủy bỏ tài nguyên theo quy trình Reverse Cleanup đảm bảo $0.00 lingering cost sau bài lab.
