@echo off
rem 一键构建：安装前端依赖 -> Vite 构建 -> 同步到 embed 目录 -> 编译单文件服务端
setlocal
cd /d "%~dp0"
node scripts\build.mjs
if errorlevel 1 (
  echo.
  echo 构建失败，请查看上面的错误信息。
  exit /b 1
)
echo.
echo 完成：menu.exe，双击或在终端运行即可。
exit /b 0
