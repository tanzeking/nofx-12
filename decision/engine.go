package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"regexp"
	"strings"
	"time"
)

// 预编译正则表达式（性能优化：避免每次调用时重新编译）
var (
	// ✅ 安全的正則：精確匹配 ```json 代碼塊
	// 使用反引號 + 拼接避免轉義問題
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime     string                  `json:"current_time"`
	RuntimeMinutes  int                     `json:"runtime_minutes"`
	CallCount       int                     `json:"call_count"`
	Account         AccountInfo             `json:"account"`
	Positions       []PositionInfo          `json:"positions"`
	CandidateCoins  []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap   map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap    map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance     interface{}             `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage  int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
	Exchange        string                  `json:"-"` // 交易所ID（binance/okx等）
	HistoryDecisions []*HistoryDecision     `json:"-"` // 历史决策记录（最近3-5次，用于连续性分析）
}

// HistoryDecision 历史决策记录（简化版，用于传递给AI）
type HistoryDecision struct {
	CycleNumber int                `json:"cycle_number"` // 周期编号
	Timestamp   string             `json:"timestamp"`    // 决策时间
	Decisions   []Decision         `json:"decisions"`    // 决策列表
	CoTTrace    string             `json:"cot_trace"`    // 思维链（推理过程）
}

// Decision AI的交易决策
type Decision struct {
	Symbol          string  `json:"symbol"`
	Action          string  `json:"action"` // "open_long", "open_short", "close_long", "close_short", "update_stop_loss", "update_take_profit", "partial_close", "hold", "wait"

	// 开仓参数
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// 调整参数（新增）
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"`     // 用于 update_stop_loss
	NewTakeProfit   float64 `json:"new_take_profit,omitempty"`   // 用于 update_take_profit
	ClosePercentage float64 `json:"close_percentage,omitempty"`  // 用于 partial_close (0-100)

	// 通用参数
	Confidence      int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD         float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning       string  `json:"reasoning"`
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // 系统提示词（发送给AI的系统prompt）
	UserPrompt   string     `json:"user_prompt"`   // 发送给AI的输入prompt
	CoTTrace     string     `json:"cot_trace"`     // 思维链分析（AI输出）
	Decisions    []Decision `json:"decisions"`     // 具体决策列表
	Timestamp    time.Time  `json:"timestamp"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient *mcp.Client) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
}

// GetFullDecisionWithCustomPrompt 获取AI的完整交易决策（支持自定义prompt和模板选择）
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPromptWithCustom(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, customPrompt, overrideBase, templateName)
	userPrompt := buildUserPrompt(ctx)

	// V1.70版本：输出详细的输入提示词（用于调试和查看）
	log.Printf("\n" + strings.Repeat("=", 80))
	log.Printf("📋 【系统提示词】 (System Prompt)")
	log.Printf(strings.Repeat("=", 80))
	log.Printf("%s", systemPrompt)
	log.Printf(strings.Repeat("=", 80))
	log.Printf("📊 【用户提示词】 (User Prompt)")
	log.Printf(strings.Repeat("=", 80))
	log.Printf("%s", userPrompt)
	log.Printf(strings.Repeat("=", 80))
	
	// 计算token数量（粗略估算：中文字符数 * 1.3 + 英文字符数 * 0.25）
	systemPromptTokens := estimateTokenCount(systemPrompt)
	userPromptTokens := estimateTokenCount(userPrompt)
	totalTokens := systemPromptTokens + userPromptTokens
	log.Printf("📊 Token估算: System=%d, User=%d, Total=%d", systemPromptTokens, userPromptTokens, totalTokens)
	log.Printf(strings.Repeat("=", 80) + "\n")

	// 3. 调用AI API（使用 system + user prompt）
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 4. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, ctx.MarketDataMap)
	if err != nil {
		return decision, fmt.Errorf("解析AI响应失败: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.SystemPrompt = systemPrompt // 保存系统prompt
	decision.UserPrompt = userPrompt     // 保存输入prompt
	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 获取交易所ID（默认binance）
	exchangeID := "binance"
	if ctx.Exchange != "" {
		exchangeID = ctx.Exchange
	}
	
	// 并发获取市场数据，增加重试机制确保数据完整
	for symbol := range symbolSet {
		var data *market.Data
		var err error
		maxRetries := 3
		
		// 重试获取市场数据
		for attempt := 1; attempt <= maxRetries; attempt++ {
			data, err = market.GetWithExchange(symbol, exchangeID)
			if err == nil {
				break
			}
			if attempt < maxRetries {
				log.Printf("⚠️  获取 %s 市场数据失败（尝试 %d/%d），1秒后重试: %v", symbol, attempt, maxRetries, err)
				time.Sleep(time.Duration(attempt) * time.Second) // 指数退避
			} else {
				log.Printf("❌ 获取 %s 市场数据失败（已重试%d次）: %v", symbol, maxRetries, err)
			}
		}
		
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
			continue
		}

		// V1.63版本：移除流动性过滤，让AI自由选择币种
		ctx.MarketDataMap[symbol] = data
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// ⚠️ 重要：限制候选币种数量，避免 Prompt 过大
	// 根据持仓数量动态调整：持仓越少，可以分析更多候选币
	const (
		maxCandidatesWhenEmpty    = 30 // 无持仓时最多分析30个候选币
		maxCandidatesWhenHolding1 = 25 // 持仓1个时最多分析25个候选币
		maxCandidatesWhenHolding2 = 20 // 持仓2个时最多分析20个候选币
		maxCandidatesWhenHolding3 = 15 // 持仓3个时最多分析15个候选币（避免 Prompt 过大）
	)

	positionCount := len(ctx.Positions)
	var maxCandidates int

	switch positionCount {
	case 0:
		maxCandidates = maxCandidatesWhenEmpty
	case 1:
		maxCandidates = maxCandidatesWhenHolding1
	case 2:
		maxCandidates = maxCandidatesWhenHolding2
	default: // 3+ 持仓
		maxCandidates = maxCandidatesWhenHolding3
	}

	// 返回实际候选币数量和上限中的较小值
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom 构建包含自定义内容的 System Prompt
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string) string {
	// 如果覆盖基础prompt且有自定义prompt，只使用自定义prompt
	if overrideBase && customPrompt != "" {
		return customPrompt
	}

	// 获取基础prompt（使用指定的模板）
	basePrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName)

	// 如果没有自定义prompt，直接返回基础prompt
	if customPrompt == "" {
		return basePrompt
	}

	// 添加自定义prompt部分到基础prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 个性化交易策略\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("注意: 以上个性化策略是对基础规则的补充，不能违背基础风险控制原则。\n")

	return sb.String()
}

// buildSystemPrompt 构建 System Prompt（使用模板+动态部分）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string) string {
	var sb strings.Builder

	// 1. 加载提示词模板（核心交易策略部分）
	if templateName == "" {
		templateName = "default" // 默认使用 default 模板
	}

	template, err := GetPromptTemplate(templateName)
	if err != nil {
		// 如果模板不存在，记录错误并使用 default
		log.Printf("⚠️  提示词模板 '%s' 不存在，使用 default: %v", templateName, err)
		template, err = GetPromptTemplate("default")
		if err != nil {
			// 如果连 default 都不存在，使用内置的简化版本
			log.Printf("❌ 无法加载任何提示词模板，使用内置简化版本")
			sb.WriteString("你是专业的加密货币交易AI。请根据市场数据做出交易决策。\n\n")
		} else {
			sb.WriteString(template.Content)
			sb.WriteString("\n\n")
		}
	} else {
		sb.WriteString(template.Content)
		sb.WriteString("\n\n")
	}

	// V1.70版本：精简系统提示词，减少token使用（控制在1天1元左右）
	// 核心规则和约束（精简版）
	sb.WriteString("# 核心规则\n\n")
	sb.WriteString(fmt.Sprintf("- 风险回报比≥3:1 | 杠杆: 山寨币≤%dx, BTC/ETH≤%dx | 最多3个持仓\n", altcoinLeverage, btcEthLeverage))
	sb.WriteString("- 开仓: 最小20%%账户净值，推荐50-80%%账户净值\n")
	sb.WriteString("- 爆仓价: 做多=入场×(1-1/杠杆), 做空=入场×(1+1/杠杆)\n")
	sb.WriteString("- 止损必须在爆仓价上方，否则止损失效\n\n")
	
	sb.WriteString("# 可用动作\n\n")
	sb.WriteString("open_long/open_short/close_long/close_short/partial_close/update_stop_loss/update_take_profit/hold/wait\n\n")
	
	sb.WriteString("# 输出格式\n\n")
	sb.WriteString("JSON: action, symbol, leverage, position_size_usd, stop_loss, take_profit, confidence(0-100), reasoning\n")
	sb.WriteString("开仓必填: leverage, position_size_usd, stop_loss, take_profit, confidence, reasoning\n")
	sb.WriteString("wait/hold/close操作: 可省略开仓字段或设为null\n")
	sb.WriteString("💡 position_size_usd是仓位价值，保证金=position_size_usd/leverage\n\n")

	return sb.String()
}

// buildUserPrompt 构建 User Prompt（动态数据）
// V1.70版本：增强用户提示词，添加详细的账户信息、持仓信息、市场数据
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// ========== 1. 系统状态 ==========
	sb.WriteString(fmt.Sprintf("【时间】%s | 周期#%d | 运行%d分钟\n\n", ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// ========== 2. 账户详细信息 ==========
	sb.WriteString("【账户信息】\n")
	sb.WriteString(fmt.Sprintf("  账户净值（本金）: %.2f USDT\n", ctx.Account.TotalEquity))
	sb.WriteString(fmt.Sprintf("  可用余额: %.2f USDT (%.1f%%)\n", ctx.Account.AvailableBalance, (ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100))
	sb.WriteString(fmt.Sprintf("  已用保证金: %.2f USDT (%.1f%%)\n", ctx.Account.MarginUsed, ctx.Account.MarginUsedPct))
	sb.WriteString(fmt.Sprintf("  总盈亏: %+.2f USDT (%+.2f%%)\n", ctx.Account.TotalPnL, ctx.Account.TotalPnLPct))
	sb.WriteString(fmt.Sprintf("  当前持仓数: %d个\n", ctx.Account.PositionCount))
	
	// 计算可开仓金额（基于可用余额和杠杆）
	availableForTrading := ctx.Account.AvailableBalance
	if availableForTrading > 0 {
		maxPositionValueAltcoin := availableForTrading * float64(ctx.AltcoinLeverage)
		maxPositionValueBtcEth := availableForTrading * float64(ctx.BTCETHLeverage)
		sb.WriteString(fmt.Sprintf("  💡 可开仓金额（基于可用余额）:\n"))
		sb.WriteString(fmt.Sprintf("     - 山寨币: 最多%.2f USDT仓位价值（可用%.2f × %dx杠杆）\n", maxPositionValueAltcoin, availableForTrading, ctx.AltcoinLeverage))
		sb.WriteString(fmt.Sprintf("     - BTC/ETH: 最多%.2f USDT仓位价值（可用%.2f × %dx杠杆）\n", maxPositionValueBtcEth, availableForTrading, ctx.BTCETHLeverage))
	}
	sb.WriteString("\n")

	// ========== 3. BTC市场概览 ==========
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString("【BTC市场】\n")
		sb.WriteString(fmt.Sprintf("  价格: %.2f USDT\n", btcData.CurrentPrice))
		sb.WriteString(fmt.Sprintf("  1小时: %+.2f%% | 4小时: %+.2f%%\n", btcData.PriceChange1h, btcData.PriceChange4h))
		sb.WriteString(fmt.Sprintf("  MACD: %.4f | RSI: %.1f | EMA20: %.2f\n", btcData.CurrentMACD, btcData.CurrentRSI7, btcData.CurrentEMA20))
		sb.WriteString("\n")
	}

	// ========== 4. 持仓详细信息 ==========
	if len(ctx.Positions) > 0 {
		sb.WriteString("【当前持仓】\n")
		for i, pos := range ctx.Positions {
			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60)
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf("%d分钟", durationMin)
				} else if durationMin < 1440 {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf("%d小时%d分钟", durationHour, durationMinRemainder)
				} else {
					durationDay := durationMin / 1440
					durationHour := (durationMin % 1440) / 60
					holdingDuration = fmt.Sprintf("%d天%d小时", durationDay, durationHour)
				}
			}
			
			// 计算仓位价值
			positionValue := pos.Quantity * pos.MarkPrice
			marginUsed := positionValue / float64(pos.Leverage)
			
			// 计算价格变化百分比
			priceChangePct := ((pos.MarkPrice - pos.EntryPrice) / pos.EntryPrice) * 100
			if pos.Side == "short" {
				priceChangePct = -priceChangePct // 做空时价格下跌是盈利
			}
			
			sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, pos.Symbol, strings.ToUpper(pos.Side)))
			sb.WriteString(fmt.Sprintf("   入场价: %.4f USDT | 当前价: %.4f USDT | 价格变化: %+.2f%%\n", 
				pos.EntryPrice, pos.MarkPrice, priceChangePct))
			sb.WriteString(fmt.Sprintf("   数量: %.8f | 仓位价值: %.2f USDT | 杠杆: %dx | 保证金: %.2f USDT\n",
				pos.Quantity, positionValue, pos.Leverage, marginUsed))
			sb.WriteString(fmt.Sprintf("   未实现盈亏: %+.2f USDT (%+.2f%%)\n", pos.UnrealizedPnL, pos.UnrealizedPnLPct))
			sb.WriteString(fmt.Sprintf("   爆仓价: %.4f USDT | 持仓时长: %s\n", pos.LiquidationPrice, holdingDuration))
			
			// 显示该币种的市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("   市场数据: EMA20=%.2f MACD=%.4f RSI=%.1f | 1h:%+.2f%% 4h:%+.2f%%\n",
					marketData.CurrentEMA20, marketData.CurrentMACD, marketData.CurrentRSI7,
					marketData.PriceChange1h, marketData.PriceChange4h))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("【当前持仓】无\n\n")
	}

	// ========== 5. 候选币种市场数据 ==========
	sb.WriteString(fmt.Sprintf("【候选币种市场数据】（%d个）\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTag := ""
		if len(coin.Sources) > 1 {
			sourceTag = "[多源]"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTag = "[OI]"
		}

		// 显示更详细的市场数据
		sb.WriteString(fmt.Sprintf("%d. %s %s\n", displayedCount, coin.Symbol, sourceTag))
		sb.WriteString(fmt.Sprintf("   价格: %.4f USDT | EMA20: %.4f | MACD: %.4f | RSI: %.1f\n",
			marketData.CurrentPrice, marketData.CurrentEMA20, marketData.CurrentMACD, marketData.CurrentRSI7))
		sb.WriteString(fmt.Sprintf("   1小时: %+.2f%% | 4小时: %+.2f%%\n",
			marketData.PriceChange1h, marketData.PriceChange4h))
		
		// 显示更多技术指标（如果可用）
		if marketData.LongerTermContext != nil && marketData.LongerTermContext.ATR14 > 0 {
			sb.WriteString(fmt.Sprintf("   ATR14: %.4f\n", marketData.LongerTermContext.ATR14))
		}
		if marketData.BollingerBands != nil {
			sb.WriteString(fmt.Sprintf("   布林带: 上轨=%.4f 中轨=%.4f 下轨=%.4f\n",
				marketData.BollingerBands.Upper, marketData.BollingerBands.Middle, marketData.BollingerBands.Lower))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// ========== 6. 历史表现 ==========
	if ctx.Performance != nil {
		type PerformanceData struct {
			SharpeRatio float64 `json:"sharpe_ratio"`
		}
		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				sb.WriteString(fmt.Sprintf("【历史表现】夏普比率: %.2f\n\n", perfData.SharpeRatio))
			}
		}
	}

	// ========== 7. 历史决策记录 ==========
	if len(ctx.HistoryDecisions) > 0 {
		sb.WriteString("【历史决策记录】\n")
		
		// 从新到旧显示，最多显示最近5次
		maxHistoryDisplay := 5
		startIdx := len(ctx.HistoryDecisions) - maxHistoryDisplay
		if startIdx < 0 {
			startIdx = 0
		}
		
		for i := len(ctx.HistoryDecisions) - 1; i >= startIdx; i-- {
			hist := ctx.HistoryDecisions[i]
			
			if len(hist.Decisions) > 0 {
				decisionSummary := []string{}
				for _, d := range hist.Decisions {
					if d.Action == "open_long" {
						decisionSummary = append(decisionSummary, fmt.Sprintf("%s开多(%dx)", d.Symbol, d.Leverage))
					} else if d.Action == "open_short" {
						decisionSummary = append(decisionSummary, fmt.Sprintf("%s开空(%dx)", d.Symbol, d.Leverage))
					} else if d.Action == "wait" || d.Action == "hold" {
						decisionSummary = append(decisionSummary, d.Action)
					} else if d.Action == "close_long" {
						decisionSummary = append(decisionSummary, fmt.Sprintf("%s平多", d.Symbol))
					} else if d.Action == "close_short" {
						decisionSummary = append(decisionSummary, fmt.Sprintf("%s平空", d.Symbol))
					} else {
						decisionSummary = append(decisionSummary, fmt.Sprintf("%s%s", d.Symbol, d.Action))
					}
				}
				if len(decisionSummary) > 0 {
					sb.WriteString(fmt.Sprintf("  周期#%d (%s): %s\n", hist.CycleNumber, hist.Timestamp, strings.Join(decisionSummary, ", ")))
				}
			} else {
				sb.WriteString(fmt.Sprintf("  周期#%d (%s): wait\n", hist.CycleNumber, hist.Timestamp))
			}
			
			// 只对最近一次决策显示实际结果
			if i == len(ctx.HistoryDecisions)-1 {
				lastDecision := hist
				openedPositions := make(map[string]bool)
				for _, d := range lastDecision.Decisions {
					if d.Action == "open_long" || d.Action == "open_short" {
						openedPositions[d.Symbol] = true
					}
				}
				
				currentPositions := make(map[string]bool)
				positionPnL := make(map[string]float64)
				for _, pos := range ctx.Positions {
					currentPositions[pos.Symbol] = true
					positionPnL[pos.Symbol] = pos.UnrealizedPnLPct
				}
				
				resultSummary := []string{}
				for symbol := range openedPositions {
					if currentPositions[symbol] {
						resultSummary = append(resultSummary, fmt.Sprintf("%s:%+.1f%%", symbol, positionPnL[symbol]))
					} else {
						resultSummary = append(resultSummary, fmt.Sprintf("%s:已平仓", symbol))
					}
				}
				if len(resultSummary) > 0 {
					sb.WriteString(fmt.Sprintf("  结果: %s\n", strings.Join(resultSummary, ", ")))
				}
			}
		}
		sb.WriteString("\n")
	}

	// ========== 8. 决策要求 ==========
	sb.WriteString("【决策要求】\n")
	sb.WriteString("1. 仔细分析账户信息（本金、可用余额、已用保证金）\n")
	sb.WriteString("2. 分析当前持仓状态（盈亏、爆仓价、持仓时长）\n")
	sb.WriteString("3. 评估候选币种市场数据（价格、技术指标、趋势）\n")
	sb.WriteString("4. 确保止损价在爆仓价上方，防止止损失效\n")
	sb.WriteString("5. 基于可用余额和杠杆计算可开仓金额\n")
	sb.WriteString("6. 保持决策连续性，参考历史决策结果\n")
	sb.WriteString("7. 输出思维链分析 + JSON格式决策\n\n")
	
	sb.WriteString("---\n请分析以上信息，输出决策（思维链+JSON）\n")

	return sb.String()
}

// estimateTokenCount 估算token数量（粗略估算）
// 中文字符按1.3个token计算，英文字符按0.25个token计算
func estimateTokenCount(text string) int {
	chineseCount := 0
	englishCount := 0
	
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			chineseCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			englishCount++
		} else if r == ' ' || r == '\n' || r == '\t' {
			englishCount++
		}
	}
	
	// 粗略估算：中文字符 * 1.3 + 英文字符 * 0.25
	tokens := int(float64(chineseCount)*1.3 + float64(englishCount)*0.25)
	return tokens
}

// parseFullDecisionResponse 解析AI的完整决策响应
// V1.59版本：添加marketDataMap参数，用于验证高价币种
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, marketDataMap map[string]*market.Data) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取JSON决策列表
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("提取决策失败: %w", err)
	}

	// 3. 验证决策
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, marketDataMap); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("决策验证失败: %w", err)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 查找JSON数组的开始位置
	jsonStart := strings.Index(response, "[")

	if jsonStart > 0 {
		// 思维链是JSON数组之前的内容
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到JSON，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
// V1.59版本：修复空字符串字段解析问题（AI返回wait/hold时，字段可能为空字符串）
func extractDecisions(response string) ([]Decision, error) {
	// 预清洗：去零宽/BOM
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)

	// 🔧 关键修复 (Critical Fix)：在正则匹配之前就先修复全角字符！
	// 否则正则表达式 \[ 无法匹配全角的 ［
	s = fixMissingQuotes(s)

	// V1.59版本：预处理JSON，将空字符串字段转换为null或删除（避免解析失败）
	// 例如：{"leverage":""} -> {"leverage":null} 或删除该字段
	s = fixEmptyStringFields(s)

	// 1) 优先从 ```json 代码块中提取
	if m := reJSONFence.FindStringSubmatch(s); m != nil && len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent) // 把 "[ {" 规整为 "[{"
		jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）
		jsonContent = fixEmptyStringFields(jsonContent) // V1.59：修复空字符串字段
		jsonContent = fixThousandSeparators(jsonContent) // V1.61：修复千位分隔符
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		
		// V1.59.1版本：在解析前再次检查并修复空字符串字段（确保万无一失）
		jsonContent = fixEmptyStringFields(jsonContent)
		
		// V1.59.1版本：添加调试日志，输出修复后的JSON内容
		if strings.Contains(jsonContent, `""`) {
			log.Printf("  ⚠️  警告：JSON内容中仍存在空字符串字段，尝试最后一次修复")
			// 最后一次尝试：使用更激进的方法，直接替换所有数值字段的空字符串
			jsonContent = strings.ReplaceAll(jsonContent, `"leverage":""`, `"leverage":null`)
			jsonContent = strings.ReplaceAll(jsonContent, `"position_size_usd":""`, `"position_size_usd":null`)
			jsonContent = strings.ReplaceAll(jsonContent, `"stop_loss":""`, `"stop_loss":null`)
			jsonContent = strings.ReplaceAll(jsonContent, `"take_profit":""`, `"take_profit":null`)
			jsonContent = strings.ReplaceAll(jsonContent, `"confidence":""`, `"confidence":null`)
			jsonContent = strings.ReplaceAll(jsonContent, `"risk_usd":""`, `"risk_usd":null`)
			jsonContent = strings.ReplaceAll(jsonContent, `"new_stop_loss":""`, `"new_stop_loss":null`)
			jsonContent = strings.ReplaceAll(jsonContent, `"new_take_profit":""`, `"new_take_profit":null`)
			jsonContent = strings.ReplaceAll(jsonContent, `"close_percentage":""`, `"close_percentage":null`)
		}
		
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			// V1.59.1版本：如果仍然失败，输出详细的错误信息和JSON内容
			return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s\nJSON长度: %d\n是否包含空字符串: %v", 
				err, jsonContent, len(jsonContent), strings.Contains(jsonContent, `""`))
		}
		return decisions, nil
	}

	// 2) 退而求其次 (Fallback)：全文寻找首个对象数组
	// 注意：此时 s 已经过 fixMissingQuotes()，全角字符已转换为半角
	jsonContent := strings.TrimSpace(reJSONArray.FindString(s))
	if jsonContent == "" {
		// 🔧 安全回退 (Safe Fallback)：当AI只输出思维链没有JSON时，生成保底决策（避免系统崩溃）
		log.Printf("⚠️  [SafeFallback] AI未输出JSON决策，进入安全等待模式 (AI response without JSON, entering safe wait mode)")

		// 提取思维链摘要（最多 240 字符）
		cotSummary := s
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		// 生成保底决策：所有币种进入 wait 状态
		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("模型未输出结构化JSON决策，进入安全等待；摘要：%s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	// 🔧 规整格式（此时全角字符已在前面修复过）
	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）
	jsonContent = fixEmptyStringFields(jsonContent) // V1.59：修复空字符串字段
	jsonContent = fixThousandSeparators(jsonContent) // V1.61：修复千位分隔符

	// 🔧 验证 JSON 格式（检测常见错误）
	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
	}

	// 解析JSON
	var decisions []Decision
	
	// V1.59.1版本：在解析前再次检查并修复空字符串字段（确保万无一失）
	jsonContent = fixEmptyStringFields(jsonContent)
	
	// V1.59.1版本：添加调试日志，输出修复后的JSON内容
	if strings.Contains(jsonContent, `""`) {
		log.Printf("  ⚠️  警告：JSON内容中仍存在空字符串字段，尝试最后一次修复")
		// 最后一次尝试：使用更激进的方法，直接替换所有数值字段的空字符串
		jsonContent = strings.ReplaceAll(jsonContent, `"leverage":""`, `"leverage":null`)
		jsonContent = strings.ReplaceAll(jsonContent, `"position_size_usd":""`, `"position_size_usd":null`)
		jsonContent = strings.ReplaceAll(jsonContent, `"stop_loss":""`, `"stop_loss":null`)
		jsonContent = strings.ReplaceAll(jsonContent, `"take_profit":""`, `"take_profit":null`)
		jsonContent = strings.ReplaceAll(jsonContent, `"confidence":""`, `"confidence":null`)
		jsonContent = strings.ReplaceAll(jsonContent, `"risk_usd":""`, `"risk_usd":null`)
		jsonContent = strings.ReplaceAll(jsonContent, `"new_stop_loss":""`, `"new_stop_loss":null`)
		jsonContent = strings.ReplaceAll(jsonContent, `"new_take_profit":""`, `"new_take_profit":null`)
		jsonContent = strings.ReplaceAll(jsonContent, `"close_percentage":""`, `"close_percentage":null`)
	}
	
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		// V1.59.1版本：如果仍然失败，输出详细的错误信息和JSON内容
		return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s\nJSON长度: %d\n是否包含空字符串: %v", 
			err, jsonContent, len(jsonContent), strings.Contains(jsonContent, `""`))
	}

	return decisions, nil
}

