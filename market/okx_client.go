package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	okxBaseURL = "https://www.okx.com"
)

type OKXAPIClient struct {
	client *http.Client
}

func NewOKXAPIClient() *OKXAPIClient {
	return &OKXAPIClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetKlines 获取OKX K线数据
func (c *OKXAPIClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	// 记录响应时间
	startTime := time.Now()
	
	// 转换symbol格式：BTCUSDT -> BTC-USDT-SWAP
	instID := convertSymbolToOKXInstID(symbol)
	
	// 转换时间间隔：3m -> 3m, 4h -> 4H
	okxInterval := interval
	if interval == "4h" {
		okxInterval = "4H"
	}
	
	url := fmt.Sprintf("%s/api/v5/market/candles", okxBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("instId", instID)
	q.Add("bar", okxInterval)
	q.Add("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	networkTime := time.Since(startTime)
	
	if err != nil {
		log.Printf("❌ OKX API [K线] %s %s: 请求失败，耗时 %v: %v", symbol, interval, networkTime, err)
		return nil, err
	}
	defer resp.Body.Close()
	
	// 记录网络响应时间
	log.Printf("⏱️  OKX API [K线] %s %s: 网络响应时间 %v (状态码: %d)", symbol, interval, networkTime, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var okxResponse struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data [][]interface{} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &okxResponse); err != nil {
		return nil, fmt.Errorf("解析OKX响应失败: %w, 原始响应: %s", err, string(body))
	}

	if okxResponse.Code != "0" {
		return nil, fmt.Errorf("OKX API错误: code=%s, msg=%s", okxResponse.Code, okxResponse.Msg)
	}

	var klines []Kline
	// OKX返回的数据是倒序的（最新的在前），需要反转
	for i := len(okxResponse.Data) - 1; i >= 0; i-- {
		kr := okxResponse.Data[i]
		if len(kr) < 6 {
			continue
		}

		kline := Kline{}
		// OKX格式: [ts, o, h, l, c, vol, volCcy, volCcyQuote, confirm]
		// ts: 开始时间（毫秒）
		// o: 开盘价
		// h: 最高价
		// l: 最低价
		// c: 收盘价
		// vol: 成交量（张）
		// volCcy: 成交量（币）
		// volCcyQuote: 成交量（计价币）
		
		ts, _ := strconv.ParseInt(kr[0].(string), 10, 64)
		kline.OpenTime = ts
		kline.Close, _ = strconv.ParseFloat(kr[4].(string), 64)
		kline.Open, _ = strconv.ParseFloat(kr[1].(string), 64)
		kline.High, _ = strconv.ParseFloat(kr[2].(string), 64)
		kline.Low, _ = strconv.ParseFloat(kr[3].(string), 64)
		if len(kr) > 5 {
			kline.Volume, _ = strconv.ParseFloat(kr[5].(string), 64)
		}
		// 计算CloseTime（根据interval）
		closeTimeOffset := int64(0)
		switch interval {
		case "3m":
			closeTimeOffset = 3 * 60 * 1000 // 3分钟
		case "4h":
			closeTimeOffset = 4 * 60 * 60 * 1000 // 4小时
		}
		kline.CloseTime = ts + closeTimeOffset - 1
		
		klines = append(klines, kline)
	}
	
	// 记录总耗时（包括解析时间）
	totalTime := time.Since(startTime)
	log.Printf("✓ OKX API [K线] %s %s: 获取 %d 根K线，总耗时 %v (网络: %v, 解析: %v)", 
		symbol, interval, len(klines), totalTime, networkTime, totalTime-networkTime)

	return klines, nil
}

// GetCurrentPrice 获取OKX实时价格
func (c *OKXAPIClient) GetCurrentPrice(symbol string) (float64, error) {
	// 记录响应时间
	startTime := time.Now()
	
	instID := convertSymbolToOKXInstID(symbol)
	
	url := fmt.Sprintf("%s/api/v5/market/ticker", okxBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	q := req.URL.Query()
	q.Add("instId", instID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	responseTime := time.Since(startTime)
	
	if err != nil {
		log.Printf("❌ OKX API [价格] %s: 请求失败，耗时 %v: %v", symbol, responseTime, err)
		return 0, err
	}
	defer resp.Body.Close()
	
	// 记录响应时间（仅记录成功请求）
	log.Printf("⏱️  OKX API [价格] %s: 响应时间 %v (状态码: %d)", symbol, responseTime, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var okxResponse struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Last string `json:"last"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &okxResponse); err != nil {
		return 0, fmt.Errorf("解析OKX价格响应失败: %w, 原始响应: %s", err, string(body))
	}

	if okxResponse.Code != "0" || len(okxResponse.Data) == 0 {
		return 0, fmt.Errorf("OKX API错误: code=%s, msg=%s", okxResponse.Code, okxResponse.Msg)
	}

	price, err := strconv.ParseFloat(okxResponse.Data[0].Last, 64)
	if err != nil {
		log.Printf("❌ OKX API [价格] %s: 解析价格失败: %v", symbol, err)
		return 0, err
	}
	
	// 记录总耗时（包括解析时间）
	totalTime := time.Since(startTime)
	log.Printf("✓ OKX API [价格] %s: 价格 %.4f，总耗时 %v (网络: %v, 解析: %v)", 
		symbol, price, totalTime, responseTime, totalTime-responseTime)

	return price, nil
}

// GetOpenInterest 获取OKX持仓量
func (c *OKXAPIClient) GetOpenInterest(symbol string) (*OIData, error) {
	// 记录响应时间
	startTime := time.Now()
	
	instID := convertSymbolToOKXInstID(symbol)
	
	url := fmt.Sprintf("%s/api/v5/public/open-interest", okxBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("instId", instID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	responseTime := time.Since(startTime)
	
	if err != nil {
		log.Printf("❌ OKX API [持仓量] %s: 请求失败，耗时 %v: %v", symbol, responseTime, err)
		return nil, err
	}
	defer resp.Body.Close()
	
	// 记录响应时间（仅记录成功请求）
	log.Printf("⏱️  OKX API [持仓量] %s: 响应时间 %v (状态码: %d)", symbol, responseTime, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var okxResponse struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			InstID       string `json:"instId"`
			Oi           string `json:"oi"`
			OiCcy        string `json:"oiCcy"`
			Ts           string `json:"ts"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &okxResponse); err != nil {
		return nil, fmt.Errorf("解析OKX持仓量响应失败: %w", err)
	}

	if okxResponse.Code != "0" || len(okxResponse.Data) == 0 {
		return nil, fmt.Errorf("OKX API错误: code=%s, msg=%s", okxResponse.Code, okxResponse.Msg)
	}

	oi, _ := strconv.ParseFloat(okxResponse.Data[0].Oi, 64)
	
	// 记录总耗时
	totalTime := time.Since(startTime)
	log.Printf("✓ OKX API [持仓量] %s: OI %.0f，总耗时 %v", symbol, oi, totalTime)
	
	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // 近似平均值
	}, nil
}

// GetFundingRate 获取OKX资金费率
func (c *OKXAPIClient) GetFundingRate(symbol string) (float64, error) {
	// 记录响应时间
	startTime := time.Now()
	
	instID := convertSymbolToOKXInstID(symbol)
	
	url := fmt.Sprintf("%s/api/v5/public/funding-rate", okxBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	q := req.URL.Query()
	q.Add("instId", instID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	responseTime := time.Since(startTime)
	
	if err != nil {
		log.Printf("❌ OKX API [资金费率] %s: 请求失败，耗时 %v: %v", symbol, responseTime, err)
		return 0, err
	}
	defer resp.Body.Close()
	
	// 记录响应时间（仅记录成功请求）
	log.Printf("⏱️  OKX API [资金费率] %s: 响应时间 %v (状态码: %d)", symbol, responseTime, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var okxResponse struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			InstID      string `json:"instId"`
			FundingRate string `json:"fundingRate"`
			NextFundingTime string `json:"nextFundingTime"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &okxResponse); err != nil {
		return 0, fmt.Errorf("解析OKX资金费率响应失败: %w", err)
	}

	if okxResponse.Code != "0" || len(okxResponse.Data) == 0 {
		return 0, fmt.Errorf("OKX API错误: code=%s, msg=%s", okxResponse.Code, okxResponse.Msg)
	}

	rate, _ := strconv.ParseFloat(okxResponse.Data[0].FundingRate, 64)
	
	// 记录总耗时
	totalTime := time.Since(startTime)
	log.Printf("✓ OKX API [资金费率] %s: 费率 %.6f，总耗时 %v", symbol, rate, totalTime)
	
	return rate, nil
}

// convertSymbolToOKXInstID 转换symbol格式：BTCUSDT -> BTC-USDT-SWAP
// 注意：这个函数与trader/okx_trader.go中的convertSymbolToInstID功能相同，但保持独立以避免循环依赖
func convertSymbolToOKXInstID(symbol string) string {
	symbol = strings.ToUpper(symbol)
	// 移除USDT后缀
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return fmt.Sprintf("%s-USDT-SWAP", base)
	}
	return symbol
}

