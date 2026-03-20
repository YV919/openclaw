#!/bin/sh
set -eu

REPO_SLUG=${OPENCLAW_CONFIG_REPO_SLUG:-dmxapi/openclaw_config}
REPO_URL=${OPENCLAW_CONFIG_REPO_URL:-https://cnb.cool/$REPO_SLUG}
BINARY_NAME=openclaw-config

log() {
  printf '%s\n' "$*" >&2
}

die() {
  log "安装失败: $*"
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

command_name_for() {
  case "$1" in
    windows)
      printf '%s\n' "$BINARY_NAME.exe"
      ;;
    *)
      printf '%s\n' "$BINARY_NAME"
      ;;
  esac
}

path_contains_dir() {
  search_dir=$1
  path_value=${2:-}
  old_ifs=$IFS
  IFS=:

  for entry in $path_value; do
    if [ "$entry" = "$search_dir" ]; then
      IFS=$old_ifs
      return 0
    fi
  done

  IFS=$old_ifs
  return 1
}

choose_install_dir() {
  os=$1
  command_name=$2

  if [ -n "${INSTALL_DIR:-}" ]; then
    printf '%s\n' "$INSTALL_DIR"
    return 0
  fi

  existing_path=$(command -v "$command_name" 2>/dev/null || true)
  if [ -n "$existing_path" ]; then
    existing_dir=$(dirname "$existing_path")
    if [ -w "$existing_dir" ]; then
      printf '%s\n' "$existing_dir"
      return 0
    fi
  fi

  if [ "$os" != "windows" ] && [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    printf '%s\n' "/usr/local/bin"
    return 0
  fi

  if [ "$os" = "macos" ] && [ -d "/opt/homebrew/bin" ] && [ -w "/opt/homebrew/bin" ]; then
    printf '%s\n' "/opt/homebrew/bin"
    return 0
  fi

  if [ -z "${HOME:-}" ]; then
    die "未设置 HOME，请通过 INSTALL_DIR 指定安装目录"
  fi

  printf '%s\n' "$HOME/.local/bin"
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

clear_macos_quarantine() {
  target_path=$1

  if [ "$(uname -s 2>/dev/null || true)" != "Darwin" ]; then
    return 0
  fi

  if command -v xattr >/dev/null 2>&1; then
    xattr -rd com.apple.quarantine "$target_path" >/dev/null 2>&1 || true
  fi
}

print_success() {
  install_dir=$1
  target_path=$2
  command_name=$3
  tag=$4

  log ""
  log "已安装 $command_name ($tag)"
  log "安装路径: $target_path"

  if path_contains_dir "$install_dir" "${PATH:-}"; then
    log "直接运行: $command_name --version"
    return 0
  fi

  log "当前 PATH 不包含 $install_dir"
  log "你可以直接运行: \"$target_path\" --version"
  log "也可以把下面这一行加入 shell 配置文件后重新打开终端:"
  log "export PATH=\"$install_dir:\$PATH\""
}

main() {
  command -v curl >/dev/null 2>&1 || die "未找到 curl"

  uname_s=${OPENCLAW_CONFIG_UNAME_S:-$(uname -s 2>/dev/null || echo unknown)}
  uname_m=${OPENCLAW_CONFIG_UNAME_M:-$(uname -m 2>/dev/null || echo unknown)}

  os=$(normalize_os "$uname_s") || die "暂不支持的系统: $uname_s"
  arch=$(normalize_arch "$uname_m") || die "暂不支持的架构: $uname_m"
  asset_name=$(asset_name_for "$os" "$arch") || die "暂不支持的平台组合: $os/$arch"
  command_name=$(command_name_for "$os")
  install_dir=$(choose_install_dir "$os" "$command_name")
  latest_tag=$(fetch_latest_tag)

  tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t openclaw-config-install)
  trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

  mkdir -p "$install_dir" || die "无法创建安装目录: $install_dir"

  tmp_asset="$tmp_dir/$asset_name"
  target_path="$install_dir/$command_name"

  log "检测到平台: $os/$arch"
  log "准备安装版本: $latest_tag"
  log "安装目录: $install_dir"

  download_asset "$latest_tag" "$asset_name" "$tmp_asset"
  chmod 755 "$tmp_asset" >/dev/null 2>&1 || true
  mv "$tmp_asset" "$target_path" || die "无法写入 $target_path"

  clear_macos_quarantine "$target_path"
  print_success "$install_dir" "$target_path" "$command_name" "$latest_tag"
}

if [ "${OPENCLAW_CONFIG_INSTALL_TESTING:-0}" != "1" ]; then
  main "$@"
fi
