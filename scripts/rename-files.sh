#!/bin/bash

# 重命名 Windows 文件
if [ -f "ClaudeK2Installer.zip" ]; then
    mv ClaudeK2Installer.zip ClaudeK2Installer-1.0.0-windows-x86_64.zip
    echo "✅ 已重命名: ClaudeK2Installer.zip -> ClaudeK2Installer-1.0.0-windows-x86_64.zip"
else
    echo "⚠️  文件不存在: ClaudeK2Installer.zip"
fi

# 检查所有文件是否存在
echo ""
echo "检查发布文件:"
for file in ClaudeK2Installer-1.0.0-*.zip; do
    if [ -f "$file" ]; then
        echo "✅ $file ($(ls -lh "$file" | awk '{print $5}'))"
    fi
done