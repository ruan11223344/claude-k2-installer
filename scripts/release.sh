#!/bin/bash
set -e

# 配置
VERSION="1.0.0"
REPO="your-username/claude-k2-installer"  # 修改为你的仓库名
RELEASE_TITLE="Claude K2 Installer v${VERSION}"

# 检查 gh 是否安装
if ! command -v gh &> /dev/null; then
    echo "错误: 需要先安装 GitHub CLI"
    echo "运行: brew install gh"
    exit 1
fi

# 检查是否登录
if ! gh auth status &> /dev/null; then
    echo "需要先登录 GitHub"
    gh auth login
fi

# 检查文件是否存在
FILES=(
    "ClaudeK2Installer-${VERSION}-windows-x86_64.zip"
    "ClaudeK2Installer-${VERSION}-macos-arm64.zip"
    "ClaudeK2Installer-${VERSION}-macos-x86_64.zip"
)

for file in "${FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "错误: 文件不存在 - $file"
        exit 1
    fi
done

# 创建 release notes
RELEASE_NOTES="首次发布版本

## 功能特性
- 🚀 一键安装 Claude Code CLI
- 🔧 自动配置 Kimi K2 API
- 💻 支持 Windows/macOS/Linux
- 📦 智能检测并安装依赖（Node.js、Git）
- 🇨🇳 中国地区优化，使用国内镜像源
- 🎨 友好的图形界面

## 下载说明
- **ClaudeK2Installer-${VERSION}-windows-x86_64.zip**: Windows 64位版本
- **ClaudeK2Installer-${VERSION}-macos-arm64.zip**: macOS Apple Silicon (M1/M2/M3) 版本
- **ClaudeK2Installer-${VERSION}-macos-x86_64.zip**: macOS Intel 版本

## 使用方法
1. 下载对应平台的安装包
2. 解压并运行
3. 输入 Kimi API Key
4. 点击开始安装

## 系统要求
- Windows: Windows 10 或更高版本
- macOS: macOS 10.15 或更高版本
- 需要管理员权限（用于安装依赖）

## 常见问题
- 如遇到网络问题，工具会自动使用国内镜像
- macOS 用户首次运行可能需要在系统偏好设置中允许运行
- Windows 用户可能需要允许防火墙访问"

# 创建 tag
echo "创建 tag v${VERSION}..."
git tag -a "v${VERSION}" -m "Release v${VERSION}" || true
git push origin "v${VERSION}" || true

# 创建 release
echo "创建 GitHub Release..."
gh release create "v${VERSION}" \
    --repo "$REPO" \
    --title "$RELEASE_TITLE" \
    --notes "$RELEASE_NOTES" \
    "${FILES[@]}"

echo "✅ 发布成功！"
echo "查看发布: https://github.com/$REPO/releases/tag/v${VERSION}"