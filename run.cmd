@echo off
setlocal

set "RUN_PS1_URL=https://cnb.cool/dmxapi/openclaw_config/-/git/raw/main/run.ps1"
set "RUN_PS1_PATH=%TEMP%\openclaw-config-run-%RANDOM%%RANDOM%.ps1"
set "OPENCLAW_CONFIG_EXIT_PROCESS=1"

curl.exe -fsSL "%RUN_PS1_URL%" -o "%RUN_PS1_PATH%"
if errorlevel 1 (
  echo 运行失败: 下载 run.ps1 失败 1>&2
  exit /b 1
)

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%RUN_PS1_PATH%"
set "RUN_EXIT_CODE=%ERRORLEVEL%"

del "%RUN_PS1_PATH%" >nul 2>nul
exit /b %RUN_EXIT_CODE%
