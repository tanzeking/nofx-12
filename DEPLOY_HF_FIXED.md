# 🚀 从 GitHub 仓库部署到 Hugging Face - 正确方法

## ⚠️ 重要说明

Hugging Face Spaces **不支持直接连接 GitHub 仓库**，需要通过以下方式同步代码：

---

## ✅ 方法一：使用 Git 推送（推荐，最简单）

### 步骤 1: 创建 Hugging Face Space

1. 访问 https://huggingface.co/spaces
2. 点击 **"Create new Space"**
3. 填写信息：
   - Space name: `your-username/nofx-trading`
   - SDK: **Docker**
   - Hardware: CPU basic
4. 点击 **"Create Space"**

### 步骤 2: 从 GitHub 仓库推送代码

```bash
# 1. 克隆您的 GitHub 仓库
git clone https://github.com/tanzeking/nofx-12.git
cd nofx-12

# 2. 确保有 Dockerfile（如果没有，复制 Dockerfile.hf）
cp Dockerfile.hf Dockerfile

# 3. 添加 Hugging Face Space 作为远程仓库
# 格式：https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME
git remote add hf https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME

# 例如：
# git remote add hf https://huggingface.co/spaces/tanzeking/nofx-trading

# 4. 推送代码到 Hugging Face
git push hf main
```

### 步骤 3: 配置环境变量

在 Space Settings → Variables 中添加：
- `NOFX_ADMIN_PASSWORD` = 您的密码
- `NOFX_TIMEZONE` = `Asia/Shanghai`
- `AI_MAX_TOKENS` = `4000`

### 步骤 4: 等待构建

- Hugging Face 会自动检测代码并开始构建
- 构建需要 5-15 分钟
- 构建完成后即可使用

---

## ✅ 方法二：直接上传文件

### 步骤 1: 创建 Space

同上

### 步骤 2: 准备文件

```bash
# 克隆仓库
git clone https://github.com/tanzeking/nofx-12.git
cd nofx-12

# 复制 Dockerfile
cp Dockerfile.hf Dockerfile
```

### 步骤 3: 上传到 Space

1. 在 Space 页面，点击 **"Files and versions"**
2. 点击 **"Add file"** → **"Upload files"**
3. 选择整个项目文件夹或必需文件
4. 点击 **"Commit changes"**

### 步骤 4: 配置环境变量

同上

---

## 🔧 修复 Dockerfile 平台错误

如果遇到 "no match for platform in manifest" 错误：

1. **确保使用正确的镜像版本**
   - Go: `1.21-alpine`（不是 1.25，可能不存在）
   - Node: `20-alpine`
   - Alpine: `3.19`（指定版本，不用 latest）

2. **添加平台参数**
   ```dockerfile
   FROM --platform=linux/amd64 node:20-alpine AS web-builder
   FROM --platform=linux/amd64 alpine:3.19 AS ta-lib-builder
   FROM --platform=linux/amd64 golang:1.21-alpine AS backend-builder
   FROM --platform=linux/amd64 alpine:3.19
   ```

3. **已修复的 Dockerfile.hf**
   - 已更新所有 FROM 命令
   - 添加了 `--platform=linux/amd64`
   - 使用稳定的镜像版本

---

## 📝 完整操作流程

```bash
# 1. 克隆 GitHub 仓库
git clone https://github.com/tanzeking/nofx-12.git
cd nofx-12

# 2. 准备 Dockerfile
cp Dockerfile.hf Dockerfile

# 3. 添加 Hugging Face remote
git remote add hf https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME

# 4. 推送代码
git push hf main

# 5. 在 Hugging Face Space 中配置环境变量
# 6. 等待构建完成
```

---

## 🔄 后续更新

每次更新代码后：

```bash
# 1. 推送到 GitHub
git push origin main

# 2. 推送到 Hugging Face
git push hf main

# Hugging Face 会自动重新构建
```

---

## 🐛 常见错误解决

### 错误 1: "no match for platform in manifest"

**原因**：镜像版本不存在或平台不匹配

**解决**：
- 使用已修复的 `Dockerfile.hf`（已添加 `--platform=linux/amd64`）
- 确保镜像版本存在（Go 1.21 而不是 1.25）

### 错误 2: "找不到 Connect repository"

**原因**：Hugging Face Spaces 不支持直接连接 GitHub

**解决**：使用 Git 推送方式（方法一）

### 错误 3: "Permission denied"

**原因**：没有推送权限

**解决**：
- 确认 Space 名称正确
- 确认您有该 Space 的写权限
- 检查 Hugging Face 账号权限

---

## 📚 相关文档

- **详细教程**: `DEPLOY_HF_TUTORIAL.md`
- **GitHub 部署**: `DEPLOY_FROM_GITHUB_TO_HF.md`
- **快速指南**: `一键部署指南.md`

---

**版本**: V1.77  
**最后更新**: 2025-11-09


