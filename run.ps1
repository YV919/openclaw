param(
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$BinaryArgs = @()
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoUrl = if ($env:OPENCLAW_CONFIG_REPO_URL) { $env:OPENCLAW_CONFIG_REPO_URL } else { "https://cnb.cool/dmxapi/openclaw_config" }
$VersionOverride = $env:OPENCLAW_CONFIG_VERSION
$exitCode = 1
$shouldExitProcess = $env:OPENCLAW_CONFIG_EXIT_PROCESS -eq "1"

function Write-Log {
  param([string]$Message)
  Write-Host $Message
}

function Get-LatestTag {
  if ($VersionOverride) {
    return $VersionOverride
  }

  $response = Invoke-WebRequest -Uri "$RepoUrl/-/releases" -UseBasicParsing
  $match = [regex]::Match($response.Content, '/releases/tag/([^"]+)')
  if (-not $match.Success) {
    throw "无法从 release 页面解析最新版本"
  }

  return $match.Groups[1].Value
}

function Get-AssetName {
  $arch = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant()
  switch ($arch) {
    "x64" {
      return @{
        Arch = "amd64"
        Name = "openclaw-config-windows-amd64.exe"
      }
    }
    default {
      throw "暂不支持的 Windows 架构: $arch"
    }
  }
}

$asset = Get-AssetName
$tag = Get-LatestTag
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("openclaw-config-run-" + [guid]::NewGuid().ToString("N"))
$binaryPath = Join-Path $tempDir $asset.Name

try {
  New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

  Write-Log "检测到平台: windows/$($asset.Arch)"
  Write-Log "准备运行版本: $tag"
  Write-Log "正在下载并启动 openclaw-config..."

  Invoke-WebRequest -Uri "$RepoUrl/-/releases/download/$tag/$($asset.Name)" -OutFile $binaryPath -UseBasicParsing
  & $binaryPath @BinaryArgs
  $exitCode = $LASTEXITCODE
  if ($null -eq $exitCode) {
    $exitCode = 0
  }
}
finally {
  Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}

if ($shouldExitProcess) {
  exit $exitCode
}

$global:LASTEXITCODE = $exitCode
return