// fixMissingQuotes 替换中文引号和全角字符为英文引号和半角字符（避免AI输出全角JSON字符导致解析失败）
func fixMissingQuotes(jsonStr string) string {
	// 替换中文引号
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// ⚠️ 替换全角括号、冒号、逗号（防止AI输出全角JSON字符）
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B 全角左方括号
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D 全角右方括号
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B 全角左花括号
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D 全角右花括号
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A 全角冒号
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C 全角逗号

	// ⚠️ 替换CJK标点符号（AI在中文上下文中也可能输出这些）
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK左方头括号 U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK右方头括号 U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK左龟壳括号 U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK右龟壳括号 U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK顿号 U+3001

	// ⚠️ 替换全角空格为半角空格（JSON中不应该有全角空格）
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 全角空格

	return jsonStr
}

// fixEmptyStringFields 修复空字符串字段（V1.59版本）
// 将JSON中的空字符串字段（如 "leverage":""）转换为null，避免解析失败
// 对于wait/hold等操作，AI可能返回所有字段为空字符串，这会导致JSON解析失败
func fixEmptyStringFields(jsonStr string) string {
	// V1.59.1版本：使用更严格的匹配，确保能匹配到所有情况
	// 处理数值类型字段的空字符串，包括可能存在的空格
	patterns := []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		// leverage (int类型) - 匹配 "leverage":"" 或 "leverage" : "" 等格式
		{regexp.MustCompile(`"leverage"\s*:\s*""`), `"leverage":null`},
		// position_size_usd (float64类型)
		{regexp.MustCompile(`"position_size_usd"\s*:\s*""`), `"position_size_usd":null`},
		// stop_loss (float64类型)
		{regexp.MustCompile(`"stop_loss"\s*:\s*""`), `"stop_loss":null`},
		// take_profit (float64类型)
		{regexp.MustCompile(`"take_profit"\s*:\s*""`), `"take_profit":null`},
		// confidence (int类型)
		{regexp.MustCompile(`"confidence"\s*:\s*""`), `"confidence":null`},
		// risk_usd (float64类型)
		{regexp.MustCompile(`"risk_usd"\s*:\s*""`), `"risk_usd":null`},
		// new_stop_loss (float64类型)
		{regexp.MustCompile(`"new_stop_loss"\s*:\s*""`), `"new_stop_loss":null`},
		// new_take_profit (float64类型)
		{regexp.MustCompile(`"new_take_profit"\s*:\s*""`), `"new_take_profit":null`},
		// close_percentage (float64类型)
		{regexp.MustCompile(`"close_percentage"\s*:\s*""`), `"close_percentage":null`},
	}
	
	originalStr := jsonStr
	for _, p := range patterns {
		jsonStr = p.pattern.ReplaceAllString(jsonStr, p.replacement)
	}
	
	// 调试：如果修复了任何内容，记录日志
	if originalStr != jsonStr {
		log.Printf("  🔧 fixEmptyStringFields: 已修复空字符串字段 (修复前长度: %d, 修复后长度: %d)", len(originalStr), len(jsonStr))
	}
	
	return jsonStr
}

