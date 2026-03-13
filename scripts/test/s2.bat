@echo off
chcp 65001 > nul 2>&1
setlocal enabledelayedexpansion

set URL=http://localhost:8080/fGKUg3Ng
set CONCURRENT=200
set DURATION=5

echo 并发限流测试开始: %time%
echo 目标URL: %URL%
echo 并发数: %CONCURRENT%
echo 测试时长: %DURATION% 秒
echo.

REM 清理结果文件
if exist test_results.txt del test_results.txt

REM 创建简单的并发执行脚本
echo @echo off > temp_worker.bat
echo curl -H "X-Real-IP: 121.233.114.255" --connect-timeout 5 --max-time 5 "%URL%" ^>nul 2^>^&1 >> temp_worker.bat
echo if ^^!errorlevel^^! equ 0 (echo SUCCESS >> test_results.txt) else (echo FAIL >> test_results.txt) >> temp_worker.bat

echo 正在启动 %CONCURRENT% 个并发进程...
set START_TIME=%time%

for /l %%i in (1,1,%CONCURRENT%) do (
    start "TestWorker%%i" /min cmd /c "temp_worker.bat"
)

echo 等待 %DURATION% 秒...
timeout /t %DURATION% /nobreak >nul

echo 停止测试进程...
taskkill /f /fi "WINDOWTITLE eq TestWorker*" >nul 2>&1
timeout /t 2 /nobreak >nul

REM 统计结果
set SUCCESS_COUNT=0
set FAIL_COUNT=0

if exist test_results.txt (
    for /f %%a in ('type test_results.txt ^| find /c "SUCCESS"') do set SUCCESS_COUNT=%%a
    for /f %%a in ('type test_results.txt ^| find /c "FAIL"') do set FAIL_COUNT=%%a
)

REM 清理临时文件
del temp_worker.bat >nul 2>&1

set END_TIME=%time%

echo.
echo ========== 测试结果 ==========
echo 开始时间: %START_TIME%
echo 结束时间: %END_TIME%
echo 并发进程数: %CONCURRENT%
echo 测试时长: %DURATION% 秒
echo.
echo 成功请求: %SUCCESS_COUNT%
echo 失败请求: %FAIL_COUNT%
echo 总请求数: %CONCURRENT%
echo.

if %SUCCESS_COUNT% gtr 0 (
    set /a ACTUAL_QPS=SUCCESS_COUNT/DURATION
    echo 平均QPS: !ACTUAL_QPS!
)

if exist test_results.txt (
    echo.
    echo 结果样例:
    type test_results.txt | head -n 10
    del test_results.txt
)

echo.
echo 检查后台日志确认实际处理数量...
pause