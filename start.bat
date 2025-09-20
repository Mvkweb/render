@echo off
echo Building Render server...
go build -o bin\render.exe ./cmd/server

echo Starting Render server...
cls
bin\render.exe