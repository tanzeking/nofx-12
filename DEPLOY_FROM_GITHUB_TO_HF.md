# 🚀 从 GitHub 仓库部署到 Hugging Face Spaces 教程

本教程将指导您如何直接从 GitHub 仓库 `tanzeking/nofx-12` 部署到 Hugging Face Spaces。

---

## 📋 方法一：使用 GitHub 仓库连接（推荐）

### 步骤 1: 创建 Hugging Face Space

1. **访问 Hugging Face Spaces**
   - 打开：https://huggingface.co/spaces
   - 登录您的账号

2. **创建新 Space**
   - 点击右上角 **"Create new Space"**
   - 填写信息：
     ```
     Space name: your-username/nofx-trading
     （例如：tanzeking/nofx-trading）
     
     SDK: 选择 Docker
     
     Hardware: CPU basic（免费）或更高
     
     Visibility: Public 或 Private
     ```
   - 点击 **"Create Space"**

### 步骤 2: 从 GitHub 仓库同步代码

**重要说明**：Hugging Face Spaces 不直接支持连接 GitHub 仓库，需要通过 Git 推送的方式同步代码。

**方法 A：使用 Git 推送（推荐）**

1. **在本地克隆您的 GitHub 仓库**
   ```bash
   git clone https://github.com/tanzeking/nofx-12.git
   cd nofx-12
   ```

2. **添加 Hugging Face Space 作为远程仓库**
   ```bash
   # 格式：https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME
   git remote add hf https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME
   
   # 例如：
   # git remote add hf https://huggingface.co/spaces/tanzeking/nofx-trading
   ```

3. **推送代码到 Hugging Face**
   ```bash
   git push hf main
   ```

**方法 B：在 Space 中直接上传文件**

1. **在 Space 页面**
   - 点击 **"Files and versions"** 标签
   - 点击 **"Add file"** → **"Upload files"**
   - 上传所有项目文件

2. **或者使用 Web 界面创建文件**
   - 逐个创建必需的文件
   - 复制 GitHub 仓库中的内容

### 步骤 3: 准备 Dockerfile

由于您的仓库中有 `Dockerfile.hf`，需要将其设置为 `Dockerfile`：

**方式 A: 在 GitHub 仓库中重命名**
```bash
# 在本地仓库
git mv Dockerfile.hf Dockerfile
git commit -m "Rename Dockerfile.hf to Dockerfile for HF deployment"
git push origin main
```

**方式 B: 在 Hugging Face Space 中创建**
- 在 Space 的 "Files" 标签中
- 点击 "Add file" → "Create new file"
- 文件名：`Dockerfile`
- 复制 `Dockerfile.hf` 的内容

### 步骤 4: 配置环境变量

1. **在 Space Settings 中**
   - 点击 **"Variables and secrets"**
   - 添加以下变量：

| 变量名 | 值 | 说明 |
|--------|-----|------|
| `NOFX_ADMIN_PASSWORD` | `your-secure-password` | 管理员密码（必需） |
| `NOFX_TIMEZONE` | `Asia/Shanghai` | 时区设置 |
| `AI_MAX_TOKENS` | `4000` | AI 最大 token 数 |

2. **点击 "Save"**

### 步骤 5: 触发构建

1. **自动构建**
   - 连接仓库后，Hugging Face 会自动开始构建
   - 如果使用 GitHub 连接，每次推送代码会自动重新构建

2. **手动构建**
   - 在 Space 页面点击 **"Rebuild"** 按钮

### 步骤 6: 等待构建完成

- 构建通常需要 **5-15 分钟**
- 可以在 Space 页面的 **"Logs"** 标签查看构建进度
- 构建完成后，Space 会自动启动

---

## 📋 方法二：直接上传文件（简单快速）

如果不想连接 GitHub，可以直接上传文件：

### 步骤 1: 创建 Space

同方法一的步骤 1

### 步骤 2: 准备文件

在本地准备以下文件：

```bash
# 1. 克隆或下载您的仓库
git clone https://github.com/tanzeking/nofx-12.git
cd nofx-12

# 2. 复制 Dockerfile
cp Dockerfile.hf Dockerfile

# 3. 确认以下文件存在：
#    - Dockerfile
#    - nginx.hf.conf
#    - main.go
#    - go.mod
#    - web/package.json
#    - config.json.example
#    - 所有源代码文件
```

### 步骤 3: 上传到 Space

1. **在 Space 页面**
   - 点击 **"Files and versions"** 标签
   - 点击 **"Add file"** → **"Upload files"**

2. **上传文件**
   - 选择整个项目文件夹
   - 或逐个上传必需文件
   - 确保 `Dockerfile` 在根目录

