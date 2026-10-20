# Spearhook documentation

Operator guides for this repository. Start with the root [README](../README.md) ([简体中文](../README.zh.md) · [繁體中文](../README.zh-TW.md) · [日本語](../README.ja.md) · [한국어](../README.ko.md)) for positioning, features, architecture, and quick start.

## Published operator guides

These files are tracked in git and linked from the root README.

| Document | Language | Description |
|----------|----------|-------------|
| [placeholders.en.md](placeholders.en.md) | EN | HTML + mail template placeholders |
| [placeholders.zh.md](placeholders.zh.md) | ZH | 页面与邮件占位符说明 |
| [agent-deployment.md](agent-deployment.md) | ZH | Agent install: systemd, Windows service, reverse proxy |
| [ai-native-assessment.md](ai-native-assessment.md) | ZH | AI Native 可行性评估与落地路径 |
| [ai-native-architecture.md](ai-native-architecture.md) | ZH | AI Native 实现架构（包结构 / 工具表 / SSE / PR 切片） |
| [deployment.md](deployment.md) | ZH | 一键部署：Docker 镜像 / 单一二进制、配置、升级与排障 |
| [demo-hosting.md](demo-hosting.md) | EN | Host the read-only demo on Cloudflare Pages (Git integration) |
| [../scripts/qr-relay/README.md](../scripts/qr-relay/README.md) | EN | QR screen-capture uploader |
| [../scripts/qr-relay/README.zh.md](../scripts/qr-relay/README.zh.md) | ZH | QR 截屏上传脚本 |

## Related root files

| Path | Description |
|------|-------------|
| [../README.md](../README.md) | English platform introduction (source language) |
| [../README.zh.md](../README.zh.md) | 简体中文平台介绍 |
| [../README.zh-TW.md](../README.zh-TW.md) | 繁體中文平台介紹 |
| [../README.ja.md](../README.ja.md) | 日本語プラットフォーム紹介 |
| [../README.ko.md](../README.ko.md) | 한국어 플랫폼 소개 |
| [../config.yaml.example](../config.yaml.example) | Control-plane config template |
| [../agent.yaml.example](../agent.yaml.example) | Agent config template |

## Local working-tree notes (not published)

The following patterns are **gitignored** on purpose (see root `.gitignore`). They may exist in a developer checkout for product/design work, but they are **not** part of the published GitHub tree — do not rely on them from the public README.

- `docs/*-plan.md` (e.g. chat console plan)
- `docs/*-product-design.md`
- `docs/*-experience.md` (Slate Console adoption notes)
- `docs/*-review.md` (internal audit/security reviews)
