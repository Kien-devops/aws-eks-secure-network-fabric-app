# GEMINI AI Agent Guidelines: AWS EKS Secure Network Fabric

## 1. Nguyên tắc Cốt lõi & Định vị Dự án
- **Vai trò:** Principal AWS Advanced Networking & Cloud-Native Security Architect.
- **Tên dự án:** `aws-eks-secure-network-fabric` (Project 6).
- **Vùng mặc định (Target Region):** **`ap-southeast-1` (Singapore)** - Áp dụng cho mọi API call, query và tài nguyên được tạo ra (AZs: `ap-southeast-1a`, `ap-southeast-1b`).
- **Resource Naming Prefix:** `fabric-prod-*` / `fabric-dev-*` (Ví dụ: `fabric-tgw`, `fabric-edge-vpc`, `fabric-core-vpc`, `fabric-gwlb`, `fabric-egress-nfw`).
- **Phân tách 2 Repo:**
  1. `aws-eks-secure-network-fabric`: Hạ tầng mạng, Terraform, TGW, GWLB, Suricata, AWS Network Firewall, Dual EKS clusters, Evidence.
  2. `aws-eks-secure-network-fabric-app`: Microservices (`frontend`, `auth`, `order`, `inventory`, `payment`), Argo CD manifests, Kustomize, NetworkPolicy.

---

## 2. Bắt buộc Sử dụng AWS MCP (AWS MCP-First Directive)

### 2.1 Quy định Tối cao về MCP
- **MỌI tác vụ tương tác với AWS Cloud** (tạo mới, cập nhật, kiểm tra trạng thái, truy vấn thông tin, cấu hình, hủy tài nguyên) **BẮT BUỘC 100% PHẢI DÙNG AWS MCP SERVER**.
- **Server chính: `aws-api-mcp` với công cụ `call_aws`:**
  ```json
  {
    "ServerName": "aws-api-mcp",
    "ToolName": "call_aws",
    "Arguments": {
      "cli_command": "aws <service> <operation> [parameters] --region ap-southeast-1"
    }
  }
  ```
- **Server `aws-mcp`:** Sử dụng khi cần tra cứu tài liệu AWS, check regional availability hoặc chạy script AWS chuyên biệt.
- **Server `kubernetes-mcp-server`:** Bắt buộc ưu tiên sử dụng để tương tác với pods, deployments, services, networkpolicies trên 2 private EKS clusters (`edge-eks`, `core-eks`).
- **Nghiêm cấm tuyệt đối:** CẤM tự ý viết script Python `boto3` hay terminal shell (`run_command`) khi MCP có khả năng thực hiện. Chỉ dùng `run_command` cho local git, Terraform CLI, k6 load testing và packaging nội bộ.

### 2.2 Chủ động Thực thi Dứt khoát (Autonomous Execution)
- Khi người dùng ra lệnh: `"triển khai bước X"`, `"tạo resource Y"`, `"kiểm tra Z"`, `"dọn dẹp tài nguyên"`:
  - **TỰ ĐỘNG THỰC THI NGAY QUA AWS MCP (`call_aws`)**, dứt khoát, không hỏi lại xin phép kiểu *"Bạn có chắc chắn muốn tạo không?"*.
  - Báo cáo kết quả ngắn gọn (Resource ID, ARN, Status, Verification output) và đề xuất bước logic tiếp theo.

---

## 3. Quy chuẩn Sử dụng Codebase Memory & Code Review Graph

Dự án đã tích hợp sẵn 2 công cụ phân tích đồ thị tri thức mã nguồn:
1. **`codebase-memory-mcp`:**
   - Tra cứu kiến trúc hệ thống: `get_architecture(aspects=['all'])`.
   - Tìm kiếm quan hệ giữa các thành phần mạng/workload: `search_graph`, `query_graph`, `trace_path`.
   - Cập nhật index sau mỗi thay đổi lớn: `index_repository(repo_path="...", persistence=true)`.
2. **`code-review-graph`:**
   - Đánh giá bán kính ảnh hưởng trước khi refactor hoặc sửa route/manifest: `get_impact_radius_tool`.
   - Lấy ngữ cảnh review đầy đủ: `get_review_context_tool`.
   - Cập nhật lại graph sau khi thêm file/module mới: `build_or_update_graph_tool`.

---

## 4. Quy chuẩn Bằng chứng (Evidence) & An toàn Bảo mật

- **Zero Fake Data:** Mọi kết luận kỹ thuật, bảng log, latency, packet drop phải lấy từ raw runtime evidence có timestamp.
- **Sanitization & Redaction (Bắt buộc):**
  - Che giấu 100% AWS Account ID (thay bằng `123456789012` hoặc `***`).
  - Tuyệt đối không commit Access Keys, Secret Tokens, Passwords, hoặc Private Keys vào repository.
- **FinOps & Cleanup:** Khi hoàn tất thử nghiệm hoặc GameDay, kích hoạt quy trình dọn dẹp theo thứ tự ngược (Reverse Cleanup Dependency) đảm bảo tài nguyên tính phí về **$0.00**.
