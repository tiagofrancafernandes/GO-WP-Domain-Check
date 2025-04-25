#!/bin/bash

if [ ! -d './output/macos' ]; then
    mkdir -p ./output/macos
    echo "Created output/macos directory"
fi

echo "Building for MacOS Darwin amd64"

GOOS=darwin GOARCH=amd64 go build -o ./output/macos/wordpress-checker-darwin-amd64 main.go
