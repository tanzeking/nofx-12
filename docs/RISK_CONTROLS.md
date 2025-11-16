# 代码风险控制总结

本文档总结归纳了项目中所有代码层面的风险控制逻辑。

## 📍 风险控制代码位置

### 1. 决策验证层 (`nofx/decision/engine.go`)

#### 1.1 `validateDecision` 函数 - 单个决策验证

**位置**: `nofx/decision/engine.go:959-1129`

**验证内容**:

##### ✅ 1.1.1 Action 有效性验证
```go
validActions := map[string]bool{
    "open_long": true, "open_short": true,
    "close_long": true, "close_short": true,
    "update_stop_loss": true, "update_take_profit": true,
    "partial_close": true, "hold": true, "wait": true,
}
```
- **风险控制点**: 只允许预定义的操作，防止非法操作

##### ✅ 1.1.2 杠杆倍数验证
```go
if d.Leverage <= 0 || d.Leverage > maxLeverage {
    return fmt.Errorf("杠杆必须在1-%d之间（%s，当前配置上限%d倍）: %d", ...)
}
```
- **风险控制点**: 
  - 杠杆必须 > 0
  - 杠杆不能超过配置上限（BTC/ETH 和山寨币分别配置）
  - 防止过度杠杆导致爆仓

##### ✅ 1.1.3 仓位价值验证
```go
if d.PositionSizeUSD <= 0 {
    return fmt.Errorf("仓位大小必须大于0: %.2f", d.PositionSizeUSD)
}
```
- **风险控制点**: 仓位价值必须 > 0

##### ✅ 1.1.4 保证金验证（核心风险控制）

**最小保证金验证**:
```go
const minMargin = 1.0 // OKX合约最低保证金要求
marginRequired := d.PositionSizeUSD / float64(d.Leverage)
if marginRequired < minMargin {
    return fmt.Errorf("保证金过小(%.2f USDT)，必须≥%.2f USDT", ...)
}
```
- **风险控制点**: 确保满足交易所最低保证金要求（1 USDT）

**最大保证金验证**:
```go
maxMarginRatio := 0.30 // 最大保证金比例30%
maxMarginAllowed := accountEquity * maxMarginRatio
if marginRequired > maxMarginAllowed {
    return fmt.Errorf("保证金过大(%.2f USDT)，必须≤账户净值30%%(%.2f USDT)以避免爆仓风险", ...)
}
```
- **风险控制点**: 
  - **单笔交易最大保证金 = 账户净值 × 30%**
  - 防止单笔交易占用过多资金，降低爆仓风险
  - 确保账户有足够缓冲资金

##### ✅ 1.1.5 最小仓位价值验证
```go
const minNotional = 5.0 // OKX最小名义价值要求
if d.PositionSizeUSD < minNotional {
    return fmt.Errorf("仓位价值过小(%.2f USDT)，必须≥%.2f USDT（OKX交易所要求）", ...)
}
```
- **风险控制点**: 满足交易所最小订单价值要求（5 USDT）

##### ✅ 1.1.6 数量精度验证
```go
calculatedQuantity := d.PositionSizeUSD / currentPrice
minQuantityForPrecision := 0.001 // 3位精度要求
if calculatedQuantity < minQuantityForPrecision {
    minNotionalForPrecision := currentPrice * minQuantityForPrecision
    if minNotionalForPrecision <= accountEquity*0.50 {
        if d.PositionSizeUSD < minNotionalForPrecision {
            return fmt.Errorf("仓位价值过小，由于价格较高，最小需要%.2f USDT才能确保数量格式化后不为0", ...)
        }
    }
}
```
- **风险控制点**: 
  - 确保数量格式化后不为 0（OKX 精度要求 0.001）
  - 对于高价币种（如 BTC），需要更大的仓位价值

##### ✅ 1.1.7 止损止盈验证
```go
if d.StopLoss <= 0 || d.TakeProfit <= 0 {
    return fmt.Errorf("止损和止盈必须大于0")
}

// 做多：止损必须小于止盈
if d.Action == "open_long" {
    if d.StopLoss >= d.TakeProfit {
        return fmt.Errorf("做多时止损价必须小于止盈价")
    }
} else {
    // 做空：止损必须大于止盈
    if d.StopLoss <= d.TakeProfit {
        return fmt.Errorf("做空时止损价必须大于止盈价")
    }
}
```
- **风险控制点**: 确保止损止盈逻辑正确，防止设置错误导致意外损失

