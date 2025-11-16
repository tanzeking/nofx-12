package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// K线数据获取配置常量
const (
	// DefaultKlineLimit 默认K线数据条数（支持长周期技术指标计算）
	DefaultKlineLimit = 300 // 从100增加到300，支持更准确的技术指标计算
	// MaxKlineLimit OKX API最大支持K线数量
	MaxKlineLimit = 300
)

// Get 获取指定代币的市场数据（使用Binance）
func Get(symbol string) (*Data, error) {
	return GetWithExchange(symbol, "binance")
}

// GetWithExchange 获取指定代币的市场数据（支持多交易所）
func GetWithExchange(symbol string, exchange string) (*Data, error) {
	var klines3m, klines4h []Kline
	var err error
	// 标准化symbol
	symbol = Normalize(symbol)
	
	// 根据交易所选择K线数据源
	if exchange == "okx" {
		// OKX直接使用API客户端
		okxClient := NewOKXAPIClient()
		klines3m, err = okxClient.GetKlines(symbol, "3m", DefaultKlineLimit)
		if err != nil {
			return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
		}
		klines4h, err = okxClient.GetKlines(symbol, "4h", DefaultKlineLimit)
		if err != nil {
			return nil, fmt.Errorf("获取4小时K线失败: %v", err)
		}
	} else {
		// Binance优先使用WebSocket监控器（如果可用），否则使用API
		if WSMonitorCli != nil {
			klines3m, err = WSMonitorCli.GetCurrentKlines(symbol, "3m")
			if err == nil {
				klines4h, err = WSMonitorCli.GetCurrentKlines(symbol, "4h")
				if err == nil {
					goto gotKlines
				}
			}
		}
		// WebSocket失败，使用API客户端
		binanceClient := NewAPIClient()
		klines3m, err = binanceClient.GetKlines(symbol, "3m", DefaultKlineLimit)
		if err != nil {
			return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
		}
		klines4h, err = binanceClient.GetKlines(symbol, "4h", DefaultKlineLimit)
		if err != nil {
			return nil, fmt.Errorf("获取4小时K线失败: %v", err)
		}
	}
gotKlines:

	// 根据交易所选择API客户端
	var apiClient interface {
		GetCurrentPrice(string) (float64, error)
	}
	var oiClient interface {
		GetOpenInterest(string) (*OIData, error)
	}
	var fundingClient interface {
		GetFundingRate(string) (float64, error)
	}
	
	if exchange == "okx" {
		okxClient := NewOKXAPIClient()
		apiClient = okxClient
		oiClient = okxClient
		fundingClient = okxClient
	} else {
		binanceClient := NewAPIClient()
		apiClient = binanceClient
		oiClient = nil // Binance使用单独的getOpenInterestData函数
		fundingClient = nil // Binance使用单独的getFundingRate函数
	}
	
	// 优先获取实时价格（从ticker API），确保AI读取到最新报价
	realTimePrice, err := apiClient.GetCurrentPrice(symbol)
	if err != nil {
		// 如果获取实时价格失败，使用K线价格作为后备
		log.Printf("⚠️  获取 %s 实时价格失败，使用K线价格: %v", symbol, err)
		realTimePrice = klines3m[len(klines3m)-1].Close
	} else {
		log.Printf("✓ 获取 %s 实时价格: %.4f (K线价格: %.4f)", symbol, realTimePrice, klines3m[len(klines3m)-1].Close)
	}
	
	// 使用实时价格
	currentPrice := realTimePrice
	currentEMA20 := calculateEMA(klines3m, 20)
	currentMACD := calculateMACD(klines3m)
	currentRSI7 := calculateRSI(klines3m, 7)

	// 计算价格变化百分比
	// 1小时价格变化 = 20个3分钟K线前的价格
	priceChange1h := 0.0
	if len(klines3m) >= 21 { // 至少需要21根K线 (当前 + 20根前)
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 4小时价格变化 = 1个4小时K线前的价格
	priceChange4h := 0.0
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 获取OI数据
	var oiData *OIData
	if oiClient != nil {
		oiData, err = oiClient.GetOpenInterest(symbol)
		if err != nil {
			log.Printf("⚠️  获取 %s OI数据失败: %v", symbol, err)
			oiData = &OIData{Latest: 0, Average: 0}
		}
	} else {
		oiData, err = getOpenInterestData(symbol)
		if err != nil {
			oiData = &OIData{Latest: 0, Average: 0}
		}
	}

	// 获取Funding Rate
	var fundingRate float64
	if fundingClient != nil {
		fundingRate, _ = fundingClient.GetFundingRate(symbol)
	} else {
		fundingRate, _ = getFundingRate(symbol)
	}

	// 计算日内系列数据
	intradayData := calculateIntradaySeries(klines3m)

	// 计算长期数据
	longerTermData := calculateLongerTermData(klines4h)

	// V1.65新增：计算当前值的技术指标
	var bollingerBands *BollingerBandsData
	var kdj *KDJData
	var sma *SMAData
	var volumeMA *VolumeMAData
	var obv float64

	if len(klines3m) >= 20 {
		bollingerBands = calculateBollingerBands(klines3m, 20, 2.0)
	}
	if len(klines3m) >= 9 {
		kdj = calculateKDJ(klines3m, 9)
	}
	if len(klines3m) >= 5 {
		sma = &SMAData{}
		if len(klines3m) >= 5 {
			sma.SMA5 = calculateSMA(klines3m, 5)
		}
		if len(klines3m) >= 10 {
			sma.SMA10 = calculateSMA(klines3m, 10)
		}
		if len(klines3m) >= 20 {
			sma.SMA20 = calculateSMA(klines3m, 20)
		}
		if len(klines3m) >= 50 {
			sma.SMA50 = calculateSMA(klines3m, 50)
		}
		if len(klines3m) >= 100 {
			sma.SMA100 = calculateSMA(klines3m, 100)
		}
	}
	if len(klines3m) >= 2 {
		obv = calculateOBV(klines3m)
	}
	if len(klines3m) >= 5 {
		volumeMA = &VolumeMAData{}
		if len(klines3m) >= 5 {
			volumeMA.MA5 = calculateVolumeMA(klines3m, 5)
		}
		if len(klines3m) >= 20 {
			volumeMA.MA20 = calculateVolumeMA(klines3m, 20)
		}
		if len(klines3m) >= 50 {
			volumeMA.MA50 = calculateVolumeMA(klines3m, 50)
		}
	}

	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentRSI7:       currentRSI7,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		IntradaySeries:    intradayData,
		LongerTermContext: longerTermData,
		// V1.65新增：更多技术指标
		BollingerBands: bollingerBands,
		KDJ:            kdj,
		SMA:            sma,
		OBV:            obv,
		VolumeMA:       volumeMA,
	}, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	// 计算12期和26期EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12 - ema26
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateIntradaySeries 计算日内系列数据（V1.65: 扩展以支持更多指标）
func calculateIntradaySeries(klines []Kline) *IntradayData {
	data := &IntradayData{
		MidPrices:      make([]float64, 0, 20),
		EMA20Values:    make([]float64, 0, 20),
		MACDValues:     make([]float64, 0, 20),
		RSI7Values:     make([]float64, 0, 20),
		RSI14Values:    make([]float64, 0, 20),
		BollingerUpper: make([]float64, 0, 20),
		BollingerLower: make([]float64, 0, 20),
		BollingerMid:   make([]float64, 0, 20),
		KDJ_K:          make([]float64, 0, 20),
		KDJ_D:          make([]float64, 0, 20),
		KDJ_J:          make([]float64, 0, 20),
		SMA5:           make([]float64, 0, 20),
		SMA10:          make([]float64, 0, 20),
		SMA20:          make([]float64, 0, 20),
		SMA50:          make([]float64, 0, 20),
		OBVValues:      make([]float64, 0, 20),
		VolumeMA5:      make([]float64, 0, 20),
		VolumeMA20:     make([]float64, 0, 20),
	}

	// V1.65: 获取最近20个数据点（增加以支持更多指标）
	dataPoints := 20
	start := len(klines) - dataPoints
	if start < 0 {
		start = 0
	}
	if start > len(klines) {
		return data
	}

	// 计算OBV序列
	obvValues := calculateOBVValues(klines)
	if len(obvValues) > 0 {
		obvStart := len(obvValues) - dataPoints
		if obvStart < 0 {
			obvStart = 0
		}
		data.OBVValues = obvValues[obvStart:]
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的MACD
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// 计算每个点的RSI
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}

		// V1.65新增：计算布林带
		if i >= 19 {
			bb := calculateBollingerBands(klines[:i+1], 20, 2.0)
			if bb != nil {
				data.BollingerUpper = append(data.BollingerUpper, bb.Upper)
				data.BollingerLower = append(data.BollingerLower, bb.Lower)
				data.BollingerMid = append(data.BollingerMid, bb.Middle)
			}
		}

		// V1.65新增：计算SMA
		if i >= 4 {
			sma5 := calculateSMA(klines[:i+1], 5)
			data.SMA5 = append(data.SMA5, sma5)
		}
		if i >= 9 {
			sma10 := calculateSMA(klines[:i+1], 10)
			data.SMA10 = append(data.SMA10, sma10)
		}
		if i >= 19 {
			sma20 := calculateSMA(klines[:i+1], 20)
			data.SMA20 = append(data.SMA20, sma20)
		}
		if i >= 49 {
			sma50 := calculateSMA(klines[:i+1], 50)
			data.SMA50 = append(data.SMA50, sma50)
		}

		// V1.65新增：计算成交量MA
		if i >= 4 {
			volMA5 := calculateVolumeMA(klines[:i+1], 5)
			data.VolumeMA5 = append(data.VolumeMA5, volMA5)
		}
		if i >= 19 {
			volMA20 := calculateVolumeMA(klines[:i+1], 20)
			data.VolumeMA20 = append(data.VolumeMA20, volMA20)
		}
	}

	// V1.65新增：计算KDJ序列（使用最近的数据）
	if len(klines) >= 9 {
		kValues, dValues, jValues := calculateKDJValues(klines, 9)
		if len(kValues) > 0 {
			kStart := len(kValues) - dataPoints
			if kStart < 0 {
				kStart = 0
			}
			if kStart < len(kValues) {
				data.KDJ_K = kValues[kStart:]
				data.KDJ_D = dValues[kStart:]
				data.KDJ_J = jValues[kStart:]
			}
		}
	}

	return data
}