// fixThousandSeparators 修复JSON数字中的千位分隔符（V1.61版本）
// AI可能在数字中使用逗号作为千位分隔符（如 100,500），这在JSON中是无效的
// 自动移除这些逗号，而不是报错
func fixThousandSeparators(jsonStr string) string {
	// 使用正则表达式匹配JSON值中的数字（不在字符串中）
	// 模式：匹配 ": 数字,数字" 或 ":数字,数字" 格式
	// 更精确的模式：在冒号后面，匹配数字+逗号+3位数字的模式
	// 使用更宽松的匹配，因为JSON值可能是 "stop_loss":100,500 或 "stop_loss": 100,500
	
	originalStr := jsonStr
	
	// 匹配模式：数字+逗号+3位数字（千位分隔符的典型模式）
	// 例如：100,500 -> 100500
	// 使用循环处理多个千位分隔符（如 1,234,567）
	re := regexp.MustCompile(`(\d+),(\d{3})`)
	for {
		newStr := re.ReplaceAllString(jsonStr, `$1$2`)
		if newStr == jsonStr {
			break
		}
		jsonStr = newStr
	}
	
	// 如果修复了任何内容，记录日志
	if originalStr != jsonStr {
		log.Printf("  🔧 fixThousandSeparators: 已移除千位分隔符 (修复前长度: %d, 修复后长度: %d)", len(originalStr), len(jsonStr))
	}
	
	return jsonStr
}

