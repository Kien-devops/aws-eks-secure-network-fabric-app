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

## 2. AWS MCP-First Directive (Bắt buộc 100%)
- **Quy tắc tuyệt đối:**
  - Mọi thao tác tương tác AWS Cloud (Describe, Create, Update, Delete, Verify status) **BẮT BUỘC ƯU TIÊN SỬ DỤNG AWS MCP SERVER**.
  - Server chính: `aws-api-mcp` với công cụ `call_aws`:
    ```json
    {
      "ServerName": "aws-api-mcp",
      "ToolName": "call_aws",
      "Arguments": {
        "cli_command": "aws <service> <operation> [parameters] --region ap-southeast-1"
      }
    }
    ```
  - Tra cứu tài liệu và regional check: dùng server `aws-mcp`.
  - Tương tác với Kubernetes/EKS: dùng server `kubernetes-mcp-server`.
- **Nghiêm cấm:** Không tự ý viết script Python `boto3` hay terminal shell (`run_command`) khi MCP thực hiện được.
- **Hành động tự động dứt khoát:** Khi có yêu cầu triển khai hoặc kiểm tra, thực thi ngay qua AWS MCP, không xin phép rườm rà.

---

## 3. Quản lý Đồ thị Mã nguồn & Bộ nhớ Codebase
- Dự án sử dụng song song 2 MCP servers:
  1. `codebase-memory-mcp`: Dùng để duy trì đồ thị tri thức, tìm kiếm kiến trúc (`get_architecture`), truy vấn đường đi (`trace_path`).
  2. `code-review-graph`: Dùng để xác định phạm vi ảnh hưởng (`get_impact_radius_tool`), trích xuất context review (`get_review_context_tool`).
- Mỗi khi hoàn thành một module hạ tầng hoặc cập nhật service, chủ động gọi `index_repository` và `build_or_update_graph_tool` để đồng bộ đồ thị tri thức.

---

## 4. Quy chuẩn Thực thi & Evidence
- **Console-First to IaC:** Hiểu rõ flow trên Console -> Triển khai và xác thực bằng AWS MCP -> Chuyển giao vào Terraform/ArgoCD.
- **Redaction 100%:** Redact toàn bộ Account ID, Access Key, Database Password trước khi commit evidence.
- **Reverse Teardown:** Dọn dẹp tài nguyên theo thứ tự ngược lại để đảm bảo không kẹt ENI hay phát sinh chi phí ngầm.
