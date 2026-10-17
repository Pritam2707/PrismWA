@echo off
echo ====================================================
echo        Building PrismBot for Android / Termux
echo ====================================================

set CGO_ENABLED=0
set GOOS=linux
set GOLANG_PROTOBUF_REGISTRATION_CONFLICT=ignore
set LDFLAGS=-s -w -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore

echo.
echo [1/2] Compiling for ARM64 (aarch64 - Standard modern Android/Termux)...
set GOARCH=arm64
go build -ldflags="%LDFLAGS%" -o prismbot-termux-arm64 .
if %ERRORLEVEL% neq 0 (
    echo [ERROR] ARM64 build failed with exit code %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)
copy /y prismbot-termux-arm64 prismbot-termux >nul
echo [OK] Built: prismbot-termux (ARM64)

echo.
echo [2/2] Compiling for ARMv7 (32-bit ARM)...
set GOARCH=arm
set GOARM=7
go build -ldflags="%LDFLAGS%" -o prismbot-termux-armv7 .
if %ERRORLEVEL% neq 0 (
    echo [WARNING] ARMv7 build had an issue, but ARM64 succeeded.
) else (
    echo [OK] Built: prismbot-termux-armv7
)

echo.
echo ====================================================
echo Build Complete!
echo Transfer 'prismbot-termux' and '.env' (plus dashboard folder) to Termux:
echo.
echo In Termux:
echo   pkg update ^&^& pkg install -y ffmpeg ca-certificates
echo   chmod +x prismbot-termux
echo   ./prismbot-termux
echo ====================================================