// validateJSONFormat 验证 JSON 格式，检测常见错误
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	// 允许 [ 和 { 之间存在任意空白（含零宽）
	if !reArrayHead.MatchString(trimmed) {
		// 检查是否是纯数字/范围数组（常见错误）
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("不是有效的决策数组（必须包含对象 {}），实际内容: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON 必须以 [{ 开头（允许空白），实际: %s", trimmed[:min(20, len(trimmed))])
	}

	// 检查是否包含范围符号 ~（LLM 常见错误）
	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON 中不可包含范围符号 ~，所有数字必须是精确的单一值")
	}

	// V1.61版本：移除千位分隔符检查，因为fixThousandSeparators会自动修复
	// 不再报错，而是自动修复

	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeInvisibleRunes 去除零宽字符和 BOM，避免肉眼看不见的前缀破坏校验
func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

// compactArrayOpen 规整开头的 "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
// V1.59版本：添加marketDataMap参数，根据价格判断高价币种
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, marketDataMap map[string]*market.Data) error {
	for i, decision := range decisions {
		// 获取当前价格（如果可用）
		currentPrice := 0.0
		if marketDataMap != nil {
			if data, ok := marketDataMap[decision.Symbol]; ok && data != nil {
				currentPrice = data.CurrentPrice
			}
		}
		
		// 如果无法获取价格，尝试从market包获取（fallback）
		if currentPrice <= 0 && (decision.Action == "open_long" || decision.Action == "open_short") {
			if data, err := market.Get(decision.Symbol); err == nil {
				currentPrice = data.CurrentPrice
			}
		}
		
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage, currentPrice); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
}

