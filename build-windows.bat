@echo off
setlocal enabledelayedexpansion

REM Claude Code + K2 Installer Windows 构建脚本（批处理版本）

echo ========================================
echo Claude Code + K2 Installer 构建工具
echo ========================================
echo.

set APP_NAME=ClaudeK2Installer
set VERSION=1.0.0

REM 检查 Go 环境
echo [检查] Go 环境...
go version >nul 2>&1
if errorlevel 1 (
    echo [错误] 未找到 Go 环境，请先安装 Go
    pause
    exit /b 1
)
echo [成功] Go 环境正常

REM 获取 CPU 核心数
for /f "tokens=2 delims==" %%i in ('wmic cpu get NumberOfLogicalProcessors /value') do set CORES=%%i
if not defined CORES set CORES=4
echo [信息] 检测到 %CORES% 个 CPU 核心

REM 清理旧构建
echo.
echo [清理] 删除旧的构建文件...
if exist build rmdir /s /q build
mkdir build\windows

REM 设置 Go 代理
echo.
echo [配置] 设置 Go 代理为国内镜像...
set GOPROXY=https://goproxy.cn,direct

REM 获取依赖
echo.
echo [依赖] 下载项目依赖...
go mod download
if errorlevel 1 (
    echo [错误] 依赖下载失败
    pause
    exit /b 1
)

REM 设置多线程编译
echo.
echo [编译] 开始多线程编译（使用 %CORES% 个线程）...
set GOMAXPROCS=%CORES%

REM 执行编译
go build -p %CORES% -ldflags="-H windowsgui -w -s" -tags bundled -o "build\windows\%APP_NAME%.exe" .

if errorlevel 1 (
    echo [错误] 编译失败
    pause
    exit /b 1
)

echo [成功] 编译完成！

REM 检查输出文件
if not exist "build\windows\%APP_NAME%.exe" (
    echo [错误] 可执行文件未找到
    pause
    exit /b 1
)

REM 获取文件大小
for %%A in ("build\windows\%APP_NAME%.exe") do set SIZE=%%~zA
set /a SIZE_MB=%SIZE% / 1048576
echo [信息] 可执行文件大小: %SIZE_MB% MB

REM 创建压缩包
echo.
echo [打包] 创建发布包...
cd build

REM 检查是否有 PowerShell
powershell -Command "Get-Command Compress-Archive" >nul 2>&1
if errorlevel 1 (
    echo [警告] PowerShell 压缩命令不可用，请手动压缩 build\windows 目录
) else (
    powershell -Command "Compress-Archive -Path 'windows\*' -DestinationPath '%APP_NAME%-%VERSION%-windows.zip' -Force"
    if exist "%APP_NAME%-%VERSION%-windows.zip" (
        echo [成功] 已创建压缩包: %APP_NAME%-%VERSION%-windows.zip
    )
)

cd ..

echo.
echo ========================================
echo 构建完成！
echo 输出目录: build\windows\
echo 可执行文件: %APP_NAME%.exe
echo ========================================
echo.
pause