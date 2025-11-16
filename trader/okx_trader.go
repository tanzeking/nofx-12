package trader

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OKX 普通用户一档手续费率（合约交易）
const (
	OKXMakerFeeRate = 0.0008 // 挂单手续费率 0.08%
	OKXTakerFeeRate = 0.0010 // 吃单手续费率 0.10%（市价单使用）
)

// OKXTrader OKX合约交易器
type OKXTrader struct {
	apiKey     string
	secretKey  string
	passphrase string
	baseURL    string
	client     *http.Client

	// 余额缓存
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// 持仓缓存
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// 缓存有效期（15秒）
	cacheDuration time.Duration

	// 交易对精度缓存
	symbolPrecision map[string]int
	precisionMutex  sync.RWMutex
	
	// 交易对lotSz缓存（V1.66版本：新增）
	symbolLotSz map[string]float64
	lotSzMutex  sync.RWMutex
}

// NewOKXTrader 创建OKX合约交易器
func NewOKXTrader(apiKey, secretKey, passphrase string, testnet bool) *OKXTrader {
	baseURL := "https://www.okx.com"
	if testnet {
		baseURL = "https://www.okx.com" // OKX测试网使用相同域名，通过API key区分
	}

	trader := &OKXTrader{
		apiKey:      apiKey,
		secretKey:  secretKey,
		passphrase: passphrase,
		baseURL:    baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second, // 增加到60秒，避免超时
		},
		cacheDuration:  10 * time.Second, // 降低到10秒，提高实时性
		symbolPrecision: make(map[string]int),
		symbolLotSz:      make(map[string]float64), // V1.66版本：初始化lotSz缓存
	}

	log.Printf("✓ OKX交易器初始化成功 (testnet=%v)", testnet)
	return trader
}

// signRequest 生成OKX API签名
func (t *OKXTrader) signRequest(method, path, body string, timestamp string) string {
	// OKX签名格式: timestamp + method + path + body
	message := timestamp + method + path + body
	
	// HMAC-SHA256签名
	mac := hmac.New(sha256.New, []byte(t.secretKey))
	mac.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	
	return signature
}

