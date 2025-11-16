# DeepSeek API Token 优化方案

## ⚠️ 重要说明：DeepSeek API 不支持内置记忆功能

**DeepSeek API 本身不支持跨会话的记忆功能**。每次 API 调用都是独立的，必须发送完整的上下文（System Prompt + User Prompt）。

## 🔍 实际情况分析

### 当前 Token 使用情况

**每次 API 调用**:
- System Prompt: ~2000-5000 tokens（取决于模板）
- User Prompt: ~500-2000 tokens（取决于持仓和市场数据）
- **总计**: ~2500-7000 tokens/次

### Token 消耗来源

1. **System Prompt（最大部分）**:
   - 提示词模板内容（~1000-3000 tokens）
   - 风险控制规则（~500-1000 tokens）
   - JSON格式说明（~300-500 tokens）
   - 账户净值、杠杆配置（~50-100 tokens）

2. **User Prompt**:
   - 账户信息（~100 tokens）
   - 持仓信息（~50-200 tokens/持仓）
   - 市场数据（~50-100 tokens/币种）
   - 候选币种（~30-50 tokens/币种）

## 💡 可行的优化方案

### 方案1：使用精简模板（✅ 已实现，推荐）

**效果**: 可以减少 30-50% 的 System Prompt token

**实施**:
```go
// 在配置中默认使用"精简"模板
templateName := "精简" // 而不是 "default" 或 "激进"
```

**Token 节省**:
- 完整模板: ~3000 tokens
- 精简模板: ~1500 tokens
- **节省**: ~1500 tokens/次（约50%）

### 方案2：优化 System Prompt 内容（✅ 部分实现）

**当前优化**:
- ✅ 使用精简模板减少冗余
- ✅ 动态调整内容（根据账户净值）
- ✅ 移除小账户特殊说明

**进一步优化建议**:
1. **压缩JSON格式说明**: 使用更简洁的示例
2. **简化风险控制说明**: 只保留关键规则
3. **移除重复内容**: 检查提示词模板中的重复说明

### 方案3：优化 User Prompt 内容（✅ 已实现）

**当前优化**:
- ✅ 精简市场数据展示
- ✅ 只显示关键指标
- ✅ 动态调整候选币种数量

**进一步优化**:
1. **减少市场数据字段**: 只显示最关键的指标
2. **压缩持仓信息**: 只显示必要字段
3. **限制候选币种数量**: 根据持仓数量动态调整

### 方案4：System Prompt 构建缓存（🔄 可实施）

**原理**: 虽然 DeepSeek API 不支持记忆，但可以缓存 System Prompt 的构建结果，避免重复构建。

**实现**:
```go
// 在 decision/engine.go 中添加缓存
var systemPromptCache = make(map[string]string)
var cacheMutex sync.RWMutex

func buildSystemPromptWithCache(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string) string {
    // 生成缓存键（基于关键参数）
    cacheKey := fmt.Sprintf("%.2f_%d_%d_%s", accountEquity, btcEthLeverage, altcoinLeverage, templateName)
    
    // 检查缓存
    cacheMutex.RLock()
    if cached, exists := systemPromptCache[cacheKey]; exists {
        cacheMutex.RUnlock()
        return cached
    }
    cacheMutex.RUnlock()
    
    // 构建 System Prompt
    systemPrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName)
    
    // 存入缓存
    cacheMutex.Lock()
    systemPromptCache[cacheKey] = systemPrompt
    cacheMutex.Unlock()
    
    return systemPrompt
}
```

**效果**: 
- 不减少 token 使用（仍需发送完整 System Prompt）
- 但可以减少 CPU 开销（避免重复构建）
- 适合账户净值不经常变化的场景

## 📊 Token 优化效果估算

### 当前情况（完整模板）

- System Prompt: ~3000 tokens
- User Prompt: ~1000 tokens
- **总计**: ~4000 tokens/次
- **每日成本**（5分钟间隔，288次）: ~1,152,000 tokens/天

### 使用精简模板后

- System Prompt: ~1500 tokens（节省50%）
- User Prompt: ~1000 tokens
- **总计**: ~2500 tokens/次
- **每日成本**: ~720,000 tokens/天
- **节省**: ~432,000 tokens/天（约37.5%）

### 进一步优化 User Prompt（减少30%）

- System Prompt: ~1500 tokens
- User Prompt: ~700 tokens（节省30%）
- **总计**: ~2200 tokens/次
- **每日成本**: ~633,600 tokens/天
- **节省**: ~518,400 tokens/天（约45%）

## 🎯 推荐实施步骤

### 第一步：立即优化（✅ 已完成）

1. ✅ 默认使用"精简"模板
2. ✅ 优化 System Prompt 内容
3. ✅ 精简 User Prompt 展示

### 第二步：进一步优化（建议实施）

1. **添加 System Prompt 构建缓存**
   - 基于账户净值和杠杆配置
   - 减少重复构建开销

2. **优化提示词模板内容**
   - 检查并移除冗余说明
   - 压缩JSON格式示例
   - 简化风险控制说明

3. **动态调整 User Prompt 长度**
   - 根据账户状态调整详细程度
   - 无持仓时简化显示
   - 持仓较多时只显示关键信息

### 第三步：监控和调优（长期）

1. **添加 Token 使用统计**
   - 记录每次调用的 token 使用量
   - 分析哪些部分消耗最多
   - 针对性优化

2. **A/B 测试不同模板**
   - 比较不同模板的 token 使用和决策质量
   - 找到最佳平衡点

## ⚠️ 重要限制

### DeepSeek API 不支持的功能

1. **❌ 跨会话记忆**: 每次调用都是独立的
2. **❌ 引用机制**: 不能引用之前发送的内容
3. **❌ 上下文压缩**: 必须发送完整上下文

### 因此，优化方向应该是

1. **✅ 减少 Prompt 长度**: 精简内容，移除冗余
2. **✅ 优化 Prompt 结构**: 更高效的信息组织
3. **✅ 智能内容选择**: 根据场景动态调整详细程度
4. **✅ 缓存构建结果**: 避免重复构建（虽然仍需发送）

## 📝 总结

### DeepSeek API 记忆功能

**答案**: ❌ DeepSeek API **不支持内置记忆功能**

### 如何减少 Token 使用

1. **使用精简模板**（✅ 已实现，可节省 ~37.5%）
2. **优化 Prompt 内容**（✅ 部分实现，可再节省 ~10-20%）
3. **动态调整内容**（✅ 已实现，根据账户状态调整）
4. **System Prompt 缓存**（🔄 可实施，减少构建开销，但不减少 token）

### 最佳实践

1. **默认使用"精简"模板**
2. **定期检查和优化 Prompt 内容**
3. **监控 Token 使用情况**
4. **根据实际情况调整策略**

---

**文档版本**: V1.0  
**最后更新**: 2025-11-08