// calculateLongerTermData 计算长期数据（V1.65: 扩展以支持更多指标）
func calculateLongerTermData(klines []Kline) *LongerTermData {
	data := &LongerTermData{
		MACDValues: make([]float64, 0, 20),
		RSI14Values: make([]float64, 0, 20),
	}

	// 计算EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// 计算ATR
	data.ATR3 = calculateATR(klines, 3)
	data.ATR14 = calculateATR(klines, 14)

	// 计算成交量
	if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		// 计算平均成交量
		sum := 0.0
		for _, k := range klines {
			sum += k.Volume
		}
		data.AverageVolume = sum / float64(len(klines))
	}

	// V1.65新增：计算布林带
	if len(klines) >= 20 {
		data.BollingerBands = calculateBollingerBands(klines, 20, 2.0)
	}

	// V1.65新增：计算KDJ
	if len(klines) >= 9 {
		data.KDJ = calculateKDJ(klines, 9)
	}

	// V1.65新增：计算SMA（多周期）
	data.SMA = &SMAData{}
	if len(klines) >= 5 {
		data.SMA.SMA5 = calculateSMA(klines, 5)
	}
	if len(klines) >= 10 {
		data.SMA.SMA10 = calculateSMA(klines, 10)
	}
	if len(klines) >= 20 {
		data.SMA.SMA20 = calculateSMA(klines, 20)
	}
	if len(klines) >= 50 {
		data.SMA.SMA50 = calculateSMA(klines, 50)
	}
	if len(klines) >= 100 {
		data.SMA.SMA100 = calculateSMA(klines, 100)
	}

	// V1.65新增：计算OBV
	data.OBV = calculateOBV(klines)

	// V1.65新增：计算成交量MA
	data.VolumeMA = &VolumeMAData{}
	if len(klines) >= 5 {
		data.VolumeMA.MA5 = calculateVolumeMA(klines, 5)
	}
	if len(klines) >= 20 {
		data.VolumeMA.MA20 = calculateVolumeMA(klines, 20)
	}
	if len(klines) >= 50 {
		data.VolumeMA.MA50 = calculateVolumeMA(klines, 50)
	}

	// 计算MACD和RSI序列（V1.65: 增加到20个数据点）
	dataPoints := 20
	start := len(klines) - dataPoints
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	return data
}

