# 🚀 NOFX 一键部署到 Hugging Face Spaces 教程

本教程将指导您如何将 NOFX AI 交易系统一键部署到 Hugging Face Spaces。

---

## 📋 前置要求

在开始之前，请确保您有：

- ✅ [Hugging Face 账号](https://huggingface.co/join)（免费注册）
- ✅ Git 已安装（可选，用于代码推送）
- ✅ 基本的命令行操作知识

---

## 🎯 方法一：使用部署脚本（推荐）

### 步骤 1: 准备代码

```bash
# 1. 进入项目目录
cd nofx

# 2. 确保所有文件已提交到 Git（可选但推荐）
git add .
git commit -m "Prepare for Hugging Face deployment"
```

### 步骤 2: 运行部署脚本

```bash
# 在 Linux/Mac 上
chmod +x deploy-hf.sh
./deploy-hf.sh

# 在 Windows 上（使用 Git Bash 或 WSL）
bash deploy-hf.sh
```

脚本会自动：
- ✅ 检查必需文件
- ✅ 准备部署文件
- ✅ 提供部署说明

### 步骤 3: 按照脚本提示操作

脚本会显示详细的部署步骤，按照提示完成即可。

---

## 🎯 方法二：手动部署（详细步骤）

### 步骤 1: 创建 Hugging Face Space

1. **访问 Hugging Face Spaces**
   - 打开浏览器，访问：https://huggingface.co/spaces
   - 登录您的账号

2. **创建新 Space**
   - 点击右上角的 **"Create new Space"** 按钮
   - 填写以下信息：
     ```
     Space name: your-username/nofx-trading
     （例如：john/nofx-trading）
     
     SDK: 选择 Docker
     
     Hardware: 选择 CPU basic（免费）或更高配置
     
     Visibility: Public（公开）或 Private（私有）
     ```

3. **点击 "Create Space"**

### 步骤 2: 准备部署文件

在您的本地项目目录中：

```bash
# 1. 复制 Dockerfile.hf 为 Dockerfile
cp Dockerfile.hf Dockerfile

# 2. 确认以下文件存在：
#    - Dockerfile
#    - nginx.hf.conf
#    - main.go
#    - go.mod
#    - web/package.json
#    - config.json.example
```

### 步骤 3: 上传代码到 Space

#### 方式 A: 使用 Git（推荐）

```bash
# 1. 添加 Hugging Face remote
git remote add hf https://huggingface.co/spaces/YOUR_USERNAME/YOUR_SPACE_NAME

# 例如：
# git remote add hf https://huggingface.co/spaces/john/nofx-trading

# 2. 推送代码
git push hf main
# 或者
git push hf master
```

#### 方式 B: 使用 Web 界面上传

1. 在 Space 页面，点击 **"Files and versions"** 标签
2. 点击 **"Add file"** → **"Upload files"**
3. 拖拽或选择以下文件上传：
   - `Dockerfile`（从 `Dockerfile.hf` 复制）
   - `nginx.hf.conf`
   - 所有源代码文件（或整个项目文件夹）

### 步骤 4: 配置环境变量

1. 在 Space 页面，点击 **"Settings"** 标签
2. 找到 **"Variables and secrets"** 部分
3. 点击 **"New variable"**，添加以下变量：

| 变量名 | 值 | 说明 |
|--------|-----|------|
| `NOFX_ADMIN_PASSWORD` | `your-secure-password` | 管理员密码（如果启用 admin_mode） |
| `NOFX_TIMEZONE` | `Asia/Shanghai` | 时区设置 |
| `AI_MAX_TOKENS` | `4000` | AI 响应的最大 token 数 |

**重要提示**：
- `NOFX_ADMIN_PASSWORD` 是必需的（如果启用管理员模式）
- 密码应该足够复杂，不要使用简单密码
- 这些变量是私密的，不会公开显示

### 步骤 5: 等待构建完成

1. 上传代码后，Hugging Face 会自动开始构建
2. 在 Space 页面可以看到构建进度
3. 构建通常需要 **5-15 分钟**（取决于网络和服务器负载）
4. 构建完成后，Space 会自动启动

### 步骤 6: 访问您的应用

构建完成后：

1. Space 页面会显示 **"Running"** 状态
2. 点击页面上的 **"App"** 标签
3. 或者直接访问：`https://YOUR_USERNAME-nofx-trading.hf.space`

---

## 🔧 配置说明

### 首次启动配置

1. **访问 Web 界面**
   - 打开您的 Space URL
   - 系统会自动创建默认配置

2. **配置 AI 模型**
   - 点击 "AI Models" 菜单
   - 添加您的 DeepSeek 或 Qwen API 密钥
   - 测试连接是否正常

3. **配置交易所**
   - 点击 "Exchanges" 菜单
   - 添加交易所 API 密钥（Binance/OKX/Hyperliquid/Aster）
   - 选择是否使用测试网

4. **创建交易员**
   - 点击 "Traders" 菜单
   - 创建新的交易员
   - 选择 AI 模型和交易所
   - 配置交易参数

### 存储说明

所有数据自动保存在 Hugging Face 的持久化存储中：

```
/data/
├── config.db              # 数据库（配置、交易数据）
├── config.json            # 配置文件
└── decision_logs/         # 决策日志
    └── {trader_id}/
        └── decision_*.json
```

**重要**：
- ✅ 这些数据在 Space 重启后**不会丢失**
- ✅ 数据会一直保留，直到您手动删除 Space
- ✅ 建议定期备份重要配置

---

## 🐛 常见问题排查

### 问题 1: 构建失败

**症状**：构建过程中出现错误

**解决方法**：
1. 检查构建日志中的错误信息
2. 确认 `Dockerfile` 文件是否正确
3. 确认所有必需文件都已上传
4. 检查 `go.mod` 和 `package.json` 是否正确

**常见错误**：
- `Dockerfile not found` → 确保 `Dockerfile.hf` 已复制为 `Dockerfile`
- `Module not found` → 检查 `go.mod` 文件
- `npm install failed` → 检查 `web/package.json` 文件

### 问题 2: 应用无法启动

**症状**：构建成功但应用无法运行

**解决方法**：
1. 查看运行日志（Space 页面的 "Logs" 标签）
2. 检查环境变量是否配置正确
3. 确认端口配置（应该是 7860）

**常见错误**：
- `Port already in use` → 检查是否有其他服务占用端口
- `Database error` → 检查 `/data` 目录权限
- `Missing environment variable` → 检查环境变量配置

### 问题 3: API 无法访问

**症状**：前端可以访问，但 API 请求失败

**解决方法**：
1. 检查后端服务是否启动（查看日志）
2. 确认 Nginx 配置正确
3. 检查 API 路由是否正确

**测试方法**：
```bash
# 在浏览器中访问
https://YOUR_SPACE.hf.space/api/health

# 应该返回：{"status":"ok"}
```

### 问题 4: 配置丢失

**症状**：重启后配置丢失

**解决方法**：
1. 确认使用了 `/data` 目录（持久化存储）
2. 检查环境变量 `NOFX_DB_PATH` 是否正确
3. 查看启动日志确认数据库路径

---

## 📊 监控和维护

### 查看日志

1. **构建日志**
   - Space 页面 → "Logs" 标签
   - 查看构建过程的详细信息

2. **运行日志**
   - Space 页面 → "Logs" 标签
   - 查看应用运行时的日志
   - 容器内的日志：`/app/logs/nofx.log`

3. **决策日志**
   - 在 Web 界面查看
   - 或访问：`/data/decision_logs/{trader_id}/`

### 更新部署

当代码更新后：

```bash
# 1. 提交更改
git add .
git commit -m "Update to V1.76"

# 2. 推送到 Hugging Face
git push hf main

# 3. Hugging Face 会自动重新构建
```

或者在 Space 页面点击 **"Rebuild"** 按钮。

### 备份数据

重要数据建议定期备份：

```bash
# 1. 下载数据库文件
# 在 Space 的 "Files" 标签中下载 /data/config.db

# 2. 导出配置
# 在 Web 界面导出配置 JSON
```

---

## 🔒 安全建议

### 1. API 密钥管理

- ✅ 使用 Hugging Face **Secrets** 存储敏感信息
- ❌ 不要在代码中硬编码 API 密钥
- ❌ 不要将 `config.json` 提交到公开仓库

### 2. 访问控制

- ✅ 启用 `admin_mode` 保护 API
- ✅ 使用强密码
- ✅ 定期更换密码

### 3. 网络安全

- ✅ 使用 HTTPS（Hugging Face 自动提供）
- ✅ 配置 CORS 策略
- ✅ 限制 API 访问频率

---

## 📝 快速检查清单

部署前检查：

- [ ] Hugging Face 账号已创建
- [ ] Space 已创建（SDK: Docker）
- [ ] `Dockerfile` 已准备（从 `Dockerfile.hf` 复制）
- [ ] 所有必需文件已上传
- [ ] 环境变量已配置
- [ ] API 密钥已准备好

部署后检查：

- [ ] 构建成功完成
- [ ] 应用可以访问
- [ ] API 健康检查通过
- [ ] 可以登录/注册
- [ ] 可以配置 AI 模型
- [ ] 可以配置交易所
- [ ] 数据持久化正常

---

## 🎉 完成！

恭喜！您已经成功将 NOFX 部署到 Hugging Face Spaces。

### 下一步

1. **配置系统**
   - 添加 AI 模型 API 密钥
   - 配置交易所
   - 创建交易员

2. **开始交易**
   - 启动交易员
   - 监控交易状态
   - 查看决策日志

3. **优化配置**
   - 根据实际情况调整参数
   - 优化 AI 提示词
   - 调整风险控制参数

### 获取帮助

- 📖 查看 [README.hf.md](README.hf.md) 了解更多详情
- 🐛 遇到问题？查看 [故障排除指南](../docs/guides/TROUBLESHOOTING.zh-CN.md)
- 💬 加入社区讨论

---

## 📚 相关链接

- [Hugging Face Spaces 文档](https://huggingface.co/docs/hub/spaces)
- [NOFX 主文档](../README.md)
- [Docker 部署指南](../docs/getting-started/docker-deploy.zh-CN.md)

---

**版本**: V1.76  
**最后更新**: 2025-11-09

