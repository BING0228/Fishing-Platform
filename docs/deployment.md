# 部署说明

> 适用版本：包含 AI 助手、QR Relay 与无头浏览器克隆的当前版本。
> 两种方式：**单一 Linux 二进制 + systemd**，或 **Docker 镜像**。
> 一键脚本：`scripts/deploy.sh`；旧的 `scripts/deploy-remote.sh` 仍可用，但只支持二进制。

## 1. 选哪种

| 场景 | 推荐 |
|---|---|
| 单台 VPS、想少一层容器、直接 systemd 管理 | **binary** |
| 希望环境自包含、包含 Chromium、便于整体升级/回滚 | **docker** |
| 服务器没有 Docker | binary，或 `INSTALL_DOCKER=1` 让脚本安装 |
| 需要无头浏览器克隆 SPA/QR 页面 | 两者都支持；binary 需装 Chromium，Docker 镜像已内置 |

## 2. 环境要求

构建机（运行 `deploy.sh` 的机器）：

- Go 1.24+（binary 模式构建）；
- Node 20+ / npm（binary 模式构建前端，已有 `frontend/dist` 可跳过）；
- Docker 20+（docker 模式构建镜像）；
- SSH 客户端、`openssl`。

目标服务器：

- Linux x86_64 / arm64，systemd；
- 2 GB 内存起（无头 Chromium 渲染会额外占用内存）；
- 端口 `8000`（或自定义 `PORT`）对公网开放；
- root 或可免密 sudo 的用户（脚本默认 root）；
- `openssl`（生成密钥）、`curl`；
- binary 模式如需 SPA/QR 克隆：`chromium`（脚本默认安装）；
- docker 模式：本机 Docker（构建镜像）+ 目标机 Docker（运行）。

## 3. 一键部署

在仓库根目录执行。

### 3.1 二进制 + systemd

```bash
SSH_PASSWORD='***' ./scripts/deploy.sh binary 203.0.113.10 root 22
```

脚本会：

1. 必要时构建前端与 `linux/amd64|arm64` 二进制；
2. 上传到 `/opt/spearhook`；
3. 首次部署生成 `config.yaml`（随机密钥）；
4. 安装 `chromium`（`INSTALL_CHROMIUM=0` 可跳过）与 Python/Flask 本地运行时（`INSTALL_PYTHON=0` 可跳过）；
5. 安装并启动 `spearhook.service`；
6. 尝试放行防火墙端口；
7. 打印初始管理员密码。

常用变量：

```bash
PORT=8001 \
REMOTE_DIR=/opt/spearhook \
SERVICE_NAME=spearhook \
PUBLIC_BASE_URL=http://203.0.113.10:8001 \
SSH_PASSWORD='***' \
./scripts/deploy.sh binary 203.0.113.10 root 22
```

### 3.2 Docker

```bash
SSH_PASSWORD='***' ./scripts/deploy.sh docker 203.0.113.10 root 22
```

首次在目标机安装 Docker：

```bash
SSH_PASSWORD='***' INSTALL_DOCKER=1 \
  ./scripts/deploy.sh docker 203.0.113.10 root 22
```

脚本会：

1. 本地构建 `linux/amd64|arm64` 镜像（内置 Chromium、Python/Flask）；
2. `docker save | ssh docker load` 传到目标机；
3. 生成 `/opt/spearhook/config.yaml` 与 `data/`；
4. `docker run -d --network host`，挂载配置、数据、Docker socket；
5. 打印管理员密码。

> Docker 模式使用 `--network host` 让本地 Flask 项目直接绑定宿主机端口；同时挂载 `/var/run/docker.sock` 并设置 `DOCKER_HOST=unix:///var/run/docker.sock`，让项目容器在宿主机 Docker 上运行。目标机需为 Linux。

镜像标签与容器名：

```bash
IMAGE=spearhook:latest SERVICE_NAME=spearhook
```

## 4. Docker Compose（可选）

仓库根目录已提供 [docker-compose.yml](../docker-compose.yml)：

```bash
cp config.yaml.example config.yaml   # 修改 server.port 与 public_base_url
mkdir -p data
docker compose up -d --build
```

- 数据：`./data:/app/data`
- 配置：`./config.yaml:/app/config.yaml:ro`
- 网络：`network_mode: host`，应用端口以 `config.yaml` 的 `server.port` 为准；不通过 compose 映射端口
- 环境：`CHROME_PATH=/usr/bin/chromium`、`DOCKER_HOST=unix:///var/run/docker.sock`
- 运行时：镜像已包含 Chromium、Python/Flask，可运行 SPA 克隆、本地 Flask 项目
- 宿主机 Docker socket 让项目容器在宿主机上运行；容器内不再需要 Docker CLI
- Docker socket：挂载 `/var/run/docker.sock` 后，容器可创建宿主机上的项目容器（等同 root 权限，生产需评估）
- 不需要在宿主机额外安装 Chromium；宿主机需有 Docker daemon

如果由 `deploy.sh docker` 部署，容器已经启动；compose 只作为后续管理方式。宿主机需放行配置中的 `server.port`。

## 5. 配置

首次启动由脚本生成 `config.yaml`，关键项：