// getOpenInterestData 获取OI数据
func getOpenInterestData(symbol string) (*OIData, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // 近似平均值
	}, nil
}

// getFundingRate 获取资金费率
func getFundingRate(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)
	return rate, nil
}

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("current_price = %.2f, current_ema20 = %.3f, current_macd = %.3f, current_rsi (7 period) = %.3f\n\n",
		data.CurrentPrice, data.CurrentEMA20, data.CurrentMACD, data.CurrentRSI7))

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n",
			data.OpenInterest.Latest, data.OpenInterest.Average))
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))

	if data.IntradaySeries != nil {
		sb.WriteString("Intraday series (3‑minute intervals, oldest → latest):\n\n")

		if len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
		}

		if len(data.IntradaySeries.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
		}

		if len(data.IntradaySeries.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
		}

		if len(data.IntradaySeries.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
		}

		if len(data.IntradaySeries.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
		}
	}

	if data.LongerTermContext != nil {
		sb.WriteString("Longer‑term context (4‑hour timeframe):\n\n")

		sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
			data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))

		sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
			data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))

		sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
			data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))

		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
		}

		if len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
		}
	}

	return sb.String()
}

// formatFloatSlice 格式化float64切片为字符串
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.3f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// ========== V1.65新增：更多技术指标计算函数 ==========

