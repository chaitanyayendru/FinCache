@echo off
REM FinCache Test Script for Windows
REM Tests both Redis protocol and HTTP API functionality

echo 🧪 FinCache Test Suite
echo =====================

REM Configuration
set FINCACHE_HOST=localhost
set FINCACHE_PORT=6379
set API_PORT=8080

REM Test counters
set TESTS_PASSED=0
set TESTS_FAILED=0

echo Starting FinCache tests...

REM Wait for services to be ready
echo Waiting for services to be ready...
timeout /t 5 /nobreak >nul

REM Test Redis protocol
echo.
echo Testing Redis Protocol
echo ----------------------

REM Test PING
redis-cli -h %FINCACHE_HOST% -p %FINCACHE_PORT% PING >nul 2>&1
if %errorlevel% equ 0 (
    echo ✓ PING test passed
    set /a TESTS_PASSED+=1
) else (
    echo ✗ PING test failed
    set /a TESTS_FAILED+=1
)

REM Test SET/GET
redis-cli -h %FINCACHE_HOST% -p %FINCACHE_PORT% SET testkey testvalue >nul 2>&1
if %errorlevel% equ 0 (
    echo ✓ SET test passed
    set /a TESTS_PASSED+=1
) else (
    echo ✗ SET test failed
    set /a TESTS_FAILED+=1
)

redis-cli -h %FINCACHE_HOST% -p %FINCACHE_PORT% GET testkey >nul 2>&1
if %errorlevel% equ 0 (
    echo ✓ GET test passed
    set /a TESTS_PASSED+=1
) else (
    echo ✗ GET test failed
    set /a TESTS_FAILED+=1
)

REM Test HTTP API
echo.
echo Testing HTTP API
echo ----------------

REM Test health endpoint
curl -s http://%FINCACHE_HOST%:%API_PORT%/health >nul 2>&1
if %errorlevel% equ 0 (
    echo ✓ Health check passed
    set /a TESTS_PASSED+=1
) else (
    echo ✗ Health check failed
    set /a TESTS_FAILED+=1
)

REM Test API key operations
curl -s -X POST http://%FINCACHE_HOST%:%API_PORT%/api/v1/keys/testkey -H "Content-Type: application/json" -d "{\"value\":\"testvalue\"}" >nul 2>&1
if %errorlevel% equ 0 (
    echo ✓ API SET test passed
    set /a TESTS_PASSED+=1
) else (
    echo ✗ API SET test failed
    set /a TESTS_FAILED+=1
)

curl -s http://%FINCACHE_HOST%:%API_PORT%/api/v1/keys/testkey >nul 2>&1
if %errorlevel% equ 0 (
    echo ✓ API GET test passed
    set /a TESTS_PASSED+=1
) else (
    echo ✗ API GET test failed
    set /a TESTS_FAILED+=1
)

REM Cleanup
redis-cli -h %FINCACHE_HOST% -p %FINCACHE_PORT% DEL testkey >nul 2>&1
curl -s -X DELETE http://%FINCACHE_HOST%:%API_PORT%/api/v1/keys/testkey >nul 2>&1

REM Summary
echo.
echo Test Summary
echo ------------
echo Tests Passed: %TESTS_PASSED%
echo Tests Failed: %TESTS_FAILED%

if %TESTS_FAILED% equ 0 (
    echo.
    echo 🎉 All tests passed!
    exit /b 0
) else (
    echo.
    echo ❌ Some tests failed!
    exit /b 1
) 