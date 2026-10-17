@echo off
set CGO_ENABLED=0
set GOLANG_PROTOBUF_REGISTRATION_CONFLICT=ignore
go build -ldflags="-s -w -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore" -o prismbot.exe .

if %ERRORLEVEL% equ 0 (
    echo Build successful: prismbot.exe
) else (
    echo Build failed with code %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)