// calculateSMA 计算简单移动平均线（SMA）
func calculateSMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}
	
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	return sum / float64(period)
}

// calculateSMAValues 计算SMA序列
func calculateSMAValues(klines []Kline, period int) []float64 {
	var values []float64
	for i := period; i <= len(klines); i++ {
		sma := calculateSMA(klines[:i], period)
		values = append(values, sma)
	}
	return values
}

// calculateBollingerBands 计算布林带（Bollinger Bands）
// period: 周期（通常为20）
// stdDev: 标准差倍数（通常为2）
func calculateBollingerBands(klines []Kline, period int, stdDev float64) *BollingerBandsData {
	if len(klines) < period {
		return nil
	}
	
	// 计算中轨（SMA20）
	middle := calculateSMA(klines, period)
	
	// 计算标准差
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		diff := klines[i].Close - middle
		sum += diff * diff
	}
	variance := sum / float64(period)
	std := math.Sqrt(variance)
	
	// 计算上轨和下轨
	upper := middle + (stdDev * std)
	lower := middle - (stdDev * std)
	
	return &BollingerBandsData{
		Upper:  upper,
		Middle: middle,
		Lower:  lower,
	}
}

// calculateBollingerBandsValues 计算布林带序列
func calculateBollingerBandsValues(klines []Kline, period int, stdDev float64) ([]float64, []float64, []float64) {
	var upper, middle, lower []float64
	for i := period; i <= len(klines); i++ {
		bb := calculateBollingerBands(klines[:i], period, stdDev)
		if bb != nil {
			upper = append(upper, bb.Upper)
			middle = append(middle, bb.Middle)
			lower = append(lower, bb.Lower)
		}
	}
	return upper, middle, lower
}

