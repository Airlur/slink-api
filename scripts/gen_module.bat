@echo off
setlocal enabledelayedexpansion

:: 检查 MODULE 环境变量是否传入（Make 会把 MODULE=xxx 转为环境变量）
if not defined MODULE (
    echo "❌ 请指定模块名称"
    echo "Usage: make gen MODULE=module_name"
    echo "Example: make gen MODULE=shortlink"
    echo.
    echo "可用的模块:"
    :: 调用 make list-sql 显示可用模块
    make list-sql
    exit /b 1
)

:: 定义 SQL 目录（和 Makefile 中的 SQL_DIR 保持一致）
set "SQL_DIR=scripts\sql"
set "OUTPUT_DIR=."

:: 检查 SQL 文件是否存在
if not exist "!SQL_DIR!\!MODULE!.sql" (
    echo "❌ SQL文件不存在: !SQL_DIR!\!MODULE!.sql"
    echo.
    echo "可用的模块:"
    make list-sql
    exit /b 1
)

:: 执行代码生成器（调用 Go 脚本）
echo "🔄 生成 !MODULE! 模块代码..."
go run cmd\generator\main.go -sql "!SQL_DIR!\!MODULE!.sql" -module !MODULE! -output !OUTPUT_DIR!

:: 检查生成结果
if %errorlevel% equ 0 (
    echo "✅ !MODULE! 模块代码生成完成!"
) else (
    echo "❌ !MODULE! 模块代码生成失败!"
    exit /b 1
)

endlocal