#!/bin/bash
echo "Building Render server..."
go build -o bin/render ./cmd/server

echo "Starting Render server..."
cls
./bin/render