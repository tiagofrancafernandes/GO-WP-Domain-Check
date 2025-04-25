#!/bin/bash

if [ ! -d './output/windows' ]; then
    mkdir -p ./output/windows
    echo "Created output/windows directory"
fi

echo "Building for Windows amd64"

GOOS=windows GOARCH=amd64 go build -o ./output/windows/wordpress-checker-windows-amd64.exe main.go
