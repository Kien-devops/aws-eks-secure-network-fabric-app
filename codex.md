# Codex AI Agent Guidelines & Context Automation (AWS EKS Secure Network Fabric)

## 1. Automatic Session Context Retrieval & Project Standards
- **Mục tiêu dự án:** Xây dựng kiến trúc AWS Advanced Networking & Security trên EKS (`aws-eks-secure-network-fabric`).
- **Môi trường & Vùng mặc định:** **`ap-southeast-1` (Singapore)**.
- **Prefix chuẩn:** `fabric-prod-*` hoặc `fabric-dev-*`.
- **Nguyên tắc định tuyến cốt lõi:**
  - East-West: `Edge EKS -> TGW -> Inspection VPC (GWLB + Suricata EC2) -> TGW -> Core EKS`.
  - Outbound Egress: `VPCs -> TGW -> Centralized Egress VPC (AWS Network Firewall -> NAT GW -> IGW)`.
  - TGW Appliance Mode: **BẬT** trên Inspection Attachment để bảo đảm Symmetric Routing.
  - Không chain GWLB và Network Firewall trên cùng 1 flow trong MVP.

---

## 2. Bắt buộc Tự động Sử dụng Danh mục MCP Tools Đã Cấp Quyền (Auto-Invoke)

Tất cả các MCP Tools sau đây đã ở trạng thái **Allow** và phải được tự động kích hoạt:

### 2.1 AWS Operations (`aws-api-mcp` & `aws-mcp`)
- **`aws-api-mcp/call_aws`:** Bắt buộc tự động dùng cho 100% thao tác tương tác AWS Cloud (Describe, Create, Update, Delete, Verify status). Luôn chỉ định `--region ap-southeast-1`.
- **`aws-api-mcp/suggest_aws_commands`:** Tự động dùng khi cần đề xuất cú pháp CLI phức tạp.
- **`aws-mcp/aws__run_script`:** Tự động dùng khi cần chạy script tự động hóa AWS.
- **`aws-mcp/aws__search_documentation`:** Tự động dùng khi cần tra cứu tài liệu AWS chính thống.
- **`kubernetes-mcp-server`:** Tự động dùng cho các tài nguyên Pods, Deployments, Services, NetworkPolicies trên 2 EKS clusters.

> **Cấm:** Không tự ý viết script Python `boto3` hay terminal shell (`run_command`) khi MCP thực hiện được.

### 2.2 Phân tích Code & Đồ thị (`code-review-graph` & `codebase-memory-mcp`)
- **`code-review-graph/get_architecture_overview_tool`:** Tự động gọi khi cần bức tranh kiến trúc tổng thể.
- **`code-review-graph/query_graph_tool`:** Tự động gọi khi truy vấn phụ thuộc giữa các thành phần mạng & manifests.
- **`code-review-graph/list_graph_stats_tool`:** Tự động gọi khi kiểm tra stats và số lượng nodes/edges.
- **`code-review-graph/find_large_functions_tool`:** Tự động gọi để rà soát các hàm phức tạp hoặc cấu hình quá dài.
- **`codebase-memory-mcp`:** Tự động cập nhật bằng `index_repository(repo_path="...", persistence=true)` và tra cứu bằng `get_architecture`, `search_graph`, `trace_path`.

---

## 3. Quy chuẩn Thực thi & Evidence
- **Console-First to IaC:** Hiểu rõ flow trên Console -> Triển khai và xác thực bằng AWS MCP -> Chuyển giao vào Terraform/ArgoCD.
- **Redaction 100%:** Redact toàn bộ Account ID, Access Key, Database Password trước khi commit evidence.
- **Reverse Teardown:** Dọn dẹp tài nguyên theo thứ tự ngược lại để đảm bảo không kẹt ENI hay phát sinh chi phí ngầm.

---

## 4. Bỏ qua (Ignore) Các Skill Thừa Thãi
- Không kích hoạt các third-party agent skills (như Bug Bounty, Pentest, Marketing, SEO...). Tập trung 100% vào core AWS Networking & MCP tools.
