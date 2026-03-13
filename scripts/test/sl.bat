@echo off
chcp 65001 > nul 2>&1
setlocal enabledelayedexpansion

set URL=http://localhost:8080/fGKUg3Ng
set QPS=100
set DURATION=10

echo 测试短链接访问日志机制...
echo 目标URL: %URL%
echo 测试QPS: %QPS%
echo 测试时长: %DURATION% 秒
echo.

set /a TOTAL_REQUESTS=QPS*DURATION
set SUCCESS_COUNT=0
set FAIL_COUNT=0
set START_TIME=%time%

echo 开始测试 [%START_TIME%]
echo.

if exist temp_results.txt del temp_results.txt

for /l %%s in (1,1,%DURATION%) do (
    echo [第 %%s 秒] 发送 %QPS% 个请求...
    
    for /l %%i in (1,1,%QPS%) do (
        curl -H "X-Real-IP: 128.176.224.255" --connect-timeout 5 --max-time 5 "%URL%" >nul 2>&1
        if !errorlevel! equ 0 (
            echo SUCCESS >> temp_results.txt
            set /a SUCCESS_COUNT+=1
            echo   [!SUCCESS_COUNT!] 成功
        ) else (
            echo FAIL >> temp_results.txt
            set /a FAIL_COUNT+=1
            echo   [!FAIL_COUNT!] 失败
        )
    )
    
    echo 本秒完成，等待下一秒...
    echo.
)

set END_TIME=%time%

echo ========== 测试结果 ==========
echo 开始时间: %START_TIME%
echo 结束时间: %END_TIME%
echo 总请求数: %TOTAL_REQUESTS%
echo 成功请求: %SUCCESS_COUNT%
echo 失败请求: %FAIL_COUNT%
if %SUCCESS_COUNT% gtr 0 (
    set /a ACTUAL_QPS=SUCCESS_COUNT/DURATION
    echo 实际QPS: !ACTUAL_QPS!
)

if exist temp_results.txt del temp_results.txt
pause