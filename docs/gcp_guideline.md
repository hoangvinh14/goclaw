# GoClaw — GCP Deployment Guide (Compute Engine)

Hướng dẫn deploy GoClaw lên Google Cloud Platform sử dụng Compute Engine VM với Docker Compose.

---

## Yêu cầu

- [gcloud CLI](https://cloud.google.com/sdk/docs/install) đã cài đặt
- Tài khoản Google Cloud có billing được kích hoạt
- Repository GoClaw (public hoặc private với token)

---

## Bước 1: Cấu hình gcloud CLI

```bash
# Đăng nhập
gcloud auth login

# Tạo project mới (hoặc dùng project có sẵn)
gcloud projects create goclaw-prod --name="GoClaw Production"

# Gcloud sẽ tự tạo ID dạng goclaw-prod-XXXXXX, check bằng:
gcloud projects list

# Set project (dùng ID thực tế từ lệnh trên)
gcloud config set project <PROJECT_ID>

# Liên kết billing account
gcloud billing accounts list
gcloud billing projects link <PROJECT_ID> --billing-account=<BILLING_ACCOUNT_ID>

# Bật các API cần thiết
gcloud services enable compute.googleapis.com
```

---

## Bước 2: Tạo VM

```bash
gcloud compute instances create goclaw-server \
  --zone=asia-southeast1-b \
  --machine-type=e2-small \
  --image-family=debian-12 \
  --image-project=debian-cloud \
  --tags=http-server,https-server \
  --boot-disk-size=20GB
```

---

## Bước 3: Mở Firewall

```bash
# Mở port 3000 (Web UI)
gcloud compute firewall-rules create allow-goclaw-ui \
  --allow tcp:3000 \
  --target-tags http-server \
  --project <PROJECT_ID> \
  --description="GoClaw Web UI"
```

> **Lưu ý:** Port 18790 (backend API) **không cần** mở ra ngoài — chỉ cần giao tiếp nội bộ giữa các container Docker.

---

## Bước 4: SSH vào VM & Cài Docker

```bash
# SSH từ máy local
gcloud compute ssh goclaw-server --zone=asia-southeast1-b

# Trong VM: cài Docker Engine (từ official repo)
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Thêm user vào group docker
sudo usermod -aG docker $USER
newgrp docker

# Verify
docker --version
docker compose version
```

---

## Bước 5: Clone Repo & Chuẩn bị Env

```bash
# Clone repo
git clone https://github.com/nextlevelbuilder/goclaw.git
cd goclaw

# (Tuỳ chọn) Checkout branch cụ thể
git fetch
git switch -c feat/my-branch origin/feat/my-branch

# Tạo file .env và tự generate token/key
chmod +x prepare-env.sh
./prepare-env.sh

# (Tuỳ chọn) Thêm API keys vào .env
vi .env
# Ví dụ: thêm dòng GOCLAW_DASHSCOPE_API_KEY=sk-...
```

---

## Bước 6: Build & Chạy Docker Compose

```bash
# Chạy với PostgreSQL + Web UI (khuyến nghị)
docker compose \
  -f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml \
  up -d --build
```

Sau khi build xong (~2-3 phút), kiểm tra containers:

```bash
docker ps -a
# Phải thấy 3 containers đang chạy:
# goclaw-goclaw-1     (healthy) — port 18790
# goclaw-postgres-1   (healthy) — port 5432
# goclaw-goclaw-ui-1  (healthy) — port 3000
```

---

## Bước 7: Truy cập

Lấy IP của VM:
```bash
gcloud compute instances describe goclaw-server \
  --zone=asia-southeast1-b \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)'
```

Truy cập Web UI: **`http://<VM_IP>:3000`**

---

## Troubleshooting

### Không access được port 3000
1. Kiểm tra firewall rule tồn tại:
   ```bash
   gcloud compute firewall-rules list --project <PROJECT_ID>
   # Phải thấy allow-goclaw-ui với ALLOW=tcp:3000
   ```

2. Kiểm tra VM có tag `http-server`:
   ```bash
   gcloud compute instances describe goclaw-server \
     --zone=asia-southeast1-b \
     --format="get(tags.items)"
   # Phải thấy: http-server;https-server
   ```

3. Kiểm tra container UI đang chạy:
   ```bash
   docker ps -a
   # Nếu thiếu goclaw-goclaw-ui-1, chạy lại với đầy đủ -f flags (xem Bước 6)
   ```

### Xem logs

```bash
# Tất cả services
docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml logs --tail=50

# Một service cụ thể
docker logs goclaw-goclaw-1 --tail=50 -f
```

---

## Lưu ý bảo mật

- File `.env` chứa secrets — **không commit lên git**
- Cân nhắc dùng **Nginx reverse proxy + HTTPS** cho production thay vì expose port trực tiếp
- Hoặc dùng **SSH tunnel** để truy cập an toàn mà không cần mở firewall:
  ```bash
  gcloud compute ssh goclaw-server --zone=asia-southeast1-b \
    -- -L 3000:localhost:3000
  # Rồi truy cập http://localhost:3000 trên máy local
  ```
