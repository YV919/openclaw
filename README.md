# OpenClaw Config

<p align="center">
  <strong>OpenClaw DMXAPI 一键配置工具</strong>
</p>

<p align="center">
  基于 Wails + Vue 3 + TypeScript 构建的跨平台桌面 GUI 应用
</p>

---

## 简介

OpenClaw Config 是一个简洁易用的桌面配置工具，帮助用户快速配置 OpenClaw 的 DMXAPI 设置，无需手动编辑配置文件。

### 主要功能

- 可视化配置 DMXAPI 的 Base URL、API Key 和模型
- 内置多种预设模型快速选择
- 支持自定义模型名称
- 一键保存配置到 OpenClaw 配置文件

### 支持的预设模型

- Claude 系列：claude-opus-4-6-cc、claude-sonnet-4-5、claude-haiku-4-5 等
- Kimi：kimi-k2.5-cc
- MiniMax：MiniMax-M2.1-cc、MiniMax-M2-cc
- DeepSeek：DeepSeek-V3.2-cc
- GLM：glm-4.7-cc
- 更多模型持续更新中...

---

## 下载安装

前往 [Releases](../../releases) 页面下载对应平台的版本：

| 平台 | 架构 | 文件 | 说明 |
|------|------|------|------|
| Windows | x64 | `openclaw-config-windows` | 直接运行 |
| macOS | Universal | `openclaw-config-macos` | 支持 Intel 和 Apple Silicon |
| Linux | x64 | `openclaw-config-linux` | 需添加执行权限 |

---

## 使用说明

### Windows

下载后直接双击运行 `openclaw-config-windows`

### Linux

```bash
# 下载后添加执行权限
chmod +x openclaw-config-linux

# 运行
./openclaw-config-linux
```

### macOS

1. 下载后添加执行权限：`chmod +x openclaw-config-macos`
2. 双击运行或在终端执行
3. 首次运行可能需要在「系统设置 > 隐私与安全性」中点击「仍要打开」

---

## 配置完成后

配置保存后，使用以下命令使配置生效：

```bash
# 重启网关使配置生效
openclaw gateway restart
```

### 常用命令

| 命令 | 说明 |
|------|------|
| `openclaw gateway restart` | 重启网关使配置生效 |
| `/model` | 在聊天中查看/切换模型 |
| `openclaw models list` | 列出所有可用模型 |
| `openclaw-cn tui` | 打开终端聊天助手 |

---

## 本地开发

### 环境要求

- Go 1.21+
- Node.js 18+
- Wails CLI v2.x

### 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 克隆项目

```bash
git clone https://github.com/YeSongYun/openclaw_config.git
cd openclaw_config
```

### 安装前端依赖

```bash
cd frontend && npm install && cd ..
```

### 开发模式

```bash
wails dev
```

开发模式下支持热重载，前端修改会自动刷新。

### 构建

```bash
# 构建当前平台（开发测试用）
wails build

# 构建 macOS Universal 版本（同时支持 Intel 和 Apple Silicon）
# 替换 v1.0.0 为实际版本号
wails build -platform darwin/universal -ldflags "-s -w" -o openclaw-config-v1.0.0-macos-universal

# 构建 Windows 版本
wails build -platform windows/amd64 -ldflags "-s -w" -o openclaw-config-v1.0.0-windows-amd64.exe

# 构建 Linux 版本
wails build -platform linux/amd64 -ldflags "-s -w" -o openclaw-config-v1.0.0-linux-amd64
wails build -platform linux/arm64 -ldflags "-s -w" -o openclaw-config-v1.0.0-linux-arm64
```

构建产物位于 `build/bin/` 目录。

---

## 项目结构

```
openclaw_config/
├── app.go                    # 主应用逻辑，暴露给前端的 API
├── main.go                   # 程序入口
├── wails.json                # Wails 配置文件
├── internal/
│   ├── config/
│   │   ├── manager.go        # 配置管理器
│   │   └── types.go          # 配置类型定义
│   └── models/
│       └── presets.go        # 预设模型列表
├── frontend/                 # Vue 3 前端
│   └── src/
│       ├── App.vue           # 主组件
│       └── components/
│           └── ConfigForm.vue # 配置表单组件
├── build/                    # 构建相关资源
├── .github/workflows/        # GitHub Actions CI/CD
└── .cnb.yml                  # CNB 云原生构建配置
```

---

## CI/CD

项目配置了自动化构建和发布：

- **CNB**：推送 Tag 时创建 Release，需手动上传构建产物
- **GitHub Actions**：推送 Tag 时自动构建 Linux、Windows 和 macOS 版本

---

## 相关链接

- [OpenClaw 项目主页](https://cnb.cool/dmxapi/openclaw_config)
- [Wails 官方文档](https://wails.io/docs)

---

## 作者

**yesongyun** - yesongyun@foxmail.com

---

## License

MIT
