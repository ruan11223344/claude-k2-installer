# Claude Code + K2 Installer Windows 构建脚本
# PowerShell 版本

param(
    [string]$AppName = "ClaudeK2Installer",
    [string]$Version = "1.0.0"
)

# 颜色输出函数
function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

Write-ColorOutput "开始构建 Claude Code + K2 Installer (Windows)..." "Green"
Write-ColorOutput "应用名称: $AppName" "Blue"
Write-ColorOutput "版本: $Version" "Blue"

# 清理旧的构建
Write-ColorOutput "清理旧构建文件..." "Yellow"
if (Test-Path "build") {
    Remove-Item -Path "build" -Recurse -Force
}
New-Item -ItemType Directory -Path "build\windows" -Force | Out-Null

# 检查Go环境
Write-ColorOutput "检查Go环境..." "Yellow"
try {
    $goVersion = go version
    Write-ColorOutput "✓ Go环境正常: $goVersion" "Green"
} catch {
    Write-ColorOutput "✗ Go环境未找到，请先安装Go" "Red"
    exit 1
}

# 获取依赖
Write-ColorOutput "获取Go依赖..." "Yellow"
$env:GOPROXY = "https://goproxy.cn,direct"
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-ColorOutput "✗ 依赖获取失败" "Red"
    exit 1
}

# 获取CPU核心数
$cores = (Get-CimInstance Win32_ComputerSystem).NumberOfLogicalProcessors
if (-not $cores) { $cores = 4 }  # 默认值
Write-ColorOutput "检测到 $cores 个CPU核心" "Blue"

# 设置环境变量以使用多线程
$env:GOMAXPROCS = $cores

# 执行多线程编译
Write-ColorOutput "执行多线程编译（$cores 线程）..." "Yellow"
go build -p $cores -ldflags="-H windowsgui -w -s" -tags bundled -o "build\windows\$AppName.exe" .

if ($LASTEXITCODE -eq 0) {
    Write-ColorOutput "✓ 编译成功" "Green"
} else {
    Write-ColorOutput "✗ 编译失败" "Red"
    exit 1
}

# 创建发布包
Write-ColorOutput "创建发布包..." "Green"
$zipName = "$AppName-$Version-windows.zip"

# 压缩文件
try {
    Add-Type -AssemblyName "System.IO.Compression.FileSystem"
    [System.IO.Compression.ZipFile]::CreateFromDirectory(
        (Join-Path $PWD "build\windows"),
        (Join-Path $PWD "build\$zipName")
    )
    Write-ColorOutput "✓ 输出文件: build\$zipName" "Green"
} catch {
    Write-ColorOutput "压缩失败，尝试使用PowerShell Compress-Archive..." "Yellow"
    Compress-Archive -Path "build\windows\*" -DestinationPath "build\$zipName" -Force
    if ($LASTEXITCODE -eq 0) {
        Write-ColorOutput "✓ 输出文件: build\$zipName" "Green"
    } else {
        Write-ColorOutput "✗ 压缩失败" "Red"
        exit 1
    }
}

# 显示构建结果
Write-ColorOutput "构建完成！" "Green"

# 显示文件大小
$buildFile = "build\windows\$AppName.exe"
if (Test-Path $buildFile) {
    $fileSize = [math]::Round((Get-Item $buildFile).Length / 1MB, 2)
    Write-ColorOutput "可执行文件大小: $fileSize MB" "Blue"
}

Write-ColorOutput "Windows构建脚本执行完成！" "Green"