##### ✅ 1.1.8 风险回报比验证（核心风险控制）
```go
// 做多：风险 = (入场价 - 止损价) / 入场价，收益 = (止盈价 - 入场价) / 入场价
if d.Action == "open_long" {
    risk := (currentPrice - d.StopLoss) / currentPrice
    reward := (d.TakeProfit - currentPrice) / currentPrice
    if risk > 0 {
        riskRewardRatio = reward / risk
    }
} else {
    // 做空：风险 = (止损价 - 入场价) / 入场价，收益 = (入场价 - 止盈价) / 入场价
    risk := (d.StopLoss - currentPrice) / currentPrice
    reward := (currentPrice - d.TakeProfit) / currentPrice
    if risk > 0 {
        riskRewardRatio = reward / risk
    }
}

// 硬约束：风险回报比必须≥3:1
if riskRewardRatio < 3.0 {
    return fmt.Errorf("风险回报比过低(%.2f:1)，必须≥3:1", ...)
}
```
- **风险控制点**: 
  - **风险回报比必须 ≥ 3:1**
  - 确保每笔交易的潜在收益至少是风险的 3 倍
  - 提高交易质量，减少低质量交易
  - 使用当前价格作为入场价进行计算

##### ✅ 1.1.9 其他操作验证
- `update_stop_loss`: 新止损价格必须 > 0
- `update_take_profit`: 新止盈价格必须 > 0
- `partial_close`: 平仓百分比必须在 0-100 之间

#### 1.2 `validateDecisions` 函数 - 批量决策验证

**位置**: `nofx/decision/engine.go:911-935`

**验证内容**:
- 遍历所有决策，逐个调用 `validateDecision` 验证
- 如果任何决策验证失败，返回错误

---

### 2. 交易执行层 (`nofx/trader/auto_trader.go`)

#### 2.1 `executeOpenLongWithRecord` / `executeOpenShortWithRecord` - 开仓执行验证

**位置**: `nofx/trader/auto_trader.go:790-920`

**验证内容**:

##### ✅ 2.1.1 防止仓位叠加
```go
// 检查是否已有同币种同方向持仓
positions, err := at.trader.GetPositions()
if err == nil {
    for _, pos := range positions {
        if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
            return fmt.Errorf("❌ %s 已有多仓，拒绝开仓以防止仓位叠加超限", ...)
        }
    }
}
```
- **风险控制点**: 防止同一币种同一方向重复开仓，避免仓位叠加

##### ✅ 2.1.2 保证金+手续费验证
```go
requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)
estimatedFee := decision.PositionSizeUSD * 0.0010 // OKX Taker费率0.10%
totalRequired := requiredMargin + estimatedFee

if totalRequired > availableBalance {
    return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT", ...)
}
```
- **风险控制点**: 
  - **保证金 + 手续费必须 ≤ 可用余额**
  - 防止因保证金不足导致开仓失败
  - 手续费按 OKX Taker 费率 0.10% 估算

---

### 3. 市场数据过滤层 (`nofx/decision/engine.go`)

#### 3.1 流动性过滤（持仓价值阈值）

**位置**: `nofx/decision/engine.go:219-235`

**验证内容**:
```go
const minOIThresholdMillions = 5.0 // 激进策略：5M USD
oiValue := data.OpenInterest.Latest * data.CurrentPrice
oiValueInMillions := oiValue / 1_000_000
if oiValueInMillions < minOIThresholdMillions {
    // 跳过此币种
    continue
}
```
- **风险控制点**: 
  - 持仓价值（Open Interest）必须 ≥ 5M USD
  - 过滤低流动性币种，降低滑点和流动性风险
  - 现有持仓不受此限制（需要决策是否平仓）

---

## 📊 风险控制参数汇总

### 硬约束参数（代码中定义）