3. **提交更改**
   - 点击 **"Commit changes"**

### 步骤 4: 配置环境变量

同方法一的步骤 4

### 步骤 5: 等待构建

同方法一的步骤 5-6

---

## 🔧 重要配置说明

### Dockerfile 配置

确保 Space 根目录有 `Dockerfile`：

```dockerfile
# 如果使用 Dockerfile.hf，需要重命名
# 或者在 Space 中创建符号链接
```

### 必需文件清单

确保以下文件在 Space 中：

```
Space 根目录/
├── Dockerfile              # ✅ 必需（从 Dockerfile.hf 复制）
├── nginx.hf.conf           # ✅ 必需
├── main.go                 # ✅ 必需
├── go.mod                  # ✅ 必需
├── go.sum                  # ✅ 必需
├── config.json.example     # ✅ 必需
├── web/                    # ✅ 必需（前端代码）
│   ├── package.json
│   └── src/
├── prompts/                # ✅ 必需
└── ... (其他源代码文件)
```

### 环境变量配置

**必需的环境变量：**

```bash
NOFX_ADMIN_PASSWORD=your-password-here
```

**可选的环境变量：**

```bash
NOFX_TIMEZONE=Asia/Shanghai
AI_MAX_TOKENS=4000
```

---

## 🔄 自动部署设置（GitHub 连接）

### 启用自动部署

1. **在 Space Settings 中**
   - 找到 **"Repository"** 部分
   - 确保已连接 GitHub 仓库
   - 选择自动部署分支（通常是 `main`）

2. **自动触发**
   - 每次推送到 GitHub 仓库
   - Hugging Face 会自动检测更改
   - 自动触发重新构建

### 手动触发构建

- 在 Space 页面点击 **"Rebuild"** 按钮
- 或在 Space Settings 中点击 **"Trigger rebuild"**

---

## 📝 快速检查清单

部署前检查：

- [ ] Hugging Face Space 已创建
- [ ] GitHub 仓库已连接（或文件已上传）
- [ ] `Dockerfile` 存在于根目录
- [ ] 所有必需文件已上传
- [ ] 环境变量已配置
- [ ] 构建已开始

部署后检查：

- [ ] 构建成功完成
- [ ] Space 状态为 "Running"
- [ ] 可以访问 Web 界面
- [ ] API 健康检查通过：`/api/health`
- [ ] 数据持久化正常（`/data` 目录）

---

## 🐛 常见问题

### 问题 1: Dockerfile 未找到

**症状**：构建失败，提示找不到 Dockerfile

**解决**：
1. 确认 `Dockerfile` 在 Space 根目录
2. 如果只有 `Dockerfile.hf`，需要重命名：
   ```bash
   # 在 Space Files 中重命名
   # 或上传时直接命名为 Dockerfile
   ```

### 问题 2: 构建超时

**症状**：构建时间过长或超时

**解决**：
1. 检查网络连接
2. 减少 Dockerfile 中的构建步骤
3. 使用更快的硬件配置（CPU basic+）

### 问题 3: 端口错误

**症状**：应用无法访问

**解决**：
1. 确认使用端口 7860（Hugging Face 默认）
2. 检查 `nginx.hf.conf` 配置
3. 查看日志确认端口绑定

### 问题 4: 环境变量未生效

**症状**：配置的环境变量没有生效

**解决**：
1. 确认在 Space Settings → Variables 中配置
2. 重启 Space（点击 "Rebuild"）
3. 检查变量名拼写是否正确

---

## 🎯 推荐工作流程

### 日常开发流程

```bash
# 1. 在本地开发
git add .
git commit -m "Your changes"
git push origin main

# 2. Hugging Face 自动检测并构建
# （如果已连接 GitHub 仓库）

# 3. 等待构建完成，测试新功能
```

### 手动更新流程

```bash
# 1. 在本地更新代码
git push origin main

# 2. 在 Hugging Face Space 中
# 点击 "Rebuild" 手动触发构建
```

---

## 📚 相关资源

- **您的 GitHub 仓库**: https://github.com/tanzeking/nofx-12
- **Hugging Face Spaces**: https://huggingface.co/spaces
- **详细部署教程**: `DEPLOY_HF_TUTORIAL.md`
- **快速部署指南**: `一键部署指南.md`

---

## 💡 提示

1. **首次部署**：建议使用方法二（直接上传），更简单直接
2. **后续更新**：使用方法一（GitHub 连接），自动部署更方便
3. **测试环境**：可以先创建 Private Space 进行测试
4. **监控日志**：定期查看构建和运行日志，及时发现问题

---

**版本**: V1.77  
**最后更新**: 2025-11-09