// calculateKDJ 计算KDJ指标（优化版本，避免递归）
// period: RSV周期（通常为9）
func calculateKDJ(klines []Kline, period int) *KDJData {
	if len(klines) < period {
		return nil
	}
	
	// 初始化K和D值
	k := 50.0
	d := 50.0
	
	// 从period开始计算KDJ序列
	for i := period; i <= len(klines); i++ {
		// 计算最近period根K线的最高价和最低价
		high := klines[i-period].High
		low := klines[i-period].Low
		for j := i - period + 1; j < i; j++ {
			if klines[j].High > high {
				high = klines[j].High
			}
			if klines[j].Low < low {
				low = klines[j].Low
			}
		}
		
		// 计算RSV
		close := klines[i-1].Close
		rsv := 0.0
		if high != low {
			rsv = ((close - low) / (high - low)) * 100
		}
		
		// 计算K值：K = (2/3) * 前K值 + (1/3) * RSV
		k = (2.0/3.0)*k + (1.0/3.0)*rsv
		
		// 计算D值：D = (2/3) * 前D值 + (1/3) * K
		d = (2.0/3.0)*d + (1.0/3.0)*k
	}
	
	// 计算J值：J = 3*K - 2*D
	j := 3*k - 2*d
	
	return &KDJData{
		K: k,
		D: d,
		J: j,
	}
}

// calculateKDJValues 计算KDJ序列（优化版本）
func calculateKDJValues(klines []Kline, period int) ([]float64, []float64, []float64) {
	if len(klines) < period {
		return nil, nil, nil
	}
	
	var kValues, dValues, jValues []float64
	k := 50.0
	d := 50.0
	
	// 从period开始计算KDJ序列
	for i := period; i <= len(klines); i++ {
		// 计算最近period根K线的最高价和最低价
		high := klines[i-period].High
		low := klines[i-period].Low
		for j := i - period + 1; j < i; j++ {
			if klines[j].High > high {
				high = klines[j].High
			}
			if klines[j].Low < low {
				low = klines[j].Low
			}
		}
		
		// 计算RSV
		close := klines[i-1].Close
		rsv := 0.0
		if high != low {
			rsv = ((close - low) / (high - low)) * 100
		}
		
		// 计算K值
		k = (2.0/3.0)*k + (1.0/3.0)*rsv
		
		// 计算D值
		d = (2.0/3.0)*d + (1.0/3.0)*k
		
		// 计算J值
		j := 3*k - 2*d
		
		kValues = append(kValues, k)
		dValues = append(dValues, d)
		jValues = append(jValues, j)
	}
	
	return kValues, dValues, jValues
}

// calculateOBV 计算能量潮指标（On-Balance Volume）
func calculateOBV(klines []Kline) float64 {
	if len(klines) < 2 {
		return 0
	}
	
	obv := 0.0
	for i := 1; i < len(klines); i++ {
		if klines[i].Close > klines[i-1].Close {
			obv += klines[i].Volume
		} else if klines[i].Close < klines[i-1].Close {
			obv -= klines[i].Volume
		}
		// 如果价格不变，OBV不变
	}
	return obv
}

// calculateOBVValues 计算OBV序列
func calculateOBVValues(klines []Kline) []float64 {
	var values []float64
	if len(klines) < 2 {
		return values
	}
	
	obv := 0.0
	values = append(values, 0.0) // 第一个值设为0
	
	for i := 1; i < len(klines); i++ {
		if klines[i].Close > klines[i-1].Close {
			obv += klines[i].Volume
		} else if klines[i].Close < klines[i-1].Close {
			obv -= klines[i].Volume
		}
		values = append(values, obv)
	}
	return values
}

// calculateVolumeMA 计算成交量移动平均
func calculateVolumeMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}
	
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Volume
	}
	return sum / float64(period)
}

// calculateVolumeMAValues 计算成交量移动平均序列
func calculateVolumeMAValues(klines []Kline, period int) []float64 {
	var values []float64
	for i := period; i <= len(klines); i++ {
		ma := calculateVolumeMA(klines[:i], period)
		values = append(values, ma)
	}
	return values
}