| 参数 | 值 | 位置 | 说明 |
|------|-----|------|------|
| **最小保证金** | 1.0 USDT | `validateDecision` | OKX合约最低要求 |
| **最大保证金比例** | 30% | `validateDecision` | 单笔交易最大保证金 = 账户净值 × 30% |
| **最小仓位价值** | 5.0 USDT | `validateDecision` | OKX交易所要求 |
| **数量精度** | 0.001 | `validateDecision` | OKX BTC/ETH合约精度要求 |
| **风险回报比** | ≥ 3:1 | `validateDecision` | 硬约束，必须满足 |
| **最小持仓价值** | 5M USD | `buildContext` | 流动性过滤阈值 |
| **手续费率** | 0.10% | `executeOpenLongWithRecord` | OKX Taker费率估算 |

### 配置参数（从配置文件读取）

| 参数 | 配置项 | 说明 |
|------|--------|------|
| **BTC/ETH最大杠杆** | `btcEthLeverage` | 从配置文件读取 |
| **山寨币最大杠杆** | `altcoinLeverage` | 从配置文件读取 |
| **仓位模式** | `isCrossMargin` | true=全仓，false=逐仓 |

### 提示词约束（不在代码中验证，由AI遵守）

| 约束 | 说明 | 位置 |
|------|------|------|
| **最多持仓5个币种** | 仅在提示词中说明，代码中未硬性验证 | `buildSystemPrompt` |
| **清算距离>8%** | 仅在提示词中说明，代码中未硬性验证 | `buildSystemPrompt` |

---

## 🔍 风险控制流程

### 开仓决策验证流程

```
1. AI生成决策
   ↓
2. extractDecisions() - 提取JSON决策
   ↓
3. validateDecisions() - 批量验证
   ↓
4. validateDecision() - 单个决策验证
   ├─ Action有效性 ✓
   ├─ 杠杆倍数验证 ✓
   ├─ 仓位价值验证 ✓
   ├─ 保证金验证 ✓
   │  ├─ 最小保证金 ≥ 1 USDT
   │  └─ 最大保证金 ≤ 账户净值30%
   ├─ 最小仓位价值 ≥ 5 USDT ✓
   ├─ 数量精度验证 ✓
   ├─ 止损止盈逻辑验证 ✓
   └─ 风险回报比 ≥ 3:1 ✓
   ↓
5. executeOpenLongWithRecord() - 执行开仓
   ├─ 防止仓位叠加验证 ✓
   └─ 保证金+手续费验证 ✓
   ↓
6. 开仓成功
```

---

## ⚠️ 重要说明

### 1. 保证金 vs 仓位价值

- **position_size_usd** = 仓位价值（名义价值）= 保证金 × 杠杆
- **保证金** = position_size_usd / leverage（实际使用的资金）
- **风险控制基于保证金，不是仓位价值**

### 2. 风险回报比计算

- 使用**当前价格**作为入场价
- 做多：风险 = (当前价 - 止损价) / 当前价，收益 = (止盈价 - 当前价) / 当前价
- 做空：风险 = (止损价 - 当前价) / 当前价，收益 = (当前价 - 止盈价) / 当前价

### 3. 未在代码中验证的约束

以下约束仅在提示词中说明，由AI遵守，代码中未硬性验证：
- **最多持仓5个币种** - 建议在代码中添加验证
- **清算距离>8%** - 需要从持仓信息中获取清算价格进行计算

### 4. 风险控制优先级

1. **最高优先级**（硬约束，必须满足）:
   - 风险回报比 ≥ 3:1
   - 最大保证金 ≤ 账户净值30%
   - 保证金 + 手续费 ≤ 可用余额

2. **中等优先级**（交易所要求）:
   - 最小保证金 ≥ 1 USDT
   - 最小仓位价值 ≥ 5 USDT
   - 数量精度要求

3. **低优先级**（提示词约束）:
   - 最多持仓5个币种
   - 清算距离>8%

---

## 📝 建议改进

1. **添加持仓数量验证**: 在 `validateDecisions` 中检查总持仓数量是否超过5个
2. **添加清算距离验证**: 在 `validateDecision` 中计算并验证清算距离是否>8%
3. **添加总保证金验证**: 检查所有持仓的总保证金是否超过账户净值的某个比例（如80%）

---

**文档版本**: V1.61  
**最后更新**: 2025-11-08





