#!/bin/bash

# Claude Code + K2 Installer 构建脚本

# 设置颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}开始构建 Claude Code + K2 Installer...${NC}"

# 设置应用名称
APP_NAME="ClaudeK2Installer"
VERSION="1.0.0"

# 检测当前系统
OS=$(uname -s)
ARCH=$(uname -m)

echo -e "${BLUE}检测到系统: $OS ($ARCH)${NC}"

# 清理旧的构建
rm -rf build/
mkdir -p build

# 获取依赖
echo -e "${YELLOW}获取依赖...${NC}"
GOPROXY=https://goproxy.cn,direct go mod download

# 根据当前系统构建
case "$OS" in
    Darwin)
        # macOS 构建
        echo -e "${GREEN}构建 macOS 版本...${NC}"
        mkdir -p build/macos
        
        if [ "$ARCH" = "arm64" ]; then
            echo -e "${BLUE}构建 Apple Silicon 版本...${NC}"
            go build -ldflags="-w -s" -tags bundled -o "build/macos/${APP_NAME}" .
        else
            echo -e "${BLUE}构建 Intel 版本...${NC}"
            go build -ldflags="-w -s" -tags bundled -o "build/macos/${APP_NAME}" .
        fi
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ macOS 版本构建成功${NC}"
        else
            echo -e "${RED}✗ macOS 版本构建失败${NC}"
            exit 1
        fi
        ;;
        
    Linux)
        # Linux 构建
        echo -e "${GREEN}构建 Linux 版本...${NC}"
        mkdir -p build/linux
        go build -ldflags="-w -s" -tags bundled -o "build/linux/${APP_NAME}" .
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ Linux 版本构建成功${NC}"
        else
            echo -e "${RED}✗ Linux 版本构建失败${NC}"
            exit 1
        fi
        ;;
        
    MINGW*|MSYS*|CYGWIN*)
        # Windows 构建
        echo -e "${GREEN}构建 Windows 版本...${NC}"
        mkdir -p build/windows
        go build -ldflags="-H windowsgui -w -s" -tags bundled -o "build/windows/${APP_NAME}.exe" .
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ Windows 版本构建成功${NC}"
        else
            echo -e "${RED}✗ Windows 版本构建失败${NC}"
            exit 1
        fi
        ;;
        
    *)
        echo -e "${RED}不支持的操作系统: $OS${NC}"
        exit 1
        ;;
esac

# 如果是 macOS，创建 .app 包
if [ "$OS" = "Darwin" ]; then
    echo -e "${GREEN}创建 macOS .app 包...${NC}"
    mkdir -p "build/macos/${APP_NAME}.app/Contents/MacOS"
    mkdir -p "build/macos/${APP_NAME}.app/Contents/Resources"

    # 创建 Info.plist
    cat > "build/macos/${APP_NAME}.app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIconFile</key>
    <string>icon.icns</string>
    <key>CFBundleIdentifier</key>
    <string>com.claude-k2.installer</string>
    <key>CFBundleName</key>
    <string>Claude K2 Installer</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.12</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

    # 复制可执行文件到 .app
    cp "build/macos/${APP_NAME}" "build/macos/${APP_NAME}.app/Contents/MacOS/"
fi

# 创建压缩包
echo -e "${GREEN}创建发布包...${NC}"
cd build

case "$OS" in
    Darwin)
        # macOS 压缩
        if [ -d "macos/${APP_NAME}.app" ]; then
            cd macos
            zip -qr "../${APP_NAME}-${VERSION}-macos-${ARCH}.zip" "${APP_NAME}.app"
            cd ..
            echo -e "${GREEN}✓ 输出文件: build/${APP_NAME}-${VERSION}-macos-${ARCH}.zip${NC}"
        else
            cd macos
            zip -q "../${APP_NAME}-${VERSION}-macos-${ARCH}.zip" "${APP_NAME}"
            cd ..
            echo -e "${GREEN}✓ 输出文件: build/${APP_NAME}-${VERSION}-macos-${ARCH}.zip${NC}"
        fi
        ;;
        
    Linux)
        # Linux 压缩
        cd linux
        tar -czf "../${APP_NAME}-${VERSION}-linux-${ARCH}.tar.gz" "${APP_NAME}"
        cd ..
        echo -e "${GREEN}✓ 输出文件: build/${APP_NAME}-${VERSION}-linux-${ARCH}.tar.gz${NC}"
        ;;
        
    MINGW*|MSYS*|CYGWIN*)
        # Windows 压缩
        cd windows
        zip -q "../${APP_NAME}-${VERSION}-windows.zip" "${APP_NAME}.exe"
        cd ..
        echo -e "${GREEN}✓ 输出文件: build/${APP_NAME}-${VERSION}-windows.zip${NC}"
        ;;
esac

cd ..

echo -e "${GREEN}构建完成！${NC}"