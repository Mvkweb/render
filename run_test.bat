@echo off
echo Building the server and client...
go build -o build/render-server.exe ./cmd/server
go build -o build/render-client.exe ./cmd/client

echo Starting server and client in separate windows...
start "Render Server" cmd /k ""%~dp0build\render-server.exe""
timeout /t 3 >nul
start "Render Client" cmd /k ""%~dp0build\render-client.exe""

echo.
echo Server and client are running in separate windows.