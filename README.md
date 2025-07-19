# Claude Code + K2 环境集成工具

一键安装配置 Claude Code 和 Kimi K2 大模型开发环境的 GUI 工具。

## 功能特点

- 🚀 **一键安装**：自动检测并安装所有必需组件
- 🔧 **智能配置**：自动配置环境变量和 API 设置
- 📚 **引导教程**：内置详细的使用教程
- 🖥️ **跨平台支持**：支持 Windows、macOS 和 Linux

## 系统要求

- Windows 10/11、macOS 10.12+ 或 Linux
- 至少 4GB 内存
- 稳定的网络连接

## 快速开始

### 1. 下载程序

从 Releases 页面下载对应系统的安装包：

- Windows: `ClaudeK2Installer-1.0.0-windows.zip`
- macOS: `ClaudeK2Installer-1.0.0-macos.zip`
- Linux: `ClaudeK2Installer-1.0.0-linux.tar.gz`

### 2. 运行程序

解压后双击运行程序。

### 3. 配置 K2 API

1. 访问 [Kimi 平台](https://platform.moonshot.cn/console/account)
2. 注册并充值至少 50 元
3. 创建 API Key
4. 在程序中输入 API Key

## 自动安装的组件

1. **Node.js** - JavaScript 运行环境
2. **Git** - 版本控制工具
3. **Claude Code** - AI 编程助手 CLI
4. **K2 API 配置** - Kimi 大模型接口

## 构建说明

### 前置要求

- Go 1.21 或更高版本
- Fyne 依赖

### 构建步骤

```bash
# 克隆仓库
git clone https://github.com/yourusername/claude-k2-installer.git
cd claude-k2-installer

# 安装依赖
go mod download

# 运行构建脚本
./build.sh
```

构建产物将在 `build/` 目录下生成。

## 开发说明

### 项目结构

```
claude-k2-installer/
├── main.go                 # 主程序入口
├── internal/
│   ├── installer/         # 安装器核心逻辑
│   └── ui/               # 用户界面组件
├── go.mod                # Go 模块定义
├── build.sh              # 构建脚本
└── README.md             # 本文件
```

## 常见问题

### Q: 为什么需要充值 50 元？

A: 免费账户的 RPM（每分钟请求数）限制为 3，不足以支持 Claude Code 的正常使用。充值后可以提升到更高的限制。

### Q: 安装失败怎么办？

A: 请查看安装日志，常见原因：
- 网络连接问题
- 权限不足（Linux/macOS 可能需要 sudo）
- 杀毒软件拦截

### Q: 支持哪些系统？

A: 目前支持：
- Windows 10/11 (64位)
- macOS 10.12+ (Intel & Apple Silicon)
- Linux (64位，主流发行版)

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

### AI 学习交流群

欢迎扫码加入 AI 学习交流群，分享最新 AI 知识，一起学习进步！

![AI学习交流群](contact_me_qr.png)

微信号：ruan11223344

### 问题反馈

如有技术问题，请在 GitHub 上提交 Issue。