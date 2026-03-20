#!/bin/sh
set -eu

REPO_SLUG=${OPENCLAW_CONFIG_REPO_SLUG:-dmxapi/openclaw_config}
REPO_URL=${OPENCLAW_CONFIG_REPO_URL:-https://cnb.cool/$REPO_SLUG}
BINARY_NAME=${OPENCLAW_CONFIG_BINARY_NAME:-openclaw-config}

log() {
  printf '%s\n' "$*" >&2
}

die() {
  log "运行失败: $*"
  exit 1
}

normalize_os() {
  case "$1" in
    Linux)
      printf '%s\n' "linux"
      ;;
    Darwin)
      printf '%s\n' "macos"
      ;;
    MINGW*|MSYS*|CYGWIN*|Windows_NT)
      printf '%s\n' "windows"
      ;;
    *)
      return 1
      ;;
  esac
}

normalize_arch() {
  case "$1" in
    x86_64|amd64)
      printf '%s\n' "amd64"
      ;;
    aarch64|arm64)
      printf '%s\n' "arm64"
      ;;
    *)
      return 1
      ;;
  esac
}

asset_name_for() {
  os=$1
  arch=$2

  case "$os-$arch" in
    linux-amd64)
      printf '%s\n' "openclaw-config-linux-amd64"
      ;;
    linux-arm64)
      printf '%s\n' "openclaw-config-linux-arm64"
      ;;
    macos-amd64)
      printf '%s\n' "openclaw-config-macos-amd64"
      ;;
    macos-arm64)
      printf '%s\n' "openclaw-config-macos-arm64"
      ;;
    windows-amd64)
      printf '%s\n' "openclaw-config-windows-amd64.exe"
      ;;
    *)
      return 1
      ;;
  esac
}

extract_latest_tag() {
  latest_tag=$(sed -n 's#.*href="/[^"]*/-/releases/tag/\([^"]*\)".*#\1#p' | head -n 1)

  if [ -z "$latest_tag" ]; then
    return 1
  fi

  printf '%s\n' "$latest_tag"
}

fetch_latest_tag() {
  if [ -n "${OPENCLAW_CONFIG_VERSION:-}" ]; then
    printf '%s\n' "$OPENCLAW_CONFIG_VERSION"
    return 0
  fi

  releases_html=$(
    curl -fsSL --retry 3 --connect-timeout 15 "$REPO_URL/-/releases"
  ) || die "无法获取 release 列表"

  latest_tag=$(
    printf '%s' "$releases_html" | extract_latest_tag
  ) || die "无法从 release 页面解析最新版本"

  printf '%s\n' "$latest_tag"
}

download_asset() {
  tag=$1
  asset_name=$2
  output_path=$3

  curl -fL --retry 3 --connect-timeout 15 -o "$output_path" \
    "$REPO_URL/-/releases/download/$tag/$asset_name" || die "下载 $asset_name 失败"
}

make_temp_dir() {
  if [ -n "${OPENCLAW_CONFIG_TMPDIR:-}" ]; then
    mkdir -p "$OPENCLAW_CONFIG_TMPDIR" || die "无法创建临时目录根路径: $OPENCLAW_CONFIG_TMPDIR"
    mktemp -d "$OPENCLAW_CONFIG_TMPDIR/openclaw-config-run.XXXXXX" 2>/dev/null ||
      die "无法在 $OPENCLAW_CONFIG_TMPDIR 下创建临时目录"
    return 0
  fi

  mktemp -d 2>/dev/null || mktemp -d -t openclaw-config-run
}

clear_macos_quarantine() {
  target_path=$1

  if [ "$(uname -s 2>/dev/null || true)" != "Darwin" ]; then
    return 0
  fi

  if command -v xattr >/dev/null 2>&1; then
    xattr -rd com.apple.quarantine "$target_path" >/dev/null 2>&1 || true
  fi
}

run_binary() {
  binary_path=$1
  shift

  if "$binary_path" "$@"; then
    return 0
  fi

  return $?
}

main() {
  command -v curl >/dev/null 2>&1 || die "未找到 curl"

  uname_s=${OPENCLAW_CONFIG_UNAME_S:-$(uname -s 2>/dev/null || echo unknown)}
  uname_m=${OPENCLAW_CONFIG_UNAME_M:-$(uname -m 2>/dev/null || echo unknown)}

  os=$(normalize_os "$uname_s") || die "暂不支持的系统: $uname_s"
  arch=$(normalize_arch "$uname_m") || die "暂不支持的架构: $uname_m"
  asset_name=$(asset_name_for "$os" "$arch") || die "暂不支持的平台组合: $os/$arch"
  latest_tag=$(fetch_latest_tag)

  tmp_dir=$(make_temp_dir)
  trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

  tmp_asset="$tmp_dir/$asset_name"

  log "检测到平台: $os/$arch"
  log "准备运行版本: $latest_tag"
  log "正在下载并启动 $BINARY_NAME..."

  download_asset "$latest_tag" "$asset_name" "$tmp_asset"
  chmod 755 "$tmp_asset" >/dev/null 2>&1 || true
  clear_macos_quarantine "$tmp_asset"

  if run_binary "$tmp_asset" "$@"; then
    exit 0
  else
    status=$?
    exit "$status"
  fi
}

if [ "${OPENCLAW_CONFIG_RUN_TESTING:-0}" != "1" ]; then
  main "$@"
fi