// findMatchingBracket 查找匹配的右括号
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// validateDecision 验证单个决策的有效性
// V1.59版本：添加currentPrice参数，根据价格判断高价币种（价格>500 USDT）
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, currentPrice float64) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":          true,
		"open_short":         true,
		"close_long":         true,
		"close_short":        true,
		"update_stop_loss":   true,
		"update_take_profit": true,
		"partial_close":      true,
		"hold":               true,
		"wait":               true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// V1.48版本：移除仓位价值上限限制 - 让AI自由决策杠杆和仓位大小
		// 根据币种使用配置的杠杆上限（仅限制杠杆倍数，不限制仓位价值）
		maxLeverage := altcoinLeverage          // 山寨币使用配置的杠杆
		
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC和ETH使用配置的杠杆
		}
		
		// V1.64版本：进一步简化验证逻辑
		// 只保留杠杆倍数验证，其他验证交给AI和交易所

		if d.Leverage <= 0 || d.Leverage > maxLeverage {
			return fmt.Errorf("杠杆必须在1-%d之间（%s，当前配置上限%d倍）: %d", maxLeverage, d.Symbol, maxLeverage, d.Leverage)
		}
		
		// 计算保证金（用于日志记录）
		marginRequired := d.PositionSizeUSD / float64(d.Leverage)
		log.Printf("  ✓ 验证通过：仓位价值%.2f USDT，杠杆%d倍，保证金%.2f USDT", 
			d.PositionSizeUSD, d.Leverage, marginRequired)
	}

	// 动态调整止损验证
	if d.Action == "update_stop_loss" {
		if d.NewStopLoss <= 0 {
			return fmt.Errorf("新止损价格必须大于0: %.2f", d.NewStopLoss)
		}
	}

	// 动态调整止盈验证
	if d.Action == "update_take_profit" {
		if d.NewTakeProfit <= 0 {
			return fmt.Errorf("新止盈价格必须大于0: %.2f", d.NewTakeProfit)
		}
	}

	// 部分平仓验证
	if d.Action == "partial_close" {
		if d.ClosePercentage <= 0 || d.ClosePercentage > 100 {
			return fmt.Errorf("平仓百分比必须在0-100之间: %.1f", d.ClosePercentage)
		}
	}

	return nil
}

