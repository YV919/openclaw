# OpenClaw Config

<p align="center">
  <strong>OpenClaw DMXAPI 一键配置工具</strong>
</p>

<p align="center">
  纯 Go 终端 TUI 工具，零 CGO，支持 5 平台一键交叉编译
</p>

---

## 简介

OpenClaw Config 是一个简洁易用的终端配置工具，帮助用户快速配置 OpenClaw 的 DMXAPI 设置，无需手动编辑配置文件。

### 主要功能

- 终端交互式配置 DMXAPI 的 Base URL、API Key 和模型
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

| 平台 | 架构 | 文件 |
|------|------|------|
| Linux | x64 | `openclaw-config-linux-amd64` |
| Linux | ARM64 | `openclaw-config-linux-arm64` |
| Windows | x64 | `openclaw-config-windows-amd64.exe` |
| macOS | Intel | `openclaw-config-darwin-amd64` |
| macOS | Apple Silicon | `openclaw-config-darwin-arm64` |

---

## 使用说明

### Linux / macOS

```bash
# 添加执行权限
chmod +x openclaw-config-linux-amd64

# 运行
./openclaw-config-linux-amd64
```

### Windows

```
openclaw-config-windows-amd64.exe
```

### 版本查看

```bash
./openclaw-config-linux-amd64 --version
```

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

- Go 1.23+

### 克隆项目

```bash
git clone https://github.com/YeSongYun/openclaw_config.git
cd openclaw_config
```

### 开发模式（直接运行）

```bash
go run .
```

### 构建

```bash
# 构建当前平台
go build -o openclaw-config .

# 交叉编译（5 平台）
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o dist/openclaw-config-linux-amd64 .
GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -o dist/openclaw-config-linux-arm64 .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o dist/openclaw-config-windows-amd64.exe .
GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -o dist/openclaw-config-darwin-amd64 .
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o dist/openclaw-config-darwin-arm64 .
```

---

## 项目结构

```
openclaw_config/
├── app.go                    # TUI 表单逻辑
├── main.go                   # 程序入口（参数解析）
├── internal/
│   ├── config/
│   │   ├── manager.go        # 配置管理器
│   │   └── types.go          # 配置类型定义
│   └── models/
│       └── presets.go        # 预设模型列表
├── .github/workflows/        # GitHub Actions CI/CD
└── .cnb.yml                  # CNB 云原生构建配置
```

---

## CI/CD

项目配置了自动化构建和发布：

- **CNB**：推送 Tag 时自动构建 5 个平台并创建 Release
- **GitHub Actions**：推送 Tag 时自动构建 5 个平台并发布

---

## 相关链接

- [OpenClaw 项目主页](https://cnb.cool/dmxapi/openclaw_config)

---

## 作者

**yesongyun** - yesongyun@foxmail.com

---

## License

MIT
