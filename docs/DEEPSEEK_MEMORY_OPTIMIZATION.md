# DeepSeek API 记忆功能与Token优化方案

## 📌 DeepSeek API 记忆功能说明

### 1. DeepSeek API 本身不支持内置记忆功能

DeepSeek API 是 OpenAI 兼容的 API，**不支持跨会话的内置记忆功能**（不像 Claude 那样有独立的记忆系统）。

### 2. 但可以通过应用层实现记忆功能

虽然 DeepSeek API 本身不支持记忆，但可以通过在 `messages` 数组中传递历史对话来实现上下文记忆，从而减少 token 使用。

## 🔍 当前实现分析

### 当前代码问题

**位置**: `nofx/mcp/client.go:139-209`

每次调用都发送完整的：
- **System Prompt**: 包含完整的交易规则、风险控制、JSON格式说明等（约2000-5000 tokens）
- **User Prompt**: 包含当前市场数据、账户状态等（约500-2000 tokens）

**问题**:
- System Prompt 大部分内容是固定的（交易规则、JSON格式等），但每次都完整发送
- 账户净值和杠杆配置变化时，System Prompt 才会变化
- 导致大量重复的 token 消耗

## 💡 优化方案

### 方案1：System Prompt 缓存（推荐）

**原理**: System Prompt 在账户净值和杠杆配置不变时，内容基本固定，可以缓存并复用。

**实现步骤**:

1. **在 MCP Client 中添加 System Prompt 缓存**
2. **检测 System Prompt 是否变化**（通过哈希或关键参数）
3. **如果 System Prompt 未变化，使用简化的引用消息**
4. **只在 System Prompt 变化时发送完整内容**

**优点**:
- 显著减少 token 使用（System Prompt 通常占 60-80% 的输入 token）
- 实现简单，风险低
- 不影响交易逻辑

**缺点**:
- 需要管理缓存状态
- 账户净值变化时需要重新发送

### 方案2：分层 System Prompt

**原理**: 将 System Prompt 分为：
- **静态部分**（交易规则、JSON格式等）：只在首次发送或变化时发送
- **动态部分**（账户净值、杠杆配置等）：每次发送

**实现步骤**:

1. **分离静态和动态 System Prompt**
2. **首次调用发送完整的 System Prompt**
3. **后续调用只发送动态部分 + 引用静态部分的消息**

**优点**:
- Token 节省更明显
- 更灵活

**缺点**:
- 实现复杂
- 需要确保 AI 能理解引用

### 方案3：使用精简模板（已实现）

**当前已有**: `templateName == "精简"` 模板

**效果**: 精简模板可以减少约 30-50% 的 token 使用

**建议**: 
- 默认使用"精简"模板
- 只在需要详细说明时使用完整模板

## 🚀 推荐实现：System Prompt 缓存

### 实现代码示例

```go
// 在 mcp.Client 结构体中添加
type Client struct {
    // ... 现有字段
    cachedSystemPrompt string  // 缓存的 System Prompt
    cachedSystemHash   string  // System Prompt 的哈希值
}

// 修改 CallWithMessages 方法
func (client *Client) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
    // 计算 System Prompt 的哈希值
    systemHash := calculateHash(systemPrompt)
    
    // 如果 System Prompt 未变化，使用缓存策略
    if client.cachedSystemHash == systemHash {
        // 使用简化的消息：只发送变化的部分
        // 或者发送一个引用消息
        return client.callWithCachedSystem(systemPrompt, userPrompt)
    }
    
    // System Prompt 变化，更新缓存并正常调用
    client.cachedSystemPrompt = systemPrompt
    client.cachedSystemHash = systemHash
    return client.callOnce(systemPrompt, userPrompt)
}
```

### 更简单的优化：检测关键参数

由于 System Prompt 主要变化来自：
- `accountEquity` (账户净值)
- `btcEthLeverage` (BTC/ETH杠杆)
- `altcoinLeverage` (山寨币杠杆)

可以基于这些参数生成缓存键：

```go
// 在 decision/engine.go 中
func buildSystemPromptWithCache(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string) (string, string) {
    // 生成缓存键
    cacheKey := fmt.Sprintf("%.2f_%d_%d_%s", accountEquity, btcEthLeverage, altcoinLeverage, templateName)
    
    // 检查缓存（可以使用内存缓存或Redis）
    if cached, exists := systemPromptCache.Get(cacheKey); exists {
        return cached.(string), cacheKey
    }
    
    // 构建 System Prompt
    systemPrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName)
    
    // 存入缓存
    systemPromptCache.Set(cacheKey, systemPrompt, time.Hour)
    
    return systemPrompt, cacheKey
}
```

