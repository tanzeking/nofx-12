#!/bin/bash
# Hugging Face Spaces 一键部署脚本
# 使用方法: ./deploy-hf.sh

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║    🚀 NOFX - Hugging Face Spaces 一键部署脚本            ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 检查是否在 Git 仓库中
if [ ! -d ".git" ]; then
    echo "❌ 错误: 当前目录不是 Git 仓库"
    echo "请先初始化 Git 仓库: git init"
    exit 1
fi

# 检查 Dockerfile.hf 是否存在
if [ ! -f "Dockerfile.hf" ]; then
    echo "❌ 错误: Dockerfile.hf 不存在"
    exit 1
fi

# 检查 nginx.hf.conf 是否存在
if [ ! -f "nginx.hf.conf" ]; then
    echo "❌ 错误: nginx.hf.conf 不存在"
    exit 1
fi

echo "📋 步骤 1: 准备部署文件..."
# 复制 Dockerfile.hf 为 Dockerfile（如果不存在）
if [ ! -f "Dockerfile" ] || [ "Dockerfile.hf" -nt "Dockerfile" ]; then
    cp Dockerfile.hf Dockerfile
    echo "✅ 已复制 Dockerfile.hf -> Dockerfile"
fi

# 检查 .gitignore 是否忽略 Dockerfile
if ! grep -q "^Dockerfile$" .gitignore 2>/dev/null; then
    echo "⚠️  警告: .gitignore 中没有忽略 Dockerfile"
    echo "   建议添加 'Dockerfile' 到 .gitignore（Hugging Face 会自动生成）"
fi

echo ""
echo "📋 步骤 2: 检查必需文件..."
REQUIRED_FILES=(
    "Dockerfile"
    "nginx.hf.conf"
    "main.go"
    "go.mod"
    "web/package.json"
    "config.json.example"
)

MISSING_FILES=()
for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$file" ] && [ ! -d "$file" ]; then
        MISSING_FILES+=("$file")
    fi
done

if [ ${#MISSING_FILES[@]} -gt 0 ]; then
    echo "❌ 缺少必需文件:"
    for file in "${MISSING_FILES[@]}"; do
        echo "   - $file"
    done
    exit 1
fi

echo "✅ 所有必需文件已就绪"

echo ""
echo "📋 步骤 3: 检查 Git 状态..."
if [ -n "$(git status --porcelain)" ]; then
    echo "⚠️  有未提交的更改:"
    git status --short
    echo ""
    read -p "是否继续部署? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ 部署已取消"
        exit 1
    fi
fi

echo ""
echo "📋 步骤 4: 部署说明"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "🚀 一键部署到 Hugging Face Spaces"
echo ""
echo "1️⃣  创建 Space:"
echo "   📍 访问: https://huggingface.co/spaces"
echo "   📍 点击 'Create new Space'"
echo "   📍 Space name: your-username/nofx-trading"
echo "   📍 SDK: 选择 Docker"
echo "   📍 Hardware: CPU basic 或更高"
echo "   📍 点击 'Create Space'"
echo ""
echo "2️⃣  上传代码（选择一种方式）:"
echo ""
echo "   方式 A - Git 推送（推荐）:"
echo "   git remote add hf https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME"
echo "   git push hf main"
echo ""
echo "   方式 B - Web 上传:"
echo "   - 在 Space 页面点击 'Files and versions'"
echo "   - 点击 'Add file' → 'Upload files'"
echo "   - 上传所有文件（包括 Dockerfile）"
echo ""
echo "3️⃣  配置环境变量:"
echo "   📍 在 Space 页面点击 'Settings'"
echo "   📍 找到 'Variables and secrets'"
echo "   📍 添加以下变量:"
echo "      • NOFX_ADMIN_PASSWORD = your-secure-password"
echo "      • NOFX_TIMEZONE = Asia/Shanghai"
echo "      • AI_MAX_TOKENS = 4000"
echo ""
echo "4️⃣  等待构建:"
echo "   📍 构建需要 5-15 分钟"
echo "   📍 构建完成后自动启动"
echo "   📍 访问 Space URL 即可使用"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📚 详细教程请查看: DEPLOY_HF_TUTORIAL.md"
echo "📚 快速指南请查看: 一键部署指南.md"
echo ""

# 检查是否已配置 Hugging Face remote
if git remote | grep -q "hf"; then
    HF_REMOTE=$(git remote get-url hf 2>/dev/null || echo "")
    if [ -n "$HF_REMOTE" ]; then
        echo "✅ 检测到 Hugging Face remote: $HF_REMOTE"
        echo ""
        read -p "是否立即推送到 Hugging Face? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            echo ""
            echo "📤 推送到 Hugging Face..."
            git add Dockerfile nginx.hf.conf
            git commit -m "Deploy to Hugging Face Spaces" || true
            git push hf main || git push hf master
            echo ""
            echo "✅ 推送完成！"
            echo "   请访问您的 Space 页面查看构建状态"
        fi
    fi
else
    echo "💡 提示: 添加 Hugging Face remote:"
    echo "   git remote add hf https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME"
fi

echo ""
echo "📚 更多信息请查看: README.hf.md"
echo ""
echo "✅ 部署准备完成！"

