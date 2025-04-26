#!/bin/bash

if [ ! -d './output/macos' ]; then
    mkdir -p ./output/macos
    echo "Created output/macos directory"
fi

echo "Building for MacOS Darwin arm64 [Apple Silicon (M1/M2)]"

GOOS=darwin GOARCH=arm64 go build -o ./output/macos/wordpress-checker-darwin-arm64 main.go
