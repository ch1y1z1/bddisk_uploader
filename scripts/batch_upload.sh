#!/bin/bash

# 批量上传脚本
# 使用方法: ./batch_upload.sh <目录路径>

set -e

# 检查参数
if [ $# -eq 0 ]; then
    echo "使用方法: $0 <目录路径>"
    echo "示例: $0 ~/Documents"
    exit 1
fi

# 检查目录是否存在
SOURCE_DIR="$1"
if [ ! -d "$SOURCE_DIR" ]; then
    echo "错误: 目录 '$SOURCE_DIR' 不存在"
    exit 1
fi

# 检查上传工具是否存在
UPLOADER="./bddisk_uploader"
if [ ! -f "$UPLOADER" ]; then
    echo "错误: 找不到上传工具 '$UPLOADER'"
    echo "请确保在项目根目录下运行此脚本"
    exit 1
fi

# 检查配置文件
if [ ! -f "config.json" ]; then
    echo "错误: 找不到配置文件 config.json"
    echo "请先运行: $UPLOADER -init"
    exit 1
fi

echo "🚀 开始批量上传..."
echo "源目录: $SOURCE_DIR"
echo ""

# 统计信息
total_files=0
success_files=0
failed_files=0
failed_list=""

# 遍历目录中的所有文件
find "$SOURCE_DIR" -type f | while IFS= read -r file; do
    total_files=$((total_files + 1))
    
    # 获取相对路径作为远程文件名
    relative_path="${file#$SOURCE_DIR/}"
    
    echo "[$total_files] 上传: $relative_path"
    
    # 执行上传
    if $UPLOADER -file "$file" -name "$relative_path"; then
        success_files=$((success_files + 1))
        echo "✅ 成功: $relative_path"
    else
        failed_files=$((failed_files + 1))
        failed_list="$failed_list\n  - $relative_path"
        echo "❌ 失败: $relative_path"
    fi
    
    echo ""
done

# 显示统计结果
echo "📊 上传完成!"
echo "总文件数: $total_files"
echo "成功: $success_files"
echo "失败: $failed_files"

if [ $failed_files -gt 0 ]; then
    echo ""
    echo "失败的文件:"
    echo -e "$failed_list"
    exit 1
fi