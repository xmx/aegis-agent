#!/bin/bash

# 安装 goversioninfo 命令
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest

# 生成全平台的 syso 文件
goversioninfo -platform-specific

# 将 syso 文件放到 main.go 同级目录
mv *.syso ../../main