```yaml
server:
  port: 8000
  gin_mode: release
database:
  path: ./data/fishing.db
security:
  encryption_key: "<random>"
  jwt_secret: "<random>"
  container_secret: "<random>"
  agent_registration_token: "<random>"
  platform_basic_auth_enabled: false
container:
  ssl_cert_path: /app/certificates/cert.pem
  ssl_key_path: /app/certificates/key.pem
public_base_url: "http://203.0.113.10:8000"
mail_tracking:
  enabled: true
  secret: ""
  allow_redirect_hosts: []
```

注意：

- `security.encryption_key` **一旦有数据就不要再改**，否则已加密凭据无法解密。
- `public_base_url` 用于 QR 公开地址、邮件追踪、占位符替换；换域名后要同步修改并重启/重建项目。
- 生成配置默认关闭平台 Basic Auth。对外生产建议开启，并配合反向代理/访问控制。
- 管理员初始密码在 `data/admin_password.txt`，登录后修改密码并删除该文件。

## 6. 无头浏览器与 QR 克隆

- **binary**：脚本默认安装 Debian `chromium`，systemd 设置 `CHROME_PATH=/usr/bin/chromium`；
- **docker**：镜像内置 Chromium，环境变量同样指向 `/usr/bin/chromium`；
- 渲染触发：URL 带 `#fragment`，或静态 HTML 含 SPA/`canvas` 标记；
- 渲染流程：Chromium 打开页面 → 等待 4 秒 → 收集 canvas → QR 线索/解码；
- **Canvas/动态 QR：整页截图 + 原 QR 遮白 + 实时 relay QR 覆盖层**。截图页面保留原布局与背景图，移除全部脚本；不再做 AI 改写，避免冻结 DOM 后布局塌陷/竖排。覆盖层使用真实 `innerWidth/innerHeight` 计算百分比，原 QR 会被白色区域盖住；
- 非 Canvas QR：替换为 `{{QR_RELAY_URL}}` → 移除脚本 → AI 改写；
- 同一时间只跑一个渲染任务，避免内存被打满；
- 若服务器无 Chromium，静态 QR `<img>` 和直接 QR 图片仍可克隆，但 SPA/canvas 页面会报错。

Binary 部署时 Debian Chromium 额外占用：

```text
/usr/lib/chromium  约 332 MB
```

主程序约 **49 MB**（50,864,312 bytes，`-s -w` 静态构建）。Docker 镜像（Alpine + Chromium + Python/Flask）实测约 **748 MB**。

## 7. 验证

```bash
curl -I http://203.0.113.10:8000/
```

二进制：

```bash
systemctl status spearhook
journalctl -u spearhook -n 50 --no-pager
```

Docker：

```bash
docker ps
docker logs --tail 50 spearhook
```

首次登录：

```text
http://203.0.113.10:8000/
用户名：admin
密码：/opt/spearhook/data/admin_password.txt
```

然后到 **AI 设置** 启用一个 AI Profile；AI 助手、页面镜像、信息搜集都需要它。

## 8. 升级与回滚

### binary

```bash
# 备份
tar -czf /root/spearhook-backup-$(date +%F).tar.gz -C /opt/spearhook config.yaml data

# 升级（保留 config/data）
SSH_PASSWORD='***' ./scripts/deploy.sh binary 203.0.113.10 root 22
```

脚本会替换二进制并重启 systemd；已有 `config.yaml` 和 `data/` 不覆盖。

回滚：保留旧二进制并替换回去，或恢复备份目录后重启。

### docker

```bash
tar -czf /root/spearhook-backup-$(date +%F).tar.gz -C /opt/spearhook config.yaml data
SSH_PASSWORD='***' ./scripts/deploy.sh docker 203.0.113.10 root 22
```

数据在宿主机 `data/`，升级镜像不影响数据。回滚使用旧镜像标签重新 `docker run`。

## 9. 常见问题

| 现象 | 处理 |
|---|---|
| SSH `Connection closed` | 检查 SSH 端口、云安全组/防火墙、fail2ban；脚本支持 `SSH_PASSWORD` |
| 端口被占用 | 改 `PORT`；binary 检查 `ss -ltnp`，docker 检查 `docker ps` |
| Docker 未安装 | `INSTALL_DOCKER=1`，或手动安装 Docker |
| 镜像构建失败 | 本机需 Docker；检查网络、`Dockerfile`、`frontend/package-lock.json` |
| SPA/QR 克隆报 Chromium 错误 | 安装 `chromium`，设置 `CHROME_PATH`；binary 默认已安装 |
| 克隆页面没有二维码 | 页面可能是 canvas/JS 动态渲染；确认已启用无头渲染，或改用直接 QR 图片 URL |
| AI 助手无响应 | 到 `/ai-settings` 启用 AI Profile |
| 本地 Flask 项目重启后不在 | 平台重启不会自动恢复子进程；在项目页重新启动 |
| 公网打不开 | 检查云安全组、`ufw`/`firewalld`、`public_base_url` |

## 10. 安全注意

- 脚本默认 root 部署；生产建议使用专用低权限用户 + systemd hardening。
- Chromium 以 `--no-sandbox` 运行（容器/root 需要）；渲染的是不可信页面，建议隔离渲染器或限制并发。
- 生成配置默认关闭 Basic Auth；公网部署请加认证、反向代理、TLS 和来源限制。
- `config.yaml`、`data/`、SSH 密码和 AI API Key 都不要提交到仓库。
- 管理员初始密码文件登录后删除；定期轮换 AI/搜索 Key 和 SSH 凭据。
