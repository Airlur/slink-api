.PHONY: all build run test clean help gen gen-module list-sql

# 终端乱码输入 chcp 65001 【目前已修改 IDE 的settings.json，默认激活 UTF-8 编码 | 若需要切换回 GBK 编码，输入 chcp 936】

# 默认目标
all: build

# 构建应用
build:
	@echo "🏗️  构建应用中..."
	go build -o bin/server cmd/main.go
	@echo "✅ 构建完成: bin/server"

# 运行应用
run:
	@echo "🚀 启动应用..."
	go run cmd/main.go

# 测试
test:
	@echo "🧪 运行测试..."
	go test ./... -v

# 清理（Windows 用 rmdir/del 命令，兼容 PowerShell 和 cmd）
clean:
	@echo "🧹 清理构建文件..."
	@if exist bin (rmdir /s /q bin)
	@echo "✅ 清理完成"

# -----------------------------------------------------------------------------
# 代码生成清理相关命令
# -----------------------------------------------------------------------------
# 清理指定模块的生成文件 (兼容 Windows)
# 使用方法: make clean-module MODULE=shortlink
.PHONY: clean-module
clean-module:
	@if not defined MODULE ( \
		echo "❌错误: 请指定模块名称. 示例: make clean-module MODULE=shortlink"; \
		exit /b 1; \
	)
	@echo "⚠️ 即将清理模块 [$(MODULE)] 的以下文件:"
	@echo "  - internal\model\$(MODULE).go"
	@echo "  - internal\dto\$(MODULE).go"
	@echo "  - internal\repository\$(MODULE).go"
	@echo "  - internal\service\$(MODULE).go"
	@echo "  - internal\api\v1\$(MODULE).go"
	@set /p "CONFIRM=确认要删除以上文件吗? (输入 y 继续，其他键取消): "
	@if /i not "%CONFIRM%"=="y" ( \
		echo "❌ 已取消清理操作"; \
		exit /b 0; \
	)
	@echo "🧹 开始清理模块 [$(MODULE)] 的生成文件..."
	@del /f /q internal\model\$(MODULE).go 2>nul
	@del /f /q internal\dto\$(MODULE).go 2>nul
	@del /f /q internal\repository\$(MODULE).go 2>nul
	@del /f /q internal\service\$(MODULE).go 2>nul
	@del /f /q internal\api\v1\$(MODULE).go 2>nul
	@echo "✅ 模块 [$(MODULE)] 清理完成."
# -----------------------------------------------------------------------------
# [可选但推荐] 为常用模块增加清理快捷方式
# -----------------------------------------------------------------------------
.PHONY: clean-shortlink clean-tag clean-share
clean-shortlink:
	make clean-module MODULE=shortlink
clean-tag:
	make clean-module MODULE=tag
clean-share:
	make clean-module MODULE=share

# 显示帮助信息
help:
	@echo "📖 可用命令:"
	@echo "  make build                  构建应用"
	@echo "  make run                    运行应用"
	@echo "  make test                   运行测试||未实现"
	@echo "  make clean                  清理构建文件"
	@echo "  make gen MODULE=name        根据建表SQL生成指定模块代码"
	@echo "  make list-sql               列出所有可用的SQL模板文件"
	@echo ""
	@echo "📝 使用示例:"
	@echo "  make gen MODULE=shortlink   (普通方式) 生成 shortlink 模块"
	@echo "  make gen-shortlink          (快捷方式) 生成 shortlink 模块"
	@echo "  make clean-shortlink        (快捷方式) 清理 shortlink 模块"

# 代码生成相关配置（供 list-sql 使用）
SQL_DIR := scripts\sql  # 这里必须是 \，和 bat 脚本保持一致
DEFAULT_MODULE := shortlink

# 列出所有可用的SQL文件（Make 语法，兼容 Windows）
list-sql:
	@echo "📁 可用的SQL模板文件:"
	@if exist $(SQL_DIR) ( \
		for %%f in ($(SQL_DIR)\*.sql) do ( \
			for /f "delims=" %%i in ("%%~nf") do echo   - %%i \
		) \
	) else ( \
		echo "❌ SQL目录不存在: $(SQL_DIR)" \
	)

# 核心修改：gen 目标仅调用批处理脚本，避免语法冲突
# MODULE 变量会自动作为环境变量传入 bat 脚本
gen:
	@echo "开始调用代码生成脚本..."
	scripts\gen_module.bat  # 直接调用批处理脚本

# 快捷目标（保持不变，仍传递 MODULE 变量）
gen-shortlink:
	make gen MODULE=shortlink

gen-tag:
	make gen MODULE=tag

gen-share:
	make gen MODULE=share