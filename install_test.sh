#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

OPENCLAW_CONFIG_INSTALL_TESTING=1
. "$SCRIPT_DIR/install.sh"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  printf 'FAIL: %s\n' "$1" >&2
}

assert_eq() {
  expected=$1
  actual=$2
  message=$3

  if [ "$expected" = "$actual" ]; then
    pass
    return
  fi

  fail "$message (expected: $expected, actual: $actual)"
}

assert_status() {
  expected_status=$1
  message=$2
  shift 2

  if "$@"; then
    status=0
  else
    status=$?
  fi

  assert_eq "$expected_status" "$status" "$message"
}

test_normalize_os() {
  assert_eq "linux" "$(normalize_os Linux)" "normalize linux"
  assert_eq "macos" "$(normalize_os Darwin)" "normalize macos"
  assert_eq "windows" "$(normalize_os MINGW64_NT-10.0)" "normalize mingw windows"
}

test_normalize_arch() {
  assert_eq "amd64" "$(normalize_arch x86_64)" "normalize x86_64"
  assert_eq "amd64" "$(normalize_arch amd64)" "normalize amd64"
  assert_eq "arm64" "$(normalize_arch arm64)" "normalize arm64"
  assert_eq "arm64" "$(normalize_arch aarch64)" "normalize aarch64"
}

test_asset_name_for() {
  assert_eq "openclaw-config-linux-amd64" "$(asset_name_for linux amd64)" "linux amd64 asset"
  assert_eq "openclaw-config-macos-arm64" "$(asset_name_for macos arm64)" "macos arm64 asset"
  assert_eq "openclaw-config-windows-amd64.exe" "$(asset_name_for windows amd64)" "windows amd64 asset"
}

test_extract_latest_tag() {
  html='
<a href="/dmxapi/openclaw_config/-/releases/tag/v1.2.2">v1.2.2</a>
<a href="/dmxapi/openclaw_config/-/releases/tag/v1.2.1">v1.2.1</a>
'
  assert_eq "v1.2.2" "$(printf "%s" "$html" | extract_latest_tag)" "extract first release tag"
}

test_extract_latest_tag_failure() {
  assert_status 1 "missing release tag should fail" sh -c '
    OPENCLAW_CONFIG_INSTALL_TESTING=1
    . "'"$SCRIPT_DIR"'/install.sh"
    printf "%s" "<html></html>" | extract_latest_tag >/dev/null
  '
}

test_normalize_os
test_normalize_arch
test_asset_name_for
test_extract_latest_tag
test_extract_latest_tag_failure

if [ "$FAIL_COUNT" -ne 0 ]; then
  printf '\n%d tests failed, %d passed\n' "$FAIL_COUNT" "$PASS_COUNT" >&2
  exit 1
fi

printf '%d tests passed\n' "$PASS_COUNT"
