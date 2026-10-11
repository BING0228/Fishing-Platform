# QR 截屏中继

在本机持续截取屏幕上的二维码，把画面上传到钓鱼台，由固定公开链接托管最新一帧。仅用于授权演练。

推荐从控制台 **工作台 → QR 钓鱼** 下载 `fishing-qr-relay.zip`（内含可改的 `config.yaml`）。

## 步骤

1. 在 **工作台 → QR 钓鱼** 创建中继，**立刻复制上传令牌**（只显示一次）。漏了就到中继详情点「轮换令牌」。
2. 把令牌写入 `config.yaml` 的 `upload_token`。`platform_url` 一般已由控制台填好。
3. 安装依赖并运行：

```bash
# macOS 解码二维码需要系统库
brew install zbar

python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# 可选：拖选屏幕区域，写入 config.yaml 的 region
python qr_relay.py --pick-region -c config.yaml

python qr_relay.py -c config.yaml
```

Windows 激活虚拟环境用 `.venv\Scripts\activate`。

## 脚本会做什么

- 只把**图片**上传到 `POST /api/qr-relay/frames/`
- 画面里暂时没有二维码时，发送 `POST /api/qr-relay/heartbeat/`（控制台健康状态保持「在线」）
- 每次截屏都会尝试解码，解码文本放在 multipart 字段 `payload`

请求头：`X-QR-Relay-Token: <令牌>`  
表单字段：`image`（png / jpeg / gif / webp，≤ 2 MiB）

对外始终返回最新一帧：

`GET /q/{slug}.png`

把该地址填进页面占位符 `{{QR_RELAY_URL}}` / `{{QR_RELAY_IMG}}` 时，须先在项目里绑定这个中继，再构建或启动项目。
