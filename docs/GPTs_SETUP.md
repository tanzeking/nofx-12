# GPTs (Assistant API) 使用说明

## 概述

GPTs (Assistant API) 是 OpenAI 提供的自定义 GPT 功能，允许您创建具有特定指令和知识库的 AI 助手。本系统支持通过 Assistant API 调用您的自定义 GPTs。

## 如何获取 Assistant ID

### 方法 1: 通过 OpenAI API 获取

1. 登录 OpenAI 平台：https://platform.openai.com
2. 进入 "Assistants" 页面：https://platform.openai.com/assistants
3. 找到您要使用的 GPTs，点击进入详情页
4. 在详情页中，您会看到 Assistant ID（格式类似：`asst_xxxxxxxxxxxxx`）

### 方法 2: 通过 ChatGPT 网站获取

1. 访问您的 GPTs 链接（例如：https://chatgpt.com/g/g-p-691083be9c3c8191ba384a08c4d50900-tou-zi/project）
2. 打开浏览器开发者工具（F12）
3. 在 Network 标签中，找到对 OpenAI API 的请求
4. 从请求中提取 Assistant ID

### 方法 3: 通过 API 列出所有 Assistants

```bash
curl https://api.openai.com/v1/assistants \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "OpenAI-Beta: assistants=v2"
```

## 配置步骤

### 1. 在 Web 界面配置 GPTs

1. 进入系统配置页面
2. 找到 "AI模型" 部分
3. 找到 "GPTs (Assistant API)" 模型
4. 启用该模型
5. 填写以下信息：
   - **API Key**: 您的 OpenAI API 密钥
   - **Custom Model Name**: 您的 GPTs Assistant ID（例如：`asst_xxxxxxxxxxxxx`）
   - **Custom API URL**: 可选，留空则使用默认 URL（`https://api.openai.com/v1`）
   - **Thread ID**: 可选，如果留空则每次创建新的 Thread；如果填写则复用现有 Thread（用于保持对话上下文）

### 2. 创建交易员时选择 GPTs

1. 创建新的交易员
2. 在 "AI模型" 下拉菜单中选择 "GPTs (Assistant API)"
3. 系统会自动使用您配置的 GPTs

## 工作原理

1. **Thread 管理**：
   - 如果未设置 Thread ID，每次调用会创建新的 Thread
   - 如果设置了 Thread ID，会复用现有 Thread，保持对话上下文

2. **消息流程**：
   - 系统会将 system prompt 和 user prompt 合并发送给 GPTs
   - GPTs 会根据其配置的指令和知识库生成响应
   - 系统解析响应并执行交易决策

3. **响应格式**：
   - GPTs 的响应格式应与系统期望的 JSON 格式一致
   - 系统会尝试从响应中提取交易决策信息

## 注意事项

1. **API 密钥权限**：确保您的 API 密钥具有访问 Assistant API 的权限
2. **Assistant 配置**：确保您的 GPTs 配置了正确的指令，能够理解交易相关的提示词
3. **响应格式**：GPTs 的响应应该包含 JSON 格式的交易决策，格式如下：
   ```json
   [
     {
       "action": "open_long",
       "symbol": "BTCUSDT",
       "leverage": 10,
       "position_size_usd": 100,
       "stop_loss": 50000,
       "take_profit": 55000,
       "confidence": 85,
       "reasoning": "基于技术分析，BTC 呈现上升趋势"
     }
   ]
   ```
4. **Thread 复用**：如果使用 Thread ID 复用 Thread，GPTs 会记住之前的对话上下文，这有助于保持决策的连续性
5. **成本考虑**：Assistant API 的使用会产生费用，请关注您的 API 使用情况

## 故障排除

### 问题：无法获取 Assistant ID

**解决方案**：
- 确认您已登录 OpenAI 平台
- 确认您的 GPTs 已创建并可用
- 尝试通过 API 列出所有 Assistants

### 问题：API 调用失败

**解决方案**：
- 检查 API 密钥是否正确
- 检查 Assistant ID 是否正确
- 检查网络连接是否正常
- 查看系统日志以获取详细错误信息

### 问题：响应格式不正确

**解决方案**：
- 检查 GPTs 的指令配置
- 确保 GPTs 理解系统期望的响应格式
- 在 GPTs 的指令中添加 JSON 格式要求

## 参考资料

- [OpenAI Assistant API 文档](https://platform.openai.com/docs/assistants/overview)
- [OpenAI API 参考](https://platform.openai.com/docs/api-reference/assistants)




