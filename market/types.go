package market

import "time"

// Data 市场数据结构（V1.65: 扩展以支持更多技术指标）
type Data struct {
	Symbol            string
	CurrentPrice      float64
	PriceChange1h     float64 // 1小时价格变化百分比
	PriceChange4h     float64 // 4小时价格变化百分比
	CurrentEMA20      float64
	CurrentMACD       float64
	CurrentRSI7       float64
	OpenInterest      *OIData
	FundingRate       float64
	IntradaySeries    *IntradayData
	LongerTermContext *LongerTermData
	// V1.65新增：更多技术指标
	BollingerBands    *BollingerBandsData // 布林带
	KDJ               *KDJData            // KDJ指标
	SMA               *SMAData            // 简单移动平均线（多周期）
	OBV               float64             // 能量潮指标
	VolumeMA          *VolumeMAData       // 成交量移动平均
}

// OIData Open Interest数据
type OIData struct {
	Latest  float64
	Average float64
}

// IntradayData 日内数据(3分钟间隔)（V1.65: 扩展以支持更多指标）
type IntradayData struct {
	MidPrices   []float64
	EMA20Values []float64
	MACDValues  []float64
	RSI7Values  []float64
	RSI14Values []float64
	// V1.65新增：更多指标序列
	BollingerUpper []float64 // 布林带上轨
	BollingerLower []float64 // 布林带下轨
	BollingerMid   []float64 // 布林带中轨
	KDJ_K          []float64 // KDJ K值
	KDJ_D          []float64 // KDJ D值
	KDJ_J          []float64 // KDJ J值
	SMA5           []float64 // SMA5序列
	SMA10          []float64 // SMA10序列
	SMA20          []float64 // SMA20序列
	SMA50          []float64 // SMA50序列
	OBVValues      []float64 // OBV序列
	VolumeMA5      []float64 // 成交量MA5序列
	VolumeMA20     []float64 // 成交量MA20序列
}

// LongerTermData 长期数据(4小时时间框架)（V1.65: 扩展以支持更多指标）
type LongerTermData struct {
	EMA20         float64
	EMA50         float64
	ATR3          float64
	ATR14         float64
	CurrentVolume float64
	AverageVolume float64
	MACDValues    []float64
	RSI14Values   []float64
	// V1.65新增：更多长期指标
	BollingerBands *BollingerBandsData // 布林带
	KDJ            *KDJData            // KDJ指标
	SMA            *SMAData            // 简单移动平均线
	OBV            float64             // 能量潮指标
	VolumeMA       *VolumeMAData       // 成交量移动平均
}

// BollingerBandsData 布林带数据
type BollingerBandsData struct {
	Upper float64 // 上轨
	Middle float64 // 中轨（SMA20）
	Lower float64 // 下轨
}

// KDJData KDJ指标数据
type KDJData struct {
	K float64 // K值
	D float64 // D值
	J float64 // J值
}

// SMAData 简单移动平均线数据（多周期）
type SMAData struct {
	SMA5  float64 // 5周期SMA
	SMA10 float64 // 10周期SMA
	SMA20 float64 // 20周期SMA
	SMA50 float64 // 50周期SMA
	SMA100 float64 // 100周期SMA（如果数据足够）
}

// VolumeMAData 成交量移动平均数据
type VolumeMAData struct {
	MA5  float64 // 5周期成交量MA
	MA20 float64 // 20周期成交量MA
	MA50 float64 // 50周期成交量MA（如果数据足够）
}

// Binance API 响应结构
type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

type SymbolInfo struct {
	Symbol            string `json:"symbol"`
	Status            string `json:"status"`
	BaseAsset         string `json:"baseAsset"`
	QuoteAsset        string `json:"quoteAsset"`
	ContractType      string `json:"contractType"`
	PricePrecision    int    `json:"pricePrecision"`
	QuantityPrecision int    `json:"quantityPrecision"`
}

type Kline struct {
	OpenTime            int64   `json:"openTime"`
	Open                float64 `json:"open"`
	High                float64 `json:"high"`
	Low                 float64 `json:"low"`
	Close               float64 `json:"close"`
	Volume              float64 `json:"volume"`
	CloseTime           int64   `json:"closeTime"`
	QuoteVolume         float64 `json:"quoteVolume"`
	Trades              int     `json:"trades"`
	TakerBuyBaseVolume  float64 `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume float64 `json:"takerBuyQuoteVolume"`
}

type KlineResponse []interface{}

type PriceTicker struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Ticker24hr struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
}

// 特征数据结构
type SymbolFeatures struct {
	Symbol           string    `json:"symbol"`
	Timestamp        time.Time `json:"timestamp"`
	Price            float64   `json:"price"`
	PriceChange15Min float64   `json:"price_change_15min"`
	PriceChange1H    float64   `json:"price_change_1h"`
	PriceChange4H    float64   `json:"price_change_4h"`
	Volume           float64   `json:"volume"`
	VolumeRatio5     float64   `json:"volume_ratio_5"`
	VolumeRatio20    float64   `json:"volume_ratio_20"`
	VolumeTrend      float64   `json:"volume_trend"`
	RSI14            float64   `json:"rsi_14"`
	SMA5             float64   `json:"sma_5"`
	SMA10            float64   `json:"sma_10"`
	SMA20            float64   `json:"sma_20"`
	HighLowRatio     float64   `json:"high_low_ratio"`
	Volatility20     float64   `json:"volatility_20"`
	PositionInRange  float64   `json:"position_in_range"`
}

// 警报数据结构
type Alert struct {
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type Config struct {
	AlertThresholds AlertThresholds `json:"alert_thresholds"`
	UpdateInterval  int             `json:"update_interval"` // seconds
	CleanupConfig   CleanupConfig   `json:"cleanup_config"`
}

type AlertThresholds struct {
	VolumeSpike      float64 `json:"volume_spike"`
	PriceChange15Min float64 `json:"price_change_15min"`
	VolumeTrend      float64 `json:"volume_trend"`
	RSIOverbought    float64 `json:"rsi_overbought"`
	RSIOversold      float64 `json:"rsi_oversold"`
}
type CleanupConfig struct {
	InactiveTimeout   time.Duration `json:"inactive_timeout"`    // 不活跃超时时间
	MinScoreThreshold float64       `json:"min_score_threshold"` // 最低评分阈值
	NoAlertTimeout    time.Duration `json:"no_alert_timeout"`    // 无警报超时时间
	CheckInterval     time.Duration `json:"check_interval"`      // 检查间隔
}

var config = Config{
	AlertThresholds: AlertThresholds{
		VolumeSpike:      3.0,
		PriceChange15Min: 0.05,
		VolumeTrend:      2.0,
		RSIOverbought:    70,
		RSIOversold:      30,
	},
	CleanupConfig: CleanupConfig{
		InactiveTimeout:   30 * time.Minute,
		MinScoreThreshold: 15.0,
		NoAlertTimeout:    20 * time.Minute,
		CheckInterval:     5 * time.Minute,
	},
	UpdateInterval: 60, // 1 minute
}
