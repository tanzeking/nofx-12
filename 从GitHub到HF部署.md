# 🚀 从 GitHub 仓库部署到 Hugging Face Spaces

## ⚡ 最简单的方法（3 步完成）

### 第 1 步：创建 Space 并连接 GitHub

1. **访问 Hugging Face Spaces**
   - 打开：https://huggingface.co/spaces
   - 点击 **"Create new Space"**

2. **填写 Space 信息**
   ```
   Space name: your-username/nofx
   SDK: Docker
   Hardware: CPU basic
   ```

3. **从 GitHub 同步代码**

   **使用 Git 推送：**
   ```bash
   # 克隆仓库
   git clone https://github.com/tanzeking/nofx-12.git
   cd nofx-12
   
   # 添加 Hugging Face Space 作为远程仓库
   git remote add hf https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME
   
   # 推送代码
   git push hf main
   ```
   
   **或者直接在 Space 中上传文件：**
   - 在 Space 页面点击 **"Files and versions"**
   - 点击 **"Add file"** → **"Upload files"**
   - 上传所有项目文件

### 第 2 步：准备 Dockerfile

**在 GitHub 仓库中添加 Dockerfile：**

```bash
# 克隆您的仓库
git clone https://github.com/tanzeking/nofx-12.git
cd nofx-12

# 复制 Dockerfile.hf 为 Dockerfile
cp Dockerfile.hf Dockerfile

# 提交并推送
git add Dockerfile
git commit -m "Add Dockerfile for Hugging Face deployment"
git push origin main
```

**或者在 Hugging Face Space 中直接创建：**
- 在 Space 的 "Files" 标签
- 点击 "Add file" → "Create new file"
- 文件名：`Dockerfile`
- 复制 `Dockerfile.hf` 的内容

### 第 3 步：配置环境变量

在 Space Settings → Variables 中添加：

```
NOFX_ADMIN_PASSWORD = your-password-here
NOFX_TIMEZONE = Asia/Shanghai
AI_MAX_TOKENS = 4000
```

### 完成！

- Hugging Face 会自动检测 GitHub 仓库的更改
- 自动开始构建（5-15 分钟）
- 构建完成后即可访问您的 Space

---

## 🔄 自动部署

连接 GitHub 后，每次您推送代码到 `tanzeking/nofx-12`，Hugging Face 会自动：
1. 检测代码更改
2. 自动触发构建
3. 部署新版本

**无需手动操作！**

---

## 📝 快速检查

- [ ] Space 已创建
- [ ] GitHub 仓库已连接（`tanzeking/nofx-12`）
- [ ] `Dockerfile` 已添加到仓库
- [ ] 环境变量已配置
- [ ] 构建已开始

---

## 🐛 遇到问题？

1. **Dockerfile 未找到**
   - 确保 `Dockerfile` 在仓库根目录
   - 或在 Space 中直接创建

2. **构建失败**
   - 查看 Space 的 "Logs" 标签
   - 检查错误信息

3. **环境变量未生效**
   - 确认在 Settings → Variables 中配置
   - 重启 Space（点击 "Rebuild"）

---

## 📚 详细教程

- **完整教程**: `DEPLOY_FROM_GITHUB_TO_HF.md`
- **快速指南**: `一键部署指南.md`
- **通用教程**: `DEPLOY_HF_TUTORIAL.md`

---

**您的仓库**: https://github.com/tanzeking/nofx-12  
**版本**: V1.77

