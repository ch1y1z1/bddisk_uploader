#!/bin/bash

# 构建脚本 - 支持多平台交叉编译

set -e

# 项目信息
PROJECT_NAME="bddisk_uploader"
VERSION="1.0.0"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建标志
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

echo "🔨 构建 ${PROJECT_NAME} v${VERSION}"
echo "构建时间: ${BUILD_TIME}"
echo "Git提交: ${GIT_COMMIT}"
echo ""

# 清理之前的构建
echo "🧹 清理旧文件..."
rm -rf dist/
mkdir -p dist/

# 构建函数
build_for_platform() {
    local GOOS=$1
    local GOARCH=$2
    local SUFFIX=$3
    
    echo "📦 构建 ${GOOS}/${GOARCH}..."
    
    local OUTPUT_NAME="${PROJECT_NAME}"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${PROJECT_NAME}.exe"
    fi
    
    local OUTPUT_PATH="dist/${PROJECT_NAME}-${GOOS}-${GOARCH}${SUFFIX}/${OUTPUT_NAME}"
    
    mkdir -p "$(dirname "$OUTPUT_PATH")"
    
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "$LDFLAGS" -o "$OUTPUT_PATH"
    
    # 复制必要文件
    local DIST_DIR="dist/${PROJECT_NAME}-${GOOS}-${GOARCH}${SUFFIX}"
    cp README.md "$DIST_DIR/"
    cp LICENSE "$DIST_DIR/"
    cp CHANGELOG.md "$DIST_DIR/"
    cp config.example.json "$DIST_DIR/"
    
    # 创建压缩包
    cd dist/
    if [ "$GOOS" = "windows" ]; then
        zip -r "${PROJECT_NAME}-${GOOS}-${GOARCH}${SUFFIX}.zip" "${PROJECT_NAME}-${GOOS}-${GOARCH}${SUFFIX}/"
    else
        tar -czf "${PROJECT_NAME}-${GOOS}-${GOARCH}${SUFFIX}.tar.gz" "${PROJECT_NAME}-${GOOS}-${GOARCH}${SUFFIX}/"
    fi
    cd ..
    
    echo "✅ ${GOOS}/${GOARCH} 构建完成"
}

# 支持的平台
echo "🎯 开始多平台构建..."
echo ""

# Linux
build_for_platform "linux" "amd64" ""
build_for_platform "linux" "arm64" ""

# macOS
build_for_platform "darwin" "amd64" ""
build_for_platform "darwin" "arm64" ""

# Windows
build_for_platform "windows" "amd64" ""

# 本地构建
echo "🏠 构建本地版本..."
go build -ldflags "$LDFLAGS" -o "$PROJECT_NAME"
echo "✅ 本地构建完成"

echo ""
echo "🎉 所有构建完成!"
echo ""
echo "📁 发布文件:"
ls -la dist/

echo ""
echo "💡 使用提示:"
echo "  本地测试: ./$PROJECT_NAME -init"
echo "  查看版本: ./$PROJECT_NAME -version"