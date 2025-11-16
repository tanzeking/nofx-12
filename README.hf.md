# NOFX - Hugging Face Spaces 部署指南

本指南将帮助您将 NOFX AI 交易系统一键部署到 Hugging Face Spaces。

## 🚀 快速开始

### 1. 创建 Hugging Face Space

1. 访问 [Hugging Face Spaces](https://huggingface.co/spaces)
2. 点击 "Create new Space"
3. 填写信息：
   - **Space name**: `your-username/nofx-trading`
   - **SDK**: 选择 `Docker`
   - **Hardware**: 建议选择 `CPU basic` 或更高
   - **Visibility**: 选择 `Public` 或 `Private`

### 2. 上传代码

将以下文件上传到 Space：

- `Dockerfile.hf` (重命名为 `Dockerfile`)
- `nginx.hf.conf` (重命名为 `nginx.hf.conf`)
- 所有源代码文件

或者使用 Git：

```bash
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx
# 将 Dockerfile.hf 复制为 Dockerfile
cp Dockerfile.hf Dockerfile
# 提交到 Hugging Face Space
```

### 3. 配置环境变量

在 Hugging Face Space 的 Settings → Variables 中添加：

| 变量名 | 说明 | 示例值 |
|--------|------|--------|
| `NOFX_ADMIN_PASSWORD` | 管理员密码（如果启用 admin_mode） | `your-secure-password` |
| `NOFX_TIMEZONE` | 时区 | `Asia/Shanghai` |
| `AI_MAX_TOKENS` | AI 最大 token 数 | `4000` |

### 4. 配置存储

Hugging Face Spaces 提供持久化存储（`/data` 目录），以下数据会自动持久化：

- ✅ **数据库**: `/data/config.db` - 所有配置和交易数据
- ✅ **决策日志**: `/data/decision_logs/` - AI 决策记录
- ✅ **配置文件**: `/data/config.json` - 系统配置

**重要**: 这些数据在 Space 重启后仍然保留。

## 📁 存储说明

### 持久化存储路径

系统会自动检测 Hugging Face 环境，并使用以下路径：

```
/data/
├── config.db              # SQLite 数据库（所有配置）
├── config.json            # 配置文件（如果存在）
└── decision_logs/         # 决策日志目录
    └── {trader_id}/       # 每个交易员的日志
        └── decision_*.json
```

### 环境变量配置

系统支持以下环境变量来配置存储路径：

- `NOFX_DB_PATH`: 数据库文件路径（默认: `/data/config.db`）
- `NOFX_LOG_DIR`: 日志目录路径（默认: `/data/decision_logs`）
- `HF_HOME`: Hugging Face 数据目录（自动检测）

## 🔧 配置说明

### 首次启动

1. 系统会自动从 `config.json.example` 创建默认配置
2. 访问 Web 界面进行配置：
   - 添加 AI 模型（DeepSeek/Qwen API 密钥）
   - 添加交易所（Binance/OKX/Hyperliquid/Aster）
   - 创建交易员

### 端口配置

- **前端**: `http://your-space.hf.space/`
- **API**: `http://your-space.hf.space/api`
- **健康检查**: `http://your-space.hf.space/health`

Hugging Face Spaces 自动使用 **7860** 端口，无需手动配置。

## 📊 监控和日志

### 查看日志

1. **Hugging Face 控制台**: 在 Space 页面查看构建和运行日志
2. **应用日志**: 容器内的 `/app/logs/nofx.log`
3. **决策日志**: `/data/decision_logs/{trader_id}/`

### 健康检查

系统提供健康检查端点：

```bash
curl http://your-space.hf.space/api/health
```

## ⚠️ 注意事项

### 1. 资源限制

- Hugging Face Spaces 有 CPU/内存限制
- 建议使用 `CPU basic` 或更高配置
- 长时间运行的交易任务可能受限制

### 2. 存储限制

- 免费版有存储空间限制
- 定期清理旧的决策日志
- 数据库文件会持续增长，注意监控

### 3. 安全性

- **不要**在代码中硬编码 API 密钥
- 使用 Hugging Face Secrets 存储敏感信息
- 启用 `admin_mode` 保护 API 端点

### 4. 网络访问

- 确保 AI API（DeepSeek/Qwen）可以从 Hugging Face 访问
- 某些地区可能需要代理

## 🔄 更新部署

### 方法 1: Git 推送

```bash
git add .
git commit -m "Update to V1.75"
git push
```

Hugging Face 会自动重新构建。

### 方法 2: Web 界面

在 Space 页面点击 "Rebuild" 按钮。

## 🐛 故障排除

### 问题 1: 构建失败

**检查**:
- Dockerfile 语法是否正确
- 所有依赖文件是否上传
- 构建日志中的错误信息

### 问题 2: 数据库无法写入

**解决**:
- 确保 `/data` 目录有写权限
- 检查存储空间是否充足

### 问题 3: API 无法访问

**检查**:
- 端口是否正确（7860）
- Nginx 配置是否正确
- 后端服务是否启动

### 问题 4: 配置丢失

**解决**:
- 检查 `/data/config.db` 是否存在
- 确认使用了持久化存储路径
- 查看启动日志确认路径

## 📝 版本信息

- **当前版本**: V1.75
- **Hugging Face 支持**: ✅ 完整支持
- **持久化存储**: ✅ 自动配置

## 🔗 相关链接

- [Hugging Face Spaces 文档](https://huggingface.co/docs/hub/spaces)
- [NOFX 主文档](../README.md)
- [Docker 部署指南](../docs/getting-started/docker-deploy.zh-CN.md)

## 💡 提示

1. **首次部署**: 建议先使用测试 API 密钥
2. **监控**: 定期检查 Space 的运行状态和日志
3. **备份**: 重要配置建议定期导出备份
4. **优化**: 根据实际使用情况调整资源配置

---

**需要帮助？** 查看 [故障排除指南](../docs/guides/TROUBLESHOOTING.zh-CN.md) 或提交 Issue。