// makeRequest 发送API请求（带重试机制）
func (t *OKXTrader) makeRequest(method, path string, body interface{}) ([]byte, error) {
	maxRetries := 3
	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		var bodyStr string
		if body != nil {
			bodyBytes, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("序列化请求体失败: %w", err)
			}
			bodyStr = string(bodyBytes)
		}

		url := t.baseURL + path
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		signature := t.signRequest(method, path, bodyStr, timestamp)

		req, err := http.NewRequest(method, url, strings.NewReader(bodyStr))
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %w", err)
		}

		// 设置请求头
		req.Header.Set("OK-ACCESS-KEY", t.apiKey)
		req.Header.Set("OK-ACCESS-SIGN", signature)
		req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
		req.Header.Set("OK-ACCESS-PASSPHRASE", t.passphrase)
		req.Header.Set("Content-Type", "application/json")

		resp, err := t.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("请求失败: %w", err)
			// 检查是否是超时或网络错误，可以重试
			if strings.Contains(err.Error(), "timeout") || 
			   strings.Contains(err.Error(), "deadline exceeded") ||
			   strings.Contains(err.Error(), "connection") {
				if attempt < maxRetries {
					waitTime := time.Duration(attempt) * 2 * time.Second
					log.Printf("⚠️  OKX API请求失败（尝试 %d/%d），%v后重试: %v", attempt, maxRetries, waitTime, err)
					time.Sleep(waitTime)
					continue
				}
			}
			return nil, lastErr
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("读取响应失败: %w", err)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API错误 (状态码: %d): %s", resp.StatusCode, string(respBody))
			// 4xx错误不重试，5xx错误可以重试
			if resp.StatusCode >= 500 && attempt < maxRetries {
				waitTime := time.Duration(attempt) * 2 * time.Second
				log.Printf("⚠️  OKX API服务器错误（尝试 %d/%d），%v后重试: %v", attempt, maxRetries, waitTime, lastErr)
				time.Sleep(waitTime)
				continue
			}
			return nil, lastErr
		}

		// 解析OKX响应格式
		var okxResp struct {
			Code string          `json:"code"`
			Msg  string          `json:"msg"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(respBody, &okxResp); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		if okxResp.Code != "0" {
			// V1.68版本：增强错误日志，记录完整的API响应和请求信息
			log.Printf("  ❌ OKX API错误: code=%s, msg=%s", okxResp.Code, okxResp.Msg)
			log.Printf("  📋 请求路径: %s %s", method, path)
			if body != nil {
				bodyBytes, _ := json.Marshal(body)
				log.Printf("  📋 请求体: %s", string(bodyBytes))
			}
			log.Printf("  📋 完整响应: %s", string(respBody))
			
			// 解析响应数据（如果有详细信息）
			if len(okxResp.Data) > 0 {
				var errorData []struct {
					SCode string `json:"sCode"`
					SMsg  string `json:"sMsg"`
				}
				if err := json.Unmarshal(okxResp.Data, &errorData); err == nil && len(errorData) > 0 {
					log.Printf("  📋 错误详情: sCode=%s, sMsg=%s", errorData[0].SCode, errorData[0].SMsg)
				}
			}
			
			return nil, fmt.Errorf("OKX API错误: %s - %s", okxResp.Code, okxResp.Msg)
		}

		return okxResp.Data, nil
	}
	
	// 所有重试都失败
	return nil, fmt.Errorf("OKX API请求失败（已重试%d次）: %w", maxRetries, lastErr)
}

// GetBalance 获取账户余额（带缓存）
func (t *OKXTrader) GetBalance() (map[string]interface{}, error) {
	// 先检查缓存是否有效
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的账户余额（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// 缓存过期或不存在，调用API
	log.Printf("🔄 缓存过期，正在调用OKX API获取账户余额...")
	data, err := t.makeRequest("GET", "/api/v5/account/balance", nil)
	if err != nil {
		log.Printf("❌ OKX API调用失败: %v", err)
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}

	// 解析余额数据（使用OKX标准字段）
	var balanceList []struct {
		Details []struct {
			Currency   string `json:"ccy"`      // 币种
			Balance    string `json:"eq"`       // 币种权益
			Available  string `json:"availEq"`  // 可用余额
			Frozen     string `json:"frozenBal"` // 冻结余额
			MarginUsed string `json:"mgnRatio"` // 保证金率（该币种）
		} `json:"details"`
		TotalEq    string `json:"totalEq"`    // 总权益
		IsoEq      string `json:"isoEq"`     // 逐仓权益
		AdjEq      string `json:"adjEq"`      // 美金层面权益
		MgnRatio   string `json:"mgnRatio"`   // 美金层面有效保证金率
		Notional   string `json:"notionalUsd"` // 美金层面持仓数量
		Utime      string `json:"uTime"`      // 更新时间
	}

	if err := json.Unmarshal(data, &balanceList); err != nil {
		return nil, fmt.Errorf("解析余额数据失败: %w", err)
	}

	if len(balanceList) == 0 {
		return nil, fmt.Errorf("未找到余额信息")
	}

	balance := balanceList[0]
	totalEq, _ := strconv.ParseFloat(balance.TotalEq, 64)
	adjEq, _ := strconv.ParseFloat(balance.AdjEq, 64)
	mgnRatio, _ := strconv.ParseFloat(balance.MgnRatio, 64)
	notional, _ := strconv.ParseFloat(balance.Notional, 64)
	
	// 查找USDT余额
	var availableEq float64
	for _, detail := range balance.Details {
		if detail.Currency == "USDT" {
			availableEq, _ = strconv.ParseFloat(detail.Available, 64)
			break
		}
	}

	// 计算未实现盈亏（需要从持仓中获取，这里先设为0，后续在GetAccountInfo中计算）
	result := make(map[string]interface{})
	result["totalWalletBalance"] = totalEq
	result["totalEquity"] = adjEq // 使用adjEq作为总权益（美金层面）
	result["availableBalance"] = availableEq
	result["totalUnrealizedProfit"] = 0.0 // 需要从持仓计算
	result["mgnRatio"] = mgnRatio         // OKX标准保证金率
	result["notionalUsd"] = notional      // 持仓名义价值
	result["isoEq"] = balance.IsoEq       // 逐仓权益

	log.Printf("✓ OKX API返回: 总权益=%.2f, 可用=%.2f, 保证金率=%.4f, 名义价值=%.2f", adjEq, availableEq, mgnRatio, notional)

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions 获取所有持仓（带缓存）
func (t *OKXTrader) GetPositions() ([]map[string]interface{}, error) {
	// 先检查缓存是否有效
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的持仓信息（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// 缓存过期或不存在，调用API
	log.Printf("🔄 缓存过期，正在调用OKX API获取持仓信息...")
	data, err := t.makeRequest("GET", "/api/v5/account/positions", nil)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	// 解析持仓数据（使用OKX标准字段）
	var positions []struct {
		InstID      string `json:"instId"`      // 交易对ID
		Pos         string `json:"pos"`         // 持仓数量（正数=多，负数=空）
		AvgPx       string `json:"avgPx"`        // 开仓均价
		MarkPx      string `json:"markPx"`      // 标记价格
		Upl         string `json:"upl"`         // 未实现盈亏
		UplRatio    string `json:"uplRatio"`    // 未实现盈亏率
		Lever       string `json:"lever"`       // 杠杆倍数
		LiqPx       string `json:"liqPx"`       // 强平价格
		PosSide     string `json:"posSide"`     // 持仓方向: "long" or "short"
		MgnMode     string `json:"mgnMode"`      // 保证金模式: "isolated" or "cross"
		Margin      string `json:"margin"`       // 保证金
		NotionalUsd string `json:"notionalUsd"` // 名义价值（USD）
		Imr         string `json:"imr"`          // 初始保证金率
		Mmr         string `json:"mmr"`         // 维持保证金率
		Interest    string `json:"interest"`    // 利息
		Fee         string `json:"fee"`          // 手续费
		Last        string `json:"last"`         // 最新成交价
		Ccy         string `json:"ccy"`          // 保证金币种
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("解析持仓数据失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		posAmt, _ := strconv.ParseFloat(pos.Pos, 64)
		if posAmt == 0 {
			continue // 跳过无持仓的
		}

		// 转换OKX交易对格式 (BTC-USDT-SWAP -> BTCUSDT)
		symbol := strings.ReplaceAll(pos.InstID, "-USDT-SWAP", "USDT")
		symbol = strings.ReplaceAll(symbol, "-", "")

		// 解析所有字段
		entryPrice, _ := strconv.ParseFloat(pos.AvgPx, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPx, 64)
		unRealizedProfit, _ := strconv.ParseFloat(pos.Upl, 64)
		unRealizedProfitRatio, _ := strconv.ParseFloat(pos.UplRatio, 64)
		leverage, _ := strconv.ParseFloat(pos.Lever, 64)
		liquidationPrice, _ := strconv.ParseFloat(pos.LiqPx, 64)
		margin, _ := strconv.ParseFloat(pos.Margin, 64)
		notionalUsd, _ := strconv.ParseFloat(pos.NotionalUsd, 64)
		imr, _ := strconv.ParseFloat(pos.Imr, 64)
		mmr, _ := strconv.ParseFloat(pos.Mmr, 64)

		// 确保posAmt为正数（使用绝对值）
		if posAmt < 0 {
			posAmt = -posAmt
		}

		posMap := make(map[string]interface{})
		posMap["symbol"] = symbol
		posMap["positionAmt"] = posAmt
		posMap["entryPrice"] = entryPrice
		posMap["markPrice"] = markPrice
		posMap["unRealizedProfit"] = unRealizedProfit
		posMap["unRealizedProfitRatio"] = unRealizedProfitRatio
		posMap["leverage"] = leverage
		posMap["liquidationPrice"] = liquidationPrice
		posMap["margin"] = margin
		posMap["notionalUsd"] = notionalUsd
		posMap["marginMode"] = pos.MgnMode
		posMap["imr"] = imr
		posMap["mmr"] = mmr

		// 判断方向：直接使用OKX API返回的posSide字段（"long"或"short"）
		// OKX标准：posSide字段明确标识方向
		if pos.PosSide == "long" {
			posMap["side"] = "long"
		} else if pos.PosSide == "short" {
			posMap["side"] = "short"
		} else {
			// 兼容处理：如果posSide为空，根据pos数量判断
			originalPos, _ := strconv.ParseFloat(pos.Pos, 64)
			if originalPos > 0 {
				posMap["side"] = "long"
			} else {
				posMap["side"] = "short"
			}
			log.Printf("⚠️  OKX持仓方向未知(posSide=%s)，使用数量判断: %s", pos.PosSide, posMap["side"])
		}

		result = append(result, posMap)
	}

	// 更新缓存
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// SetMarginMode 设置仓位模式
func (t *OKXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// 转换交易对格式 (BTCUSDT -> BTC-USDT-SWAP)
	instID := t.convertSymbolToInstID(symbol)
	
	mgnMode := "isolated"
	if isCrossMargin {
		mgnMode = "cross"
	}

	reqBody := map[string]interface{}{
		"instId": instID,
		"mgnMode": mgnMode,
	}

	_, err := t.makeRequest("POST", "/api/v5/account/set-position-mode", reqBody)
	if err != nil {
		// OKX可能返回"Position mode is already set"错误，可以忽略
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "Position mode") {
			marginModeStr := "全仓"
			if !isCrossMargin {
				marginModeStr = "逐仓"
			}
			log.Printf("  ✓ %s 仓位模式已是 %s", symbol, marginModeStr)
			return nil
		}
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		return nil // 不返回错误，让交易继续
	}

	marginModeStr := "全仓"
	if !isCrossMargin {
		marginModeStr = "逐仓"
	}
	log.Printf("  ✓ %s 仓位模式已设置为 %s", symbol, marginModeStr)
	return nil
}

// SetLeverage 设置杠杆（OKX逐仓模式需要posSide参数）
func (t *OKXTrader) SetLeverage(symbol string, leverage int) error {
	return t.SetLeverageWithPosSide(symbol, leverage, "")
}

// SetLeverageWithPosSide 设置杠杆（带posSide参数，用于逐仓模式）
func (t *OKXTrader) SetLeverageWithPosSide(symbol string, leverage int, posSide string) error {
	// 转换交易对格式
	instID := t.convertSymbolToInstID(symbol)

	reqBody := map[string]interface{}{
		"instId":  instID,
		"lever":   strconv.Itoa(leverage),
		"mgnMode": "isolated", // 逐仓模式需要设置杠杆
	}

	// OKX逐仓模式必须指定posSide（"long"或"short"）
	// 如果未指定，同时设置多空两个方向
	if posSide == "" {
		// 同时设置多空两个方向的杠杆
		reqBody["posSide"] = "long"
		_, err1 := t.makeRequest("POST", "/api/v5/account/set-leverage", reqBody)
		if err1 != nil && !strings.Contains(err1.Error(), "already") && !strings.Contains(err1.Error(), "No need") {
			log.Printf("  ⚠️ 设置多仓杠杆失败: %v", err1)
		}

		reqBody["posSide"] = "short"
		_, err2 := t.makeRequest("POST", "/api/v5/account/set-leverage", reqBody)
		if err2 != nil {
			// 如果错误信息包含"already"，说明杠杆已经是目标值
			if strings.Contains(err2.Error(), "already") || strings.Contains(err2.Error(), "No need") {
				log.Printf("  ✓ %s 杠杆已是 %dx", symbol, leverage)
				return nil
			}
			return fmt.Errorf("设置杠杆失败: %w", err2)
		}
		log.Printf("  ✓ %s 多空杠杆已切换为 %dx", symbol, leverage)
	} else {
		// 指定方向设置杠杆
		reqBody["posSide"] = posSide
		_, err := t.makeRequest("POST", "/api/v5/account/set-leverage", reqBody)
		if err != nil {
			// 如果错误信息包含"already"，说明杠杆已经是目标值
			if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "No need") {
				log.Printf("  ✓ %s %s 杠杆已是 %dx", symbol, posSide, leverage)
				return nil
			}
			return fmt.Errorf("设置杠杆失败: %w", err)
		}
		log.Printf("  ✓ %s %s 杠杆已切换为 %dx", symbol, posSide, leverage)
	}

	// 切换杠杆后等待5秒（避免冷却期错误）
	log.Printf("  ⏱ 等待5秒冷却期...")
	time.Sleep(5 * time.Second)

	return nil
}

// OpenLong 开多仓（V1.57版本：支持下单时设置止盈止损）
func (t *OKXTrader) OpenLong(symbol string, quantity float64, leverage int, stopLoss, takeProfit float64) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败（可能没有委托单）: %v", err)
	}

	// 设置杠杆（开多仓，指定long方向）
	if err := t.SetLeverageWithPosSide(symbol, leverage, "long"); err != nil {
		return nil, err
	}

	// 转换交易对格式
	instID := t.convertSymbolToInstID(symbol)

	// V1.67版本：改进数量计算和验证逻辑
	// 先获取当前价格和账户余额，用于验证格式化后的数量
	currentPrice, priceErr := t.GetMarketPrice(symbol)
	if priceErr != nil {
		return nil, fmt.Errorf("获取当前价格失败: %w", priceErr)
	}
	
	// 获取账户余额
	balance, balanceErr := t.GetBalance()
	if balanceErr != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", balanceErr)
	}
	availableBalance, _ := balance["availableBalance"].(float64)
	
	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}
	
	// 解析格式化后的数量
	formattedQuantity, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("解析格式化后的数量失败: %w", parseErr)
	}
	
	// V1.67版本：验证格式化后的数量
	// 计算格式化后的数量对应的仓位价值
	formattedPositionValue := formattedQuantity * currentPrice
	formattedMarginRequired := formattedPositionValue / float64(leverage)
	
	log.Printf("  📊 数量验证: 原始=%.8f, 格式化=%s (%.8f)", quantity, quantityStr, formattedQuantity)
	log.Printf("  💰 仓位价值: 原始=%.2f USDT, 格式化后=%.2f USDT", quantity*currentPrice, formattedPositionValue)
	log.Printf("  💰 所需保证金: 原始=%.2f USDT, 格式化后=%.2f USDT (可用余额=%.2f USDT)", 
		(quantity*currentPrice)/float64(leverage), formattedMarginRequired, availableBalance)
	
	// 检查格式化后的数量是否导致保证金不足
	if formattedMarginRequired > availableBalance {
		// 获取lotSz以提供更详细的错误信息
		lotSz, _ := t.GetSymbolLotSz(symbol)
		minPositionValue := lotSz * currentPrice
		minMarginRequired := minPositionValue / float64(leverage)
		
		return nil, fmt.Errorf("格式化后的数量导致保证金不足: 需要 %.2f USDT，但只有 %.2f USDT可用。最小可交易数量 %.8f 对应的仓位价值为 %.2f USDT，所需保证金为 %.2f USDT。建议：1) 降低杠杆倍数；2) 增加账户余额；3) 选择价格更低的币种", 
			formattedMarginRequired, availableBalance, lotSz, minPositionValue, minMarginRequired)
	}
	
	// 如果格式化后的数量大幅超过原始数量（超过10%），发出警告
	if formattedQuantity > quantity*1.1 {
		log.Printf("  ⚠️ 警告: 格式化后的数量 (%.8f) 比原始数量 (%.8f) 大 %.2f%%，仓位价值从 %.2f USDT 增加到 %.2f USDT",
			formattedQuantity, quantity, (formattedQuantity/quantity-1)*100, 
			quantity*currentPrice, formattedPositionValue)
	}
	
	// V1.68版本：在下单前验证止损/止盈价格是否合理
	if stopLoss > 0 {
		// 计算爆仓价
		liquidationPrice := currentPrice * (1 - 1.0/float64(leverage))
		// 做多时：止损应该低于当前价，但必须高于爆仓价
		if stopLoss >= currentPrice {
			return nil, fmt.Errorf("止损价设置不合理: 做多时止损价 (%.4f) 应该低于当前价 (%.4f)", stopLoss, currentPrice)
		}
		if stopLoss <= liquidationPrice {
			return nil, fmt.Errorf("止损价设置不合理: 止损价 (%.4f) 必须高于爆仓价 (%.4f)，否则止损单可能失效导致直接爆仓", stopLoss, liquidationPrice)
		}
		log.Printf("  ✓ 止损价验证通过: 当前价=%.4f, 爆仓价=%.4f, 止损价=%.4f", currentPrice, liquidationPrice, stopLoss)
	}
	
	if takeProfit > 0 {
		// 做多时：止盈应该高于当前价
		if takeProfit <= currentPrice {
			return nil, fmt.Errorf("止盈价设置不合理: 做多时止盈价 (%.4f) 应该高于当前价 (%.4f)", takeProfit, currentPrice)
		}
		// 检查止盈和止损的逻辑关系
		if stopLoss > 0 && stopLoss >= takeProfit {
			return nil, fmt.Errorf("止损和止盈设置不合理: 做多时止损 (%.4f) 应该低于止盈 (%.4f)", stopLoss, takeProfit)
		}
		log.Printf("  ✓ 止盈价验证通过: 当前价=%.4f, 止盈价=%.4f", currentPrice, takeProfit)
	}
	
	// 创建市价买入订单
	reqBody := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "isolated", // 逐仓模式
		"side":    "buy",
		"ordType": "market",
		"sz":      quantityStr,
		"posSide": "long",
	}

	// V1.57版本：如果提供了止盈止损价格，在下单时设置
	// OKX API使用attachAlgoOrds参数来附加止盈止损订单
	// 注意：OKX API要求每个attachAlgoOrds对象必须包含完整的参数，不能只设置部分字段
	if stopLoss > 0 || takeProfit > 0 {
		attachAlgoOrds := []map[string]interface{}{}
		
		// 设置止损（多仓：止损价低于当前价，使用stop_market订单类型）
		if stopLoss > 0 {
			stopLossOrder := map[string]interface{}{
				"attachAlgoClOrdId": fmt.Sprintf("sl_%s_%d", symbol, time.Now().UnixMilli()),
				"slTriggerPx": fmt.Sprintf("%.8f", stopLoss),
				"slTriggerPxType": "last",  // 触发价格类型：last表示最新价
				"slOrdPx": "-1",            // -1表示市价单（止损时立即以市价成交）
				"sz": quantityStr,
				"reduceOnly": true,         // 仅减仓
			}
			attachAlgoOrds = append(attachAlgoOrds, stopLossOrder)
			log.Printf("  📌 下单时设置止损: %.4f (触发价类型: last)", stopLoss)
		}
		
		// 设置止盈（多仓：止盈价高于当前价，使用take_profit_market订单类型）
		if takeProfit > 0 {
			takeProfitOrder := map[string]interface{}{
				"attachAlgoClOrdId": fmt.Sprintf("tp_%s_%d", symbol, time.Now().UnixMilli()),
				"tpTriggerPx": fmt.Sprintf("%.8f", takeProfit),
				"tpTriggerPxType": "last",  // 触发价格类型：last表示最新价
				"tpOrdPx": "-1",            // -1表示市价单（止盈时立即以市价成交）
				"sz": quantityStr,
				"reduceOnly": true,         // 仅减仓
			}
			attachAlgoOrds = append(attachAlgoOrds, takeProfitOrder)
			log.Printf("  📌 下单时设置止盈: %.4f (触发价类型: last)", takeProfit)
		}
		
		if len(attachAlgoOrds) > 0 {
			reqBody["attachAlgoOrds"] = attachAlgoOrds
			log.Printf("  ✅ 将在下单时同时设置 %d 个附加算法订单（止盈止损）", len(attachAlgoOrds))
		}
	}

	// V1.65版本：增强日志记录，记录完整的请求参数用于诊断
	log.Printf("  📋 开仓请求参数: instId=%s, tdMode=%s, side=%s, ordType=%s, sz=%s, posSide=%s", 
		instID, reqBody["tdMode"], reqBody["side"], reqBody["ordType"], quantityStr, reqBody["posSide"])
	if stopLoss > 0 {
		log.Printf("  📋 止损参数: stopLoss=%.4f", stopLoss)
	}
	if takeProfit > 0 {
		log.Printf("  📋 止盈参数: takeProfit=%.4f", takeProfit)
	}
	
	// V1.68版本：记录完整的请求参数（JSON格式）
	reqBodyJSON, _ := json.MarshalIndent(reqBody, "", "  ")
	log.Printf("  📋 完整请求参数 (JSON):\n%s", string(reqBodyJSON))
	
	data, err := t.makeRequest("POST", "/api/v5/trade/order", reqBody)
	if err != nil {
		// V1.68版本：增强错误诊断信息，记录完整请求和响应
		log.Printf("  ❌ 开多仓API请求失败: %v", err)
		log.Printf("  📋 请求详情: 币种=%s, 数量=%s (原始=%.8f), 杠杆=%d, 止损=%.4f, 止盈=%.4f", 
			symbol, quantityStr, quantity, leverage, stopLoss, takeProfit)
		log.Printf("  📋 完整请求参数 (JSON):\n%s", string(reqBodyJSON))
		
		// 检查账户余额
		balance, balanceErr := t.GetBalance()
		if balanceErr == nil {
			if totalEq, ok := balance["totalEquity"].(float64); ok {
				log.Printf("  💰 账户净值: %.2f USDT", totalEq)
			}
			if available, ok := balance["availableBalance"].(float64); ok {
				log.Printf("  💰 可用余额: %.2f USDT", available)
				log.Printf("  💰 格式化后所需保证金: %.2f USDT", formattedMarginRequired)
			}
		}
		
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}
	
	// V1.68版本：记录完整的API响应
	log.Printf("  📋 OKX API完整响应: %s", string(data))

	// 解析订单响应
	var orderResp []struct {
		OrdID  string `json:"ordId"`
		InstID string `json:"instId"`
		SCode  string `json:"sCode"`
		SMsg   string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orderResp); err != nil {
		log.Printf("  ❌ 解析订单响应失败: %v, 原始响应: %s", err, string(data))
		return nil, fmt.Errorf("解析订单响应失败: %w, 原始响应: %s", err, string(data))
	}

	if len(orderResp) == 0 {
		log.Printf("  ❌ 订单响应为空，原始响应: %s", string(data))
		return nil, fmt.Errorf("订单响应为空，原始响应: %s", string(data))
	}

	order := orderResp[0]
	if order.SCode != "0" {
		// V1.75版本：增强错误诊断信息，添加常见错误代码的解决方案
		log.Printf("  ❌ 开多仓失败: 错误代码=%s, 错误信息=%s", order.SCode, order.SMsg)
		log.Printf("  📋 请求详情: 币种=%s, 数量=%s (原始=%.8f, 格式化后=%.8f), 杠杆=%d, 止损=%.4f, 止盈=%.4f", 
			symbol, quantityStr, quantity, formattedQuantity, leverage, stopLoss, takeProfit)
		log.Printf("  📋 完整请求参数 (JSON):\n%s", string(reqBodyJSON))
		log.Printf("  📋 OKX API完整响应: %s", string(data))
		log.Printf("  📊 数量验证结果: 原始=%.8f, 格式化后=%.8f, 仓位价值=%.2f USDT, 所需保证金=%.2f USDT", 
			quantity, formattedQuantity, formattedPositionValue, formattedMarginRequired)
		
		// V1.75版本：针对常见错误代码提供解决方案
		switch order.SCode {
		case "51000":
			log.Printf("  🔍 错误分析: 参数错误 (51000)")
			log.Printf("     - 检查 posSide 参数是否正确（开多仓应为 'long'）")
			log.Printf("     - 检查 tdMode 参数是否正确（应为 'isolated' 或 'cross'）")
			log.Printf("     - 检查 instId 格式是否正确（应为 'BTC-USDT-SWAP' 格式）")
		case "51001":
			log.Printf("  🔍 错误分析: 参数值为空 (51001)")
			log.Printf("     - 检查所有必填参数是否都已填写")
			log.Printf("     - 检查 sz（数量）参数是否为空")
		case "51002":
			log.Printf("  🔍 错误分析: 参数值错误 (51002)")
			log.Printf("     - 检查数量是否小于最小交易量")
			log.Printf("     - 检查价格精度是否正确")
		case "51003":
			log.Printf("  🔍 错误分析: 参数类型错误 (51003)")
			log.Printf("     - 检查参数类型是否正确（数量应为字符串格式）")
		case "51004":
			log.Printf("  🔍 错误分析: 参数值超出范围 (51004)")
			log.Printf("     - 检查杠杆倍数是否在允许范围内")
			log.Printf("     - 检查数量是否超过最大持仓限制")
		case "51005":
			log.Printf("  🔍 错误分析: 请求频率过高 (51005)")
			log.Printf("     - 降低交易频率，等待后重试")
		case "51006":
			log.Printf("  🔍 错误分析: 账户余额不足 (51006)")
			log.Printf("     - 检查账户可用余额是否足够")
			log.Printf("     - 降低杠杆倍数或减少交易数量")
		case "51007":
			log.Printf("  🔍 错误分析: 持仓模式不匹配 (51007)")
			log.Printf("     - 检查账户持仓模式设置")
			log.Printf("     - 确保使用正确的 tdMode（isolated 或 cross）")
		case "51008":
			log.Printf("  🔍 错误分析: 杠杆设置失败 (51008)")
			log.Printf("     - 检查杠杆倍数是否在允许范围内")
			log.Printf("     - 检查是否有未平仓持仓")
		case "51009":
			log.Printf("  🔍 错误分析: 订单类型不支持 (51009)")
			log.Printf("     - 检查 ordType 参数是否正确")
		case "51010":
			log.Printf("  🔍 错误分析: 交易对不存在或已下架 (51010)")
			log.Printf("     - 检查交易对符号是否正确")
			log.Printf("     - 检查交易对是否仍在交易")
		case "51011":
			log.Printf("  🔍 错误分析: API权限不足 (51011)")
			log.Printf("     - 检查API密钥是否有交易权限")
			log.Printf("     - 在OKX网站上检查API密钥权限设置")
			log.Printf("     - 确保API密钥有'合约交易'权限")
		case "51012":
			log.Printf("  🔍 错误分析: 账户被限制交易 (51012)")
			log.Printf("     - 检查账户状态是否正常")
			log.Printf("     - 联系OKX客服检查账户限制")
		default:
			log.Printf("  🔍 错误分析: 未知错误代码 %s", order.SCode)
			log.Printf("     - 查看OKX API文档获取更多信息")
			log.Printf("     - 检查错误信息: %s", order.SMsg)
		}
		
		// 获取当前价格（用于计算所需保证金）
		currentPrice, priceErr := t.GetMarketPrice(symbol)
		if priceErr != nil {
			log.Printf("  ⚠️ 获取当前价格失败: %v", priceErr)
		}
		
		// 检查账户余额
		balance, balanceErr := t.GetBalance()
		if balanceErr == nil {
			if totalEq, ok := balance["totalEquity"].(float64); ok {
				log.Printf("  💰 账户净值: %.2f USDT", totalEq)
			}
			if available, ok := balance["availableBalance"].(float64); ok {
				log.Printf("  💰 可用余额: %.2f USDT", available)
				// 计算所需保证金（如果获取到当前价格）
				if priceErr == nil && currentPrice > 0 {
					positionValue := quantity * currentPrice
					marginRequired := positionValue / float64(leverage)
					log.Printf("  💰 所需保证金: %.2f USDT (仓位价值=%.2f / 杠杆=%d)", 
						marginRequired, positionValue, leverage)
					if available < marginRequired {
						log.Printf("  ⚠️ 可用余额不足！需要 %.2f USDT，但只有 %.2f USDT", marginRequired, available)
					}
					
					// 检查止损是否合理（如果设置了止损）
					if stopLoss > 0 {
						// 计算爆仓价
						liquidationPrice := currentPrice * (1 - 1.0/float64(leverage))
						log.Printf("  💰 当前价格: %.4f, 爆仓价: %.4f, 止损价: %.4f", 
							currentPrice, liquidationPrice, stopLoss)
						if stopLoss <= liquidationPrice {
							log.Printf("  ⚠️ 止损价低于或等于爆仓价！止损价必须在爆仓价上方")
							log.Printf("     做多时: 止损价必须 > 爆仓价 (%.4f)", liquidationPrice)
						}
					}
				}
			}
		}
		
		// 检查数量格式化后的值
		if quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64); parseErr == nil {
			if quantityFloat <= 0 {
				log.Printf("  ⚠️ 格式化后的数量为0或负数: %s", quantityStr)
			}
		} else {
			log.Printf("  ⚠️ 无法解析格式化后的数量: %s", quantityStr)
		}
		
		// V1.75版本：针对错误代码"1"提供额外诊断建议
		if order.SCode == "1" {
			log.Printf("  💡 额外诊断建议: 错误代码1 ('All operations failed') 通常表示:")
			log.Printf("     - 账户余额不足（检查可用余额和所需保证金）")
			log.Printf("     - 止损/止盈价格设置不合理（止损可能低于爆仓价）")
			log.Printf("     - 数量格式错误或数量为0（检查格式化后的数量）")
			log.Printf("     - 杠杆设置失败或杠杆倍数不符合要求（检查杠杆设置日志）")
			log.Printf("     - API权限不足（检查API密钥是否有交易权限）")
			log.Printf("     - 订单参数错误（检查instId、tdMode、side等参数）")
		}
		
		return nil, fmt.Errorf("开多仓失败: %s - %s", order.SCode, order.SMsg)
	}

	log.Printf("✓ 开多仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %s", order.OrdID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrdID
	result["symbol"] = symbol
	result["status"] = "filled"
	return result, nil
}

// OpenShort 开空仓（V1.57版本：支持下单时设置止盈止损）
func (t *OKXTrader) OpenShort(symbol string, quantity float64, leverage int, stopLoss, takeProfit float64) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败（可能没有委托单）: %v", err)
	}

	// 设置杠杆（开空仓，指定short方向）
	if err := t.SetLeverageWithPosSide(symbol, leverage, "short"); err != nil {
		return nil, err
	}

	// 转换交易对格式
	instID := t.convertSymbolToInstID(symbol)

	// V1.68版本：改进数量计算和验证逻辑
	// 先获取当前价格和账户余额，用于验证格式化后的数量
	currentPrice, priceErr := t.GetMarketPrice(symbol)
	if priceErr != nil {
		return nil, fmt.Errorf("获取当前价格失败: %w", priceErr)
	}
	
	// 获取账户余额
	balance, balanceErr := t.GetBalance()
	if balanceErr != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", balanceErr)
	}
	availableBalance, _ := balance["availableBalance"].(float64)
	
	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}
	
	// 解析格式化后的数量
	formattedQuantity, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("解析格式化后的数量失败: %w", parseErr)
	}
	
	// V1.68版本：验证格式化后的数量
	// 计算格式化后的数量对应的仓位价值
	formattedPositionValue := formattedQuantity * currentPrice
	formattedMarginRequired := formattedPositionValue / float64(leverage)
	
	log.Printf("  📊 数量验证: 原始=%.8f, 格式化=%s (%.8f)", quantity, quantityStr, formattedQuantity)
	log.Printf("  💰 仓位价值: 原始=%.2f USDT, 格式化后=%.2f USDT", quantity*currentPrice, formattedPositionValue)
	log.Printf("  💰 所需保证金: 原始=%.2f USDT, 格式化后=%.2f USDT (可用余额=%.2f USDT)", 
		(quantity*currentPrice)/float64(leverage), formattedMarginRequired, availableBalance)
	
	// 检查格式化后的数量是否导致保证金不足
	if formattedMarginRequired > availableBalance {
		// 获取lotSz以提供更详细的错误信息
		lotSz, _ := t.GetSymbolLotSz(symbol)
		minPositionValue := lotSz * currentPrice
		minMarginRequired := minPositionValue / float64(leverage)
		
		return nil, fmt.Errorf("格式化后的数量导致保证金不足: 需要 %.2f USDT，但只有 %.2f USDT可用。最小可交易数量 %.8f 对应的仓位价值为 %.2f USDT，所需保证金为 %.2f USDT。建议：1) 降低杠杆倍数；2) 增加账户余额；3) 选择价格更低的币种", 
			formattedMarginRequired, availableBalance, lotSz, minPositionValue, minMarginRequired)
	}
	
	// 如果格式化后的数量大幅超过原始数量（超过10%），发出警告
	if formattedQuantity > quantity*1.1 {
		log.Printf("  ⚠️ 警告: 格式化后的数量 (%.8f) 比原始数量 (%.8f) 大 %.2f%%，仓位价值从 %.2f USDT 增加到 %.2f USDT",
			formattedQuantity, quantity, (formattedQuantity/quantity-1)*100, 
			quantity*currentPrice, formattedPositionValue)
	}
	
	// V1.69版本：在下单前验证止损/止盈价格是否合理（做空）
	if stopLoss > 0 {
		// 计算爆仓价（做空）
		liquidationPrice := currentPrice * (1 + 1.0/float64(leverage))
		// 做空时：止损应该高于当前价，但必须低于爆仓价
		if stopLoss <= currentPrice {
			return nil, fmt.Errorf("止损价设置不合理: 做空时止损价 (%.4f) 应该高于当前价 (%.4f)", stopLoss, currentPrice)
		}
		if stopLoss >= liquidationPrice {
			return nil, fmt.Errorf("止损价设置不合理: 止损价 (%.4f) 必须低于爆仓价 (%.4f)，否则止损单可能失效导致直接爆仓", stopLoss, liquidationPrice)
		}
		log.Printf("  ✓ 止损价验证通过: 当前价=%.4f, 爆仓价=%.4f, 止损价=%.4f", currentPrice, liquidationPrice, stopLoss)
	}
	
	if takeProfit > 0 {
		// 做空时：止盈应该低于当前价
		if takeProfit >= currentPrice {
			return nil, fmt.Errorf("止盈价设置不合理: 做空时止盈价 (%.4f) 应该低于当前价 (%.4f)", takeProfit, currentPrice)
		}
		// 检查止盈和止损的逻辑关系
		if stopLoss > 0 && stopLoss <= takeProfit {
			return nil, fmt.Errorf("止损和止盈设置不合理: 做空时止损 (%.4f) 应该高于止盈 (%.4f)", stopLoss, takeProfit)
		}
		log.Printf("  ✓ 止盈价验证通过: 当前价=%.4f, 止盈价=%.4f", currentPrice, takeProfit)
	}
	
	// 创建市价卖出订单
	reqBody := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "isolated",
		"side":    "sell",
		"ordType": "market",
		"sz":      quantityStr,
		"posSide": "short",
	}

	// V1.57版本：如果提供了止盈止损价格，在下单时设置
	// OKX API使用attachAlgoOrds参数来附加止盈止损订单
	// 注意：OKX API要求每个attachAlgoOrds对象必须包含完整的参数，不能只设置部分字段
	if stopLoss > 0 || takeProfit > 0 {
		attachAlgoOrds := []map[string]interface{}{}
		
		// 设置止损（空仓：止损价高于当前价，使用stop_market订单类型）
		if stopLoss > 0 {
			stopLossOrder := map[string]interface{}{
				"attachAlgoClOrdId": fmt.Sprintf("sl_%s_%d", symbol, time.Now().UnixMilli()),
				"slTriggerPx": fmt.Sprintf("%.8f", stopLoss),
				"slTriggerPxType": "last",  // 触发价格类型：last表示最新价
				"slOrdPx": "-1",            // -1表示市价单（止损时立即以市价成交）
				"sz": quantityStr,
				"reduceOnly": true,         // 仅减仓
			}
			attachAlgoOrds = append(attachAlgoOrds, stopLossOrder)
			log.Printf("  📌 下单时设置止损: %.4f (触发价类型: last)", stopLoss)
		}
		
		// 设置止盈（空仓：止盈价低于当前价，使用take_profit_market订单类型）
		if takeProfit > 0 {
			takeProfitOrder := map[string]interface{}{
				"attachAlgoClOrdId": fmt.Sprintf("tp_%s_%d", symbol, time.Now().UnixMilli()),
				"tpTriggerPx": fmt.Sprintf("%.8f", takeProfit),
				"tpTriggerPxType": "last",  // 触发价格类型：last表示最新价
				"tpOrdPx": "-1",            // -1表示市价单（止盈时立即以市价成交）
				"sz": quantityStr,
				"reduceOnly": true,         // 仅减仓
			}
			attachAlgoOrds = append(attachAlgoOrds, takeProfitOrder)
			log.Printf("  📌 下单时设置止盈: %.4f (触发价类型: last)", takeProfit)
		}
		
		if len(attachAlgoOrds) > 0 {
			reqBody["attachAlgoOrds"] = attachAlgoOrds
			log.Printf("  ✅ 将在下单时同时设置 %d 个附加算法订单（止盈止损）", len(attachAlgoOrds))
		}
	}

	// V1.65版本：增强日志记录，记录完整的请求参数用于诊断
	log.Printf("  📋 开仓请求参数: instId=%s, tdMode=%s, side=%s, ordType=%s, sz=%s, posSide=%s", 
		instID, reqBody["tdMode"], reqBody["side"], reqBody["ordType"], quantityStr, reqBody["posSide"])
	if stopLoss > 0 {
		log.Printf("  📋 止损参数: stopLoss=%.4f", stopLoss)
	}
	if takeProfit > 0 {
		log.Printf("  📋 止盈参数: takeProfit=%.4f", takeProfit)
	}

	data, err := t.makeRequest("POST", "/api/v5/trade/order", reqBody)
	if err != nil {
		// V1.65版本：增强错误诊断信息
		log.Printf("  ❌ 开空仓API请求失败: %v", err)
		log.Printf("  📋 请求详情: 币种=%s, 数量=%s, 杠杆=%d, 止损=%.4f, 止盈=%.4f", 
			symbol, quantityStr, leverage, stopLoss, takeProfit)
		
		// 检查账户余额
		balance, balanceErr := t.GetBalance()
		if balanceErr == nil {
			if totalEq, ok := balance["totalEquity"].(float64); ok {
				log.Printf("  💰 账户净值: %.2f USDT", totalEq)
			}
			if available, ok := balance["availableBalance"].(float64); ok {
				log.Printf("  💰 可用余额: %.2f USDT", available)
			}
		}
		
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	// 解析订单响应
	var orderResp []struct {
		OrdID  string `json:"ordId"`
		InstID string `json:"instId"`
		SCode  string `json:"sCode"`
		SMsg   string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orderResp); err != nil {
		log.Printf("  ❌ 解析订单响应失败: %v, 原始响应: %s", err, string(data))
		return nil, fmt.Errorf("解析订单响应失败: %w, 原始响应: %s", err, string(data))
	}

	if len(orderResp) == 0 {
		log.Printf("  ❌ 订单响应为空，原始响应: %s", string(data))
		return nil, fmt.Errorf("订单响应为空，原始响应: %s", string(data))
	}

	order := orderResp[0]
	if order.SCode != "0" {
		// V1.65版本：增强错误诊断信息
		log.Printf("  ❌ 开空仓失败: 错误代码=%s, 错误信息=%s", order.SCode, order.SMsg)
		log.Printf("  📋 请求详情: 币种=%s, 数量=%s (原始=%.8f), 杠杆=%d, 止损=%.4f, 止盈=%.4f", 
			symbol, quantityStr, quantity, leverage, stopLoss, takeProfit)
		log.Printf("  📋 完整响应: %s", string(data))
		
		// 获取当前价格（用于计算所需保证金）
		currentPrice, priceErr := t.GetMarketPrice(symbol)
		if priceErr != nil {
			log.Printf("  ⚠️ 获取当前价格失败: %v", priceErr)
		}
		
		// 检查账户余额
		balance, balanceErr := t.GetBalance()
		if balanceErr == nil {
			if totalEq, ok := balance["totalEquity"].(float64); ok {
				log.Printf("  💰 账户净值: %.2f USDT", totalEq)
			}
			if available, ok := balance["availableBalance"].(float64); ok {
				log.Printf("  💰 可用余额: %.2f USDT", available)
				// 计算所需保证金（如果获取到当前价格）
				if priceErr == nil && currentPrice > 0 {
					positionValue := quantity * currentPrice
					marginRequired := positionValue / float64(leverage)
					log.Printf("  💰 所需保证金: %.2f USDT (仓位价值=%.2f / 杠杆=%d)", 
						marginRequired, positionValue, leverage)
					if available < marginRequired {
						log.Printf("  ⚠️ 可用余额不足！需要 %.2f USDT，但只有 %.2f USDT", marginRequired, available)
					}
					
					// 检查止损是否合理（如果设置了止损）
					if stopLoss > 0 {
						// 计算爆仓价（做空）
						liquidationPrice := currentPrice * (1 + 1.0/float64(leverage))
						log.Printf("  💰 当前价格: %.4f, 爆仓价: %.4f, 止损价: %.4f", 
							currentPrice, liquidationPrice, stopLoss)
						if stopLoss >= liquidationPrice {
							log.Printf("  ⚠️ 止损价高于或等于爆仓价！止损价必须在爆仓价下方")
							log.Printf("     做空时: 止损价必须 < 爆仓价 (%.4f)", liquidationPrice)
						}
					}
				}
			}
		}
		
		// 检查数量格式化后的值
		if quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64); parseErr == nil {
			if quantityFloat <= 0 {
				log.Printf("  ⚠️ 格式化后的数量为0或负数: %s", quantityStr)
			}
		} else {
			log.Printf("  ⚠️ 无法解析格式化后的数量: %s", quantityStr)
		}
		
		// 根据错误代码提供诊断建议
		switch order.SCode {
		case "1":
			log.Printf("  💡 诊断建议: 错误代码1 ('All operations failed') 通常表示:")
			log.Printf("     - 账户余额不足（检查可用余额和所需保证金）")
			log.Printf("     - 止损/止盈价格设置不合理（做空时止损可能高于爆仓价）")
			log.Printf("     - 数量格式错误或数量为0（检查格式化后的数量）")
			log.Printf("     - 杠杆设置失败或杠杆倍数不符合要求（检查杠杆设置日志）")
			log.Printf("     - API权限不足（检查API密钥是否有交易权限）")
			log.Printf("     - 订单参数错误（检查instId、tdMode、side等参数）")
		case "51008":
			log.Printf("  💡 诊断建议: 错误代码51008表示订单失败，可能是:")
			log.Printf("     - 账户余额不足")
			log.Printf("     - 订单参数错误")
			log.Printf("     - 数量或价格格式错误")
		}
		
		return nil, fmt.Errorf("开空仓失败: %s - %s", order.SCode, order.SMsg)
	}

	log.Printf("✓ 开空仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %s", order.OrdID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrdID
	result["symbol"] = symbol
	result["status"] = "filled"
	return result, nil
}

// CloseLong 平多仓
func (t *OKXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的多仓", symbol)
		}
	}

	// 转换交易对格式
	instID := t.convertSymbolToInstID(symbol)

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 创建市价卖出订单（平多）
	reqBody := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "isolated",
		"side":    "sell",
		"ordType": "market",
		"sz":      quantityStr,
		"posSide": "long",
		"reduceOnly": true,
	}

	data, err := t.makeRequest("POST", "/api/v5/trade/order", reqBody)
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: %w", err)
	}

	// 解析订单响应
	var orderResp []struct {
		OrdID  string `json:"ordId"`
		InstID string `json:"instId"`
		SCode  string `json:"sCode"`
		SMsg   string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orderResp); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	if len(orderResp) == 0 {
		return nil, fmt.Errorf("订单响应为空")
	}

	order := orderResp[0]
	if order.SCode != "0" {
		return nil, fmt.Errorf("平多仓失败: %s - %s", order.SCode, order.SMsg)
	}

	log.Printf("✓ 平多仓成功: %s 数量: %s", symbol, quantityStr)

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrdID
	result["symbol"] = symbol
	result["status"] = "filled"
	return result, nil
}

// CloseShort 平空仓
func (t *OKXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = -pos["positionAmt"].(float64) // 空仓数量是负的，取绝对值
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
		}
	}

	// 转换交易对格式
	instID := t.convertSymbolToInstID(symbol)

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 创建市价买入订单（平空）
	reqBody := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "isolated",
		"side":    "buy",
		"ordType": "market",
		"sz":      quantityStr,
		"posSide": "short",
		"reduceOnly": true,
	}

	data, err := t.makeRequest("POST", "/api/v5/trade/order", reqBody)
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: %w", err)
	}

	// 解析订单响应
	var orderResp []struct {
		OrdID  string `json:"ordId"`
		InstID string `json:"instId"`
		SCode  string `json:"sCode"`
		SMsg   string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orderResp); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	if len(orderResp) == 0 {
		return nil, fmt.Errorf("订单响应为空")
	}

	order := orderResp[0]
	if order.SCode != "0" {
		return nil, fmt.Errorf("平空仓失败: %s - %s", order.SCode, order.SMsg)
	}

	log.Printf("✓ 平空仓成功: %s 数量: %s", symbol, quantityStr)

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrdID
	result["symbol"] = symbol
	result["status"] = "filled"
	return result, nil
}

// CancelStopLossOrders 仅取消止损单
func (t *OKXTrader) CancelStopLossOrders(symbol string) error {
	instID := t.convertSymbolToInstID(symbol)
	
	// 获取该币种的所有未完成订单
	data, err := t.makeRequest("GET", fmt.Sprintf("/api/v5/trade/orders-pending?instId=%s", instID), nil)
	if err != nil {
		return fmt.Errorf("获取未完成订单失败: %w", err)
	}

	var orders []struct {
		OrdID  string `json:"ordId"`
		InstID string `json:"instId"`
		OrdType string `json:"ordType"`
		PosSide string `json:"posSide"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return fmt.Errorf("解析订单列表失败: %w", err)
	}

	// 过滤出止损单并取消
	canceledCount := 0
	for _, order := range orders {
		// OKX的止损单类型: stop_market, stop
		if order.OrdType == "stop_market" || order.OrdType == "stop" {
			cancelBody := map[string]interface{}{
				"instId": instID,
				"ordId":  order.OrdID,
			}

			_, err := t.makeRequest("POST", "/api/v5/trade/cancel-order", cancelBody)
			if err != nil {
				log.Printf("  ⚠ 取消止损单失败: %v", err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ 已取消止损单 (订单ID: %s, 类型: %s, 方向: %s)", order.OrdID, order.OrdType, order.PosSide)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止损单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止损单", symbol, canceledCount)
	}

	return nil
}

// CancelTakeProfitOrders 仅取消止盈单
func (t *OKXTrader) CancelTakeProfitOrders(symbol string) error {
	instID := t.convertSymbolToInstID(symbol)
	
	// 获取该币种的所有未完成订单
	data, err := t.makeRequest("GET", fmt.Sprintf("/api/v5/trade/orders-pending?instId=%s", instID), nil)
	if err != nil {
		return fmt.Errorf("获取未完成订单失败: %w", err)
	}

	var orders []struct {
		OrdID  string `json:"ordId"`
		InstID string `json:"instId"`
		OrdType string `json:"ordType"`
		PosSide string `json:"posSide"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return fmt.Errorf("解析订单列表失败: %w", err)
	}

	// 过滤出止盈单并取消
	canceledCount := 0
	for _, order := range orders {
		// OKX的止盈单类型: take_profit_market, take_profit
		if order.OrdType == "take_profit_market" || order.OrdType == "take_profit" {
			cancelBody := map[string]interface{}{
				"instId": instID,
				"ordId":  order.OrdID,
			}

			_, err := t.makeRequest("POST", "/api/v5/trade/cancel-order", cancelBody)
			if err != nil {
				log.Printf("  ⚠ 取消止盈单失败: %v", err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ 已取消止盈单 (订单ID: %s, 类型: %s, 方向: %s)", order.OrdID, order.OrdType, order.PosSide)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止盈单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止盈单", symbol, canceledCount)
	}

	return nil
}

// CancelAllOrders 取消该币种的所有挂单
func (t *OKXTrader) CancelAllOrders(symbol string) error {
	instID := t.convertSymbolToInstID(symbol)
	
	cancelBody := map[string]interface{}{
		"instId": instID,
	}

	_, err := t.makeRequest("POST", "/api/v5/trade/cancel-all-after", cancelBody)
	if err != nil {
		// 如果失败，尝试逐个取消
		return t.cancelAllOrdersOneByOne(instID)
	}

	log.Printf("  ✓ 已取消 %s 的所有挂单", symbol)
	return nil
}

// cancelAllOrdersOneByOne 逐个取消订单（备用方法）
func (t *OKXTrader) cancelAllOrdersOneByOne(instID string) error {
	data, err := t.makeRequest("GET", fmt.Sprintf("/api/v5/trade/orders-pending?instId=%s", instID), nil)
	if err != nil {
		return fmt.Errorf("获取未完成订单失败: %w", err)
	}

	var orders []struct {
		OrdID string `json:"ordId"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return fmt.Errorf("解析订单列表失败: %w", err)
	}

	for _, order := range orders {
		cancelBody := map[string]interface{}{
			"instId": instID,
			"ordId":  order.OrdID,
		}

		_, err := t.makeRequest("POST", "/api/v5/trade/cancel-order", cancelBody)
		if err != nil {
			log.Printf("  ⚠ 取消订单 %s 失败: %v", order.OrdID, err)
		}
	}

	return nil
}

// CancelStopOrders 取消该币种的止盈/止损单
func (t *OKXTrader) CancelStopOrders(symbol string) error {
	instID := t.convertSymbolToInstID(symbol)
	
	// 获取该币种的所有未完成订单
	data, err := t.makeRequest("GET", fmt.Sprintf("/api/v5/trade/orders-pending?instId=%s", instID), nil)
	if err != nil {
		return fmt.Errorf("获取未完成订单失败: %w", err)
	}

	var orders []struct {
		OrdID   string `json:"ordId"`
		OrdType string `json:"ordType"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return fmt.Errorf("解析订单列表失败: %w", err)
	}

	// 过滤出止盈止损单并取消
	canceledCount := 0
	for _, order := range orders {
		// OKX的止盈止损单类型
		if order.OrdType == "stop_market" || order.OrdType == "take_profit_market" ||
			order.OrdType == "stop" || order.OrdType == "take_profit" {
			
			cancelBody := map[string]interface{}{
				"instId": instID,
				"ordId":  order.OrdID,
			}

			_, err := t.makeRequest("POST", "/api/v5/trade/cancel-order", cancelBody)
			if err != nil {
				log.Printf("  ⚠ 取消订单 %s 失败: %v", order.OrdID, err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ 已取消 %s 的止盈/止损单 (订单ID: %s, 类型: %s)", symbol, order.OrdID, order.OrdType)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止盈/止损单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止盈/止损单", symbol, canceledCount)
	}

	return nil
}

// GetMarketPrice 获取市场价格
func (t *OKXTrader) GetMarketPrice(symbol string) (float64, error) {
	instID := t.convertSymbolToInstID(symbol)
	
	data, err := t.makeRequest("GET", fmt.Sprintf("/api/v5/market/ticker?instId=%s", instID), nil)
	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	var tickers []struct {
		Last string `json:"last"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, fmt.Errorf("解析价格数据失败: %w", err)
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("未找到价格")
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// SetStopLoss 设置止损单
func (t *OKXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	instID := t.convertSymbolToInstID(symbol)
	
	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 确定方向
	side := "sell"
	if positionSide == "SHORT" {
		side = "buy"
	}

	reqBody := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "isolated",
		"side":    side,
		"ordType": "stop_market",
		"sz":      quantityStr,
		"slTriggerPx": fmt.Sprintf("%.8f", stopPrice),
		"slTriggerPxType": "last",
		"posSide": positionSide,
		"reduceOnly": true,
	}

	_, err = t.makeRequest("POST", "/api/v5/trade/order", reqBody)
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	log.Printf("  止损价设置: %.4f", stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *OKXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	instID := t.convertSymbolToInstID(symbol)
	
	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 确定方向
	side := "sell"
	if positionSide == "SHORT" {
		side = "buy"
	}

	reqBody := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "isolated",
		"side":    side,
		"ordType": "take_profit_market",
		"sz":      quantityStr,
		"tpTriggerPx": fmt.Sprintf("%.8f", takeProfitPrice),
		"tpTriggerPxType": "last",
		"posSide": positionSide,
		"reduceOnly": true,
	}

	_, err = t.makeRequest("POST", "/api/v5/trade/order", reqBody)
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	log.Printf("  止盈价设置: %.4f", takeProfitPrice)
	return nil
}

// GetMinNotional 获取最小名义价值（OKX要求）
func (t *OKXTrader) GetMinNotional(symbol string) float64 {
	// OKX合约最小名义价值通常是 5 USDT
	return 5.0
}

// CheckMinNotional 检查订单是否满足最小名义价值要求
// V1.50版本：小账户放宽限制，允许更小的订单
func (t *OKXTrader) CheckMinNotional(symbol string, quantity float64) error {
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return fmt.Errorf("获取市价失败: %w", err)
	}

	notionalValue := quantity * price

	// 获取账户余额以判断是否为小账户
	balance, err := t.GetBalance()
	if err == nil {
		totalEquity := 0.0
		if equity, ok := balance["totalEquity"].(float64); ok && equity > 0 {
			totalEquity = equity
		} else if wallet, ok := balance["totalWalletBalance"].(float64); ok {
			totalEquity = wallet
		}

		// V1.50版本：小账户（<10 USDT）放宽最小订单金额限制
		if totalEquity > 0 && totalEquity < 10.0 {
			// 极小账户：允许账户净值50%的订单（最小2 USDT）
			minNotionalForSmallAccount := totalEquity * 0.5
			if minNotionalForSmallAccount < 2.0 {
				minNotionalForSmallAccount = 2.0
			}

			// 如果是BTC/ETH，允许账户净值80%（最小5 USDT）
			if symbol == "BTCUSDT" || symbol == "ETHUSDT" {
				minNotionalForSmallAccount = totalEquity * 0.8
				if minNotionalForSmallAccount < 5.0 {
					minNotionalForSmallAccount = 5.0
				}
			}

			if notionalValue >= minNotionalForSmallAccount {
				log.Printf("  ✓ 小账户模式：订单金额 %.2f USDT 满足最小要求 %.2f USDT（账户净值 %.2f USDT）",
					notionalValue, minNotionalForSmallAccount, totalEquity)
				return nil
			}
		}
	}

	// 正常账户或小账户订单仍然太小：使用标准限制
	minNotional := t.GetMinNotional(symbol)

	if notionalValue < minNotional {
		return fmt.Errorf(
			"订单金额 %.2f USDT 低于最小要求 %.2f USDT (数量: %.4f, 价格: %.4f)",
			notionalValue, minNotional, quantity, price,
		)
	}

	return nil
}

// GetSymbolPrecision 获取交易对的数量精度
func (t *OKXTrader) GetSymbolPrecision(symbol string) (int, error) {
	// 先检查缓存
	t.precisionMutex.RLock()
	if precision, ok := t.symbolPrecision[symbol]; ok {
		t.precisionMutex.RUnlock()
		return precision, nil
	}
	t.precisionMutex.RUnlock()

	instID := t.convertSymbolToInstID(symbol)
	
	// 获取交易对信息
	data, err := t.makeRequest("GET", fmt.Sprintf("/api/v5/public/instruments?instType=SWAP&instId=%s", instID), nil)
	if err != nil {
		log.Printf("  ⚠ %s 获取精度信息失败，使用默认精度3: %v", symbol, err)
		return 3, nil
	}

	var instruments []struct {
		LotSz string `json:"lotSz"` // 数量精度
	}

	if err := json.Unmarshal(data, &instruments); err != nil {
		log.Printf("  ⚠ %s 解析精度信息失败，使用默认精度3: %v", symbol, err)
		return 3, nil
	}

	if len(instruments) == 0 {
		log.Printf("  ⚠ %s 未找到精度信息，使用默认精度3", symbol)
		return 3, nil
	}

	// 从lotSz计算精度（例如 "0.001" -> 3）
	lotSz := instruments[0].LotSz
	precision := calculatePrecisionFromStepSize(lotSz)

	// 更新缓存
	t.precisionMutex.Lock()
	t.symbolPrecision[symbol] = precision
	t.precisionMutex.Unlock()

	log.Printf("  %s 数量精度: %d (lotSz: %s)", symbol, precision, lotSz)
	return precision, nil
}

// GetSymbolLotSz 获取交易对的实际lotSz（最小数量单位）
// V1.66版本：新增函数，用于获取实际的lotSz值，而不是精度
// 带缓存机制，避免重复API调用
func (t *OKXTrader) GetSymbolLotSz(symbol string) (float64, error) {
	// 先检查缓存
	t.lotSzMutex.RLock()
	if lotSz, ok := t.symbolLotSz[symbol]; ok {
		t.lotSzMutex.RUnlock()
		return lotSz, nil
	}
	t.lotSzMutex.RUnlock()

	instID := t.convertSymbolToInstID(symbol)
	
	// 获取交易对信息
	data, err := t.makeRequest("GET", fmt.Sprintf("/api/v5/public/instruments?instType=SWAP&instId=%s", instID), nil)
	if err != nil {
		log.Printf("  ⚠ %s 获取lotSz失败，使用默认值0.0001: %v", symbol, err)
		// 缓存默认值，避免重复请求
		t.lotSzMutex.Lock()
		t.symbolLotSz[symbol] = 0.0001
		t.lotSzMutex.Unlock()
		return 0.0001, nil
	}

	var instruments []struct {
		LotSz string `json:"lotSz"` // 数量精度
	}

	if err := json.Unmarshal(data, &instruments); err != nil {
		log.Printf("  ⚠ %s 解析lotSz失败，使用默认值0.0001: %v", symbol, err)
		// 缓存默认值
		t.lotSzMutex.Lock()
		t.symbolLotSz[symbol] = 0.0001
		t.lotSzMutex.Unlock()
		return 0.0001, nil
	}

	if len(instruments) == 0 {
		log.Printf("  ⚠ %s 未找到lotSz信息，使用默认值0.0001", symbol)
		// 缓存默认值
		t.lotSzMutex.Lock()
		t.symbolLotSz[symbol] = 0.0001
		t.lotSzMutex.Unlock()
		return 0.0001, nil
	}

	// 解析lotSz字符串为浮点数
	lotSz, err := strconv.ParseFloat(instruments[0].LotSz, 64)
	if err != nil {
		log.Printf("  ⚠ %s 解析lotSz值失败 (%s)，使用默认值0.0001: %v", symbol, instruments[0].LotSz, err)
		// 缓存默认值
		t.lotSzMutex.Lock()
		t.symbolLotSz[symbol] = 0.0001
		t.lotSzMutex.Unlock()
		return 0.0001, nil
	}

	// 更新缓存
	t.lotSzMutex.Lock()
	t.symbolLotSz[symbol] = lotSz
	t.lotSzMutex.Unlock()

	log.Printf("  %s lotSz: %s (%.8f)", symbol, instruments[0].LotSz, lotSz)
	return lotSz, nil
}

// FormatQuantity 格式化数量到正确的精度
// V1.66版本：使用实际的lotSz进行向上取整，避免数量格式化后为0
// 每个币种使用其实际的lotSz（最小数量单位）进行向上取整
func (t *OKXTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	precision, err := t.GetSymbolPrecision(symbol)
	if err != nil {
		// 如果获取失败，使用默认格式（保留更多小数位，避免丢失精度）
		return fmt.Sprintf("%.8f", quantity), nil
	}

	// V1.66版本：获取实际的lotSz（最小数量单位），而不是使用固定的0.0001
	// 这样可以针对每个币种使用正确的精度
	lotSz, err := t.GetSymbolLotSz(symbol)
	if err != nil {
		// 如果获取失败，使用默认值0.0001
		lotSz = 0.0001
		log.Printf("  ⚠️ %s 获取lotSz失败，使用默认值0.0001", symbol)
	}

	// 使用实际的lotSz进行向上取整
	// 逻辑：
	// - 如果数量 > 0 且 < lotSz：向上取整到 lotSz
	// - 如果数量 >= lotSz：向上取整到 lotSz 的倍数
	// 示例（假设BTC的lotSz是0.01）：
	//   - 0.00441287 → ceil(0.00441287 / 0.01) * 0.01 = ceil(0.441287) * 0.01 = 1 * 0.01 = 0.01
	//   - 0.00005 → 向上取整到 0.01（如果lotSz是0.01）
	
	if quantity > 0 {
		if quantity < lotSz {
			// 数量小于lotSz，向上取整到lotSz
			log.Printf("  ⚠️ %s 数量 %.8f 小于 lotSz %.8f，向上取整到 %.8f", symbol, quantity, lotSz, lotSz)
			quantity = lotSz
		} else {
			// 数量大于等于lotSz，向上取整到lotSz的倍数
			rounded := math.Ceil(quantity / lotSz) * lotSz
			if rounded != quantity {
				log.Printf("  ⚠️ %s 数量 %.8f 向上取整到 lotSz %.8f 的倍数: %.8f", symbol, quantity, lotSz, rounded)
			}
			quantity = rounded
		}
	}

	// 使用精度格式化
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity), nil
}

// convertSymbolToInstID 转换交易对格式 (BTCUSDT -> BTC-USDT-SWAP)
func (t *OKXTrader) convertSymbolToInstID(symbol string) string {
	// 移除USDT后缀
	base := strings.TrimSuffix(symbol, "USDT")
	// 添加OKX格式: BASE-USDT-SWAP
	return base + "-USDT-SWAP"
}

// calculatePrecisionFromStepSize 从stepSize计算精度
func calculatePrecisionFromStepSize(stepSize string) int {
	// 去除尾部的0
	stepSize = strings.TrimRight(stepSize, "0")
	stepSize = strings.TrimRight(stepSize, ".")

	// 查找小数点
	dotIndex := strings.Index(stepSize, ".")
	if dotIndex == -1 {
		return 0
	}

	// 返回小数点后的位数
	return len(stepSize) - dotIndex - 1
}

