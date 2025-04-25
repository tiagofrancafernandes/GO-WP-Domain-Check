#!/bin/bash

if [ ! -d './output/linux' ]; then
    mkdir -p ./output/linux
    echo "Created output/linux directory"
fi

echo "Building for Linux"

GOOS=linux GOARCH=amd64 go build -o ./output/linux/wordpress-checker-linux-amd64 main.go