// calculateBreakEvenPrice 计算盈亏平衡价格（考虑开仓和平仓手续费）
// entryPrice: 入场价格
// positionSizeUSD: 名义价值（USDT）
// leverage: 杠杆倍数（用于计算，但实际不影响盈亏平衡价）
// isLong: true=做多, false=做空
// 返回: 盈亏平衡的出场价格（OKX普通用户一档Taker费率0.10%）
func calculateBreakEvenPrice(entryPrice, positionSizeUSD float64, leverage int, isLong bool) float64 {
	// OKX普通用户一档Taker费率（市价单）
	const takerFeeRate = 0.0010 // 0.10%
	
	// 计算开仓手续费
	openFee := positionSizeUSD * takerFeeRate
	
	// 计算持仓数量
	quantity := positionSizeUSD / entryPrice
	if quantity <= 0 {
		return entryPrice // 避免除零
	}
	
	// 计算平仓时的名义价值（假设价格不变）
	closePositionSizeUSD := positionSizeUSD
	
	// 计算平仓手续费
	closeFee := closePositionSizeUSD * takerFeeRate
	
	// 总手续费
	totalFee := openFee + closeFee
	
	// 计算盈亏平衡价格
	if isLong {
		// 做多：需要价格上涨以覆盖手续费
		// 盈亏平衡价 = 入场价 + (总手续费 / 数量)
		breakEvenPrice := entryPrice + (totalFee / quantity)
		return math.Ceil(breakEvenPrice*10000) / 10000 // 保留4位小数，向上取整
	} else {
		// 做空：需要价格下跌以覆盖手续费
		// 盈亏平衡价 = 入场价 - (总手续费 / 数量)
		breakEvenPrice := entryPrice - (totalFee / quantity)
		return math.Floor(breakEvenPrice*10000) / 10000 // 保留4位小数，向下取整
	}
}

