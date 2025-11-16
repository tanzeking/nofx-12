# ⚡ 从 GitHub 仓库快速部署到 Hugging Face

## 🎯 3 步完成部署

### 第 1 步：创建 Space

1. 访问 https://huggingface.co/spaces
2. 点击 **"Create new Space"**
3. 填写：
   - Space name: `your-username/nofx`
   - SDK: **Docker**
   - Hardware: CPU basic
4. 点击 **"Create Space"**

### 第 2 步：推送代码

```bash
# 1. 克隆您的 GitHub 仓库
git clone https://github.com/tanzeking/nofx-12.git
cd nofx-12

# 2. 准备 Dockerfile
cp Dockerfile.hf Dockerfile

# 3. 添加 Hugging Face Space 作为远程仓库
# 替换 YOUR_USERNAME 和 YOUR_SPACE_NAME
git remote add hf https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME

# 4. 推送代码
git push hf main
```

### 第 3 步：配置环境变量

在 Space Settings → Variables 添加：
- `NOFX_ADMIN_PASSWORD` = 您的密码

**完成！** 等待构建完成即可使用。

---

## ⚠️ 重要说明

1. **Hugging Face 不支持直接连接 GitHub**
   - 必须使用 Git 推送方式
   - 或直接在 Space 中上传文件

2. **Dockerfile 平台错误已修复**
   - 已添加 `--platform=linux/amd64`
   - 已更新镜像版本（Go 1.21, Alpine 3.19）

3. **后续更新**
   ```bash
   # 推送到 GitHub
   git push origin main
   
   # 推送到 Hugging Face
   git push hf main
   ```

---

## 🐛 如果遇到错误

### "no match for platform"
- ✅ 已修复：使用修复后的 `Dockerfile.hf`
- 确保复制为 `Dockerfile`

### "host not found in upstream 'nofx'"
- ✅ 已修复：Dockerfile.hf 现在会自动删除冲突的默认配置文件
- 确保使用最新的 `Dockerfile.hf`

### "找不到 Connect repository"
- ✅ 正常：Hugging Face 不支持此功能
- 使用 Git 推送方式（第 2 步）

---

**详细教程**: `DEPLOY_HF_FIXED.md`


