# Hugging Face Spaces 必需文件清单

如果您直接在 Hugging Face Spaces 中编辑文件（而不是连接 GitHub 仓库），需要确保以下文件都存在：

## 必需文件列表

### 配置文件
- `Dockerfile` ✅
- `nginx.main.conf` ✅
- `nginx.hf.conf` ✅
- `config.json.example` ✅

### Prompts 目录
- `prompts/default.txt`
- `prompts/adaptive.txt`
- `prompts/adaptive_relaxed.txt`
- `prompts/Hansen.txt`
- `prompts/nof1.txt`
- `prompts/taro_long_prompts.txt`
- `prompts/激进.txt`
- `prompts/精简.txt`

### Go 代码文件
- `go.mod`
- `go.sum`
- `main.go`
- 以及所有其他 `.go` 文件

### Web 前端
- `web/package.json`
- `web/package-lock.json`
- `web/` 目录下的所有前端源代码

## 推荐做法

**最佳方案：连接 GitHub 仓库**

1. 在 Hugging Face Spaces 设置中
2. 选择 "Connect to GitHub"
3. 选择仓库：`tanzeking/nofx-12`
4. 选择分支：`main`
5. 这样所有文件会自动同步

