@echo off
echo ========================================================
echo Running E2E Test Suite for LMS PKBM Tunas Ilmu
echo ========================================================

echo.
echo [1/3] Running Backend Unit & 4-Tier E2E Test Suite...
cd /d "%~dp0\..\backend\cmd\server"
go test -v ./...
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Backend E2E Tests Failed!
    exit /b %ERRORLEVEL%
)

echo.
echo [2/3] Verifying Frontend TypeScript & Vite Production Build...
cd /d "%~dp0\..\frontend"
call cmd /c npm run build
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Frontend Build Failed!
    exit /b %ERRORLEVEL%
)

echo.
echo ========================================================
echo [SUCCESS] ALL E2E TESTS & BUILD VERIFICATIONS PASSED!
echo ========================================================
