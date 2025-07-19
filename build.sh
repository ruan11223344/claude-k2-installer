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

# 检查并安装fyne工具
Write-ColorOutput "检查fyne工具..." "Yellow"

# 获取Go路径并添加到PATH
$goPath = go env GOPATH
$goBin = "$goPath\bin"
$env:PATH += ";$goBin"

# 检查fyne是否可用
try {
    $fyneVersion = fyne version 2>$null
    if ($fyneVersion -match "deprecated") {
        Write-ColorOutput "检测到旧版fyne，升级到最新版本..." "Yellow"
        go install fyne.io/tools/cmd/fyne@latest
        if ($LASTEXITCODE -ne 0) {
            Write-ColorOutput "✗ fyne工具升级失败" "Red"
            exit 1
        }
    }
    Write-ColorOutput "✓ fyne工具可用" "Green"
} catch {
    Write-ColorOutput "安装fyne打包工具..." "Yellow"
    go install fyne.io/tools/cmd/fyne@latest
    if ($LASTEXITCODE -ne 0) {
        Write-ColorOutput "✗ fyne工具安装失败" "Red"
        exit 1
    }
    Write-ColorOutput "✓ fyne工具安装成功" "Green"
}

# 检查图标文件
if (-not (Test-Path "Icon.png")) {
    Write-ColorOutput "创建默认图标文件..." "Yellow"
    # 创建一个简单的占位图标文件
    New-Item -ItemType File -Path "Icon.png" -Force | Out-Null
    Write-ColorOutput "⚠️  请在项目根目录添加实际的 Icon.png 图标文件" "Yellow"
}

# 使用fyne打包Windows应用
Write-ColorOutput "使用fyne打包Windows应用..." "Blue"
fyne package -os windows -name $AppName

if ($LASTEXITCODE -eq 0) {
    # 检查生成的文件
    $exeFile = "$AppName.exe"
    if (Test-Path $exeFile) {
        # 移动到build目录
        Move-Item $exeFile "build\windows\"
        Write-ColorOutput "✓ Windows版本构建成功" "Green"
    } else {
        Write-ColorOutput "✗ 可执行文件未找到: $exeFile" "Red"
        exit 1
    }
} else {
    Write-ColorOutput "✗ Windows版本构建失败" "Red"
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
Write-ColorOutput "构建结果：" "Blue"

if (Test-Path "build") {
    Get-ChildItem "build" -Recurse | Format-Table Name, Length, LastWriteTime
} else {
    Write-ColorOutput "build目录为空" "Yellow"
}

# 显示文件大小
$buildFile = "build\windows\$AppName.exe"
if (Test-Path $buildFile) {
    $fileSize = [math]::Round((Get-Item $buildFile).Length / 1MB, 2)
    Write-ColorOutput "可执行文件大小: $fileSize MB" "Blue"
}

Write-ColorOutput "Windows构建脚本执行完成！" "Green"