## 📊 Token 节省估算

### 当前情况（每次完整发送）

- System Prompt: ~3000 tokens
- User Prompt: ~1000 tokens
- **总计**: ~4000 tokens/次

### 使用缓存后（System Prompt 缓存）

- System Prompt: ~3000 tokens（首次），0 tokens（后续，如果未变化）
- User Prompt: ~1000 tokens
- **总计**: ~4000 tokens（首次），~1000 tokens（后续）
- **节省**: 约 75% 的 token（假设 System Prompt 不经常变化）

### 使用精简模板

- System Prompt: ~1500 tokens（精简版）
- User Prompt: ~1000 tokens
- **总计**: ~2500 tokens/次
- **节省**: 约 37.5% 的 token

## 🎯 最佳实践建议

### 1. 短期优化（立即实施）

1. **默认使用"精简"模板**
   - 修改 `buildSystemPrompt` 默认使用 "精简" 模板
   - 可以减少 30-50% 的 token

2. **优化 System Prompt 内容**
   - 移除重复说明
   - 简化示例
   - 压缩冗长描述

### 2. 中期优化（1-2周实施）

1. **实现 System Prompt 缓存**
   - 基于账户净值和杠杆配置生成缓存键
   - 缓存 System Prompt 内容
   - 只在关键参数变化时重新构建

2. **添加 Token 使用监控**
   - 记录每次调用的 token 使用量
   - 分析哪些部分消耗最多 token
   - 针对性优化

### 3. 长期优化（1个月+实施）

1. **实现分层 System Prompt**
   - 静态部分缓存
   - 动态部分实时构建
   - 使用引用机制

2. **智能 Prompt 压缩**
   - 根据上下文自动选择详细程度
   - 动态调整 Prompt 长度

## 📝 注意事项

### 1. DeepSeek API 限制

- **最大上下文长度**: 通常为 32k 或 64k tokens（取决于模型）
- **不支持跨会话记忆**: 每次 API 调用都是独立的
- **Token 计费**: 输入和输出 token 都计费

### 2. 交易场景的特殊性

- **市场数据实时变化**: User Prompt 每次都必须更新
- **账户状态变化**: 账户净值、持仓等每次都可能变化
- **决策独立性**: 每次决策都是基于当前状态，不太适合使用历史对话

### 3. 缓存策略

- **缓存失效条件**: 
  - 账户净值变化
  - 杠杆配置变化
  - 模板变化
  - 自定义 Prompt 变化

- **缓存时间**: 
  - 建议使用内存缓存，交易员实例生命周期内有效
  - 不需要持久化（每次重启可以重建）

## 🔧 快速实施建议

### 第一步：立即优化（5分钟）

修改默认模板为"精简"：

```go
// 在 buildSystemPrompt 中
if templateName == "" {
    templateName = "精简" // 改为默认使用精简模板
}
```

### 第二步：添加缓存（30分钟）

```go
// 在 mcp.Client 中添加
var systemPromptCache = sync.Map{} // 简单的内存缓存

func (client *Client) CallWithCachedSystem(systemPrompt, userPrompt string, cacheKey string) (string, error) {
    // 检查是否已有缓存的 System Prompt
    if cached, ok := systemPromptCache.Load(cacheKey); ok {
        // 使用缓存，只发送 User Prompt + 简化的引用
        // 或者继续发送完整 System Prompt（DeepSeek 会处理）
        // 注意：DeepSeek 不支持真正的"引用"，所以还是需要发送完整内容
        // 但可以优化：只在关键参数变化时重新发送
    }
    // ...
}
```

### 第三步：监控 Token 使用（1小时）

添加 Token 使用统计：

```go
// 在 API 响应中解析 token 使用量
type APIResponse struct {
    Usage struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage"`
    // ...
}

// 记录和统计
func (client *Client) logTokenUsage(usage APIResponse.Usage) {
    log.Printf("📊 Token使用: 输入=%d, 输出=%d, 总计=%d", 
        usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}
```

## 📚 参考资料

- [DeepSeek API 文档](https://platform.deepseek.com/api-docs/)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat/create)
- [Token 优化最佳实践](https://platform.openai.com/docs/guides/prompt-engineering)

---

**文档版本**: V1.0  
**最后更新**: 2025-11-08






