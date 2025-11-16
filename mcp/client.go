package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Provider AI提供商类型
type Provider string

const (
	ProviderDeepSeek   Provider = "deepseek"
	ProviderQwen       Provider = "qwen"
	ProviderGoogleAI   Provider = "googleai"   // Google AI (Gemini)
	ProviderChatGPT    Provider = "chatgpt"    // OpenAI ChatGPT (Chat Completions API)
	ProviderGPTs       Provider = "gpts"       // OpenAI GPTs (Assistant API)
	ProviderCustom     Provider = "custom"
)

// Client AI API配置
type Client struct {
	Provider     Provider
	APIKey       string
	BaseURL      string
	Model        string
	Timeout      time.Duration
	UseFullURL   bool // 是否使用完整URL（不添加/chat/completions）
	MaxTokens    int  // AI响应的最大token数
	AssistantID  string // OpenAI Assistant ID (用于GPTs)
	ThreadID     string // OpenAI Thread ID (用于GPTs，可选，为空则每次创建新thread)
}

func New() *Client {
	// 从环境变量读取 MaxTokens，默认 2000
	maxTokens := 2000
	if envMaxTokens := os.Getenv("AI_MAX_TOKENS"); envMaxTokens != "" {
		if parsed, err := strconv.Atoi(envMaxTokens); err == nil && parsed > 0 {
			maxTokens = parsed
			log.Printf("🔧 [MCP] 使用环境变量 AI_MAX_TOKENS: %d", maxTokens)
		} else {
			log.Printf("⚠️  [MCP] 环境变量 AI_MAX_TOKENS 无效 (%s)，使用默认值: %d", envMaxTokens, maxTokens)
		}
	}

	// 默认配置
	client := &Client{
		Provider:  ProviderDeepSeek,
		BaseURL:   "https://api.deepseek.com/v1",
		Model:     "deepseek-chat",
		Timeout:   120 * time.Second, // 增加到120秒，因为AI需要分析大量数据
		MaxTokens: maxTokens,
	}
	
	// 记录token使用情况（用于监控）
	log.Printf("🔧 [MCP] AI配置: MaxTokens=%d (可通过环境变量AI_MAX_TOKENS调整)", maxTokens)
	
	return client
}

// SetDeepSeekAPIKey 设置DeepSeek API密钥
// customURL 为空时使用默认URL，customModel 为空时使用默认模型
func (client *Client) SetDeepSeekAPIKey(apiKey string, customURL string, customModel string) {
	client.Provider = ProviderDeepSeek
	client.APIKey = apiKey
	if customURL != "" {
		client.BaseURL = customURL
		log.Printf("🔧 [MCP] DeepSeek 使用自定义 BaseURL: %s", customURL)
	} else {
		client.BaseURL = "https://api.deepseek.com/v1"
		log.Printf("🔧 [MCP] DeepSeek 使用默认 BaseURL: %s", client.BaseURL)
	}
	if customModel != "" {
		client.Model = customModel
		log.Printf("🔧 [MCP] DeepSeek 使用自定义 Model: %s", customModel)
	} else {
		client.Model = "deepseek-chat"
		log.Printf("🔧 [MCP] DeepSeek 使用默认 Model: %s", client.Model)
	}
	// 打印 API Key 的前后各4位用于验证
	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] DeepSeek API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
}

// SetQwenAPIKey 设置阿里云Qwen API密钥
// customURL 为空时使用默认URL，customModel 为空时使用默认模型
func (client *Client) SetQwenAPIKey(apiKey string, customURL string, customModel string) {
	client.Provider = ProviderQwen
	client.APIKey = apiKey
	if customURL != "" {
		client.BaseURL = customURL
		log.Printf("🔧 [MCP] Qwen 使用自定义 BaseURL: %s", customURL)
	} else {
		client.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		log.Printf("🔧 [MCP] Qwen 使用默认 BaseURL: %s", client.BaseURL)
	}
	if customModel != "" {
		client.Model = customModel
		log.Printf("🔧 [MCP] Qwen 使用自定义 Model: %s", customModel)
	} else {
		client.Model = "qwen3-max" 
		log.Printf("🔧 [MCP] Qwen 使用默认 Model: %s", client.Model)
	}
	// 打印 API Key 的前后各4位用于验证
	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] Qwen API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
}

// SetGoogleAIAPIKey 设置Google AI (Gemini) API密钥
// customURL 为空时使用默认URL，customModel 为空时使用默认模型
func (client *Client) SetGoogleAIAPIKey(apiKey string, customURL string, customModel string) {
	client.Provider = ProviderGoogleAI
	client.APIKey = apiKey
	if customURL != "" {
		client.BaseURL = customURL
		log.Printf("🔧 [MCP] Google AI 使用自定义 BaseURL: %s", customURL)
	} else {
		// 使用 v1 API 版本（更稳定）
		client.BaseURL = "https://generativeai.googleapis.com/v1"
		log.Printf("🔧 [MCP] Google AI 使用默认 BaseURL: %s", client.BaseURL)
	}
	if customModel != "" {
		client.Model = customModel
		log.Printf("🔧 [MCP] Google AI 使用自定义 Model: %s", customModel)
	} else {
		// 使用 gemini-1.5-flash（更快、更便宜）或 gemini-1.5-pro（更强）
		client.Model = "gemini-1.5-flash"
		log.Printf("🔧 [MCP] Google AI 使用默认 Model: %s", client.Model)
	}
	client.UseFullURL = true // Google AI 使用完整URL
	// 打印 API Key 的前后各4位用于验证
	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] Google AI API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
}

// SetChatGPTAPIKey 设置OpenAI ChatGPT API密钥（Chat Completions API）
// customURL 为空时使用默认URL，customModel 为空时使用默认模型
func (client *Client) SetChatGPTAPIKey(apiKey string, customURL string, customModel string) {
	client.Provider = ProviderChatGPT
	client.APIKey = apiKey
	if customURL != "" {
		client.BaseURL = customURL
		log.Printf("🔧 [MCP] ChatGPT 使用自定义 BaseURL: %s", customURL)
	} else {
		client.BaseURL = "https://api.openai.com/v1"
		log.Printf("🔧 [MCP] ChatGPT 使用默认 BaseURL: %s", client.BaseURL)
	}
	if customModel != "" {
		client.Model = customModel
		log.Printf("🔧 [MCP] ChatGPT 使用自定义 Model: %s", customModel)
	} else {
		client.Model = "gpt-4o-mini" // 使用较新的模型，成本更低
		log.Printf("🔧 [MCP] ChatGPT 使用默认 Model: %s", client.Model)
	}
	client.UseFullURL = false // ChatGPT 使用标准路径
	// 打印 API Key 的前后各4位用于验证
	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] ChatGPT API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
}

// SetGPTsAPIKey 设置OpenAI GPTs API密钥（Assistant API）
// assistantID: GPTs的Assistant ID（必需）
// threadID: Thread ID（可选，为空则每次创建新thread）
// customURL 为空时使用默认URL
func (client *Client) SetGPTsAPIKey(apiKey string, assistantID string, threadID string, customURL string) {
	client.Provider = ProviderGPTs
	client.APIKey = apiKey
	client.AssistantID = assistantID
	client.ThreadID = threadID
	if customURL != "" {
		client.BaseURL = customURL
		log.Printf("🔧 [MCP] GPTs 使用自定义 BaseURL: %s", customURL)
	} else {
		client.BaseURL = "https://api.openai.com/v1"
		log.Printf("🔧 [MCP] GPTs 使用默认 BaseURL: %s", client.BaseURL)
	}
	client.UseFullURL = false // GPTs 使用标准路径
	log.Printf("🔧 [MCP] GPTs Assistant ID: %s", assistantID)
	if threadID != "" {
		log.Printf("🔧 [MCP] GPTs Thread ID: %s (将复用现有thread)", threadID)
	} else {
		log.Printf("🔧 [MCP] GPTs Thread ID: 未设置 (每次创建新thread)")
	}
	// 打印 API Key 的前后各4位用于验证
	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] GPTs API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
}

// SetCustomAPI 设置自定义OpenAI兼容API
func (client *Client) SetCustomAPI(apiURL, apiKey, modelName string) {
	client.Provider = ProviderCustom
	client.APIKey = apiKey

	// 检查URL是否以#结尾，如果是则使用完整URL（不添加/chat/completions）
	if strings.HasSuffix(apiURL, "#") {
		client.BaseURL = strings.TrimSuffix(apiURL, "#")
		client.UseFullURL = true
	} else {
		client.BaseURL = apiURL
		client.UseFullURL = false
	}

	client.Model = modelName
	client.Timeout = 120 * time.Second
}

// SetClient 设置完整的AI配置（高级用户）
func (client *Client) SetClient(Client Client) {
	if Client.Timeout == 0 {
		Client.Timeout = 30 * time.Second
	}
	client = &Client
}

// CallWithMessages 使用 system + user prompt 调用AI API（推荐）
func (client *Client) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	if client.APIKey == "" {
		return "", fmt.Errorf("AI API密钥未设置，请先调用相应的 SetXXXAPIKey() 方法")
	}

	// 重试配置（增加重试次数和间隔）
	maxRetries := 5
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("⚠️  AI API调用失败，正在重试 (%d/%d)...", attempt, maxRetries)
		}

		result, err := client.callOnce(systemPrompt, userPrompt)
		if err == nil {
			if attempt > 1 {
				log.Printf("✓ AI API重试成功")
			}
			return result, nil
		}

		lastErr = err
		// 如果不是网络错误，不重试
		if !isRetryableError(err) {
			return "", err
		}

		// 重试前等待（指数退避：2秒、4秒、8秒、16秒）
		if attempt < maxRetries {
			waitTime := time.Duration(1<<uint(attempt-1)) * 2 * time.Second
			if waitTime > 30*time.Second {
				waitTime = 30 * time.Second // 最大等待30秒
			}
			log.Printf("⏳ 等待%v后重试...", waitTime)
			time.Sleep(waitTime)
		}
	}

	return "", fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
}

// callOnce 单次调用AI API（内部使用）
func (client *Client) callOnce(systemPrompt, userPrompt string) (string, error) {
	// 打印当前 AI 配置
	log.Printf("📡 [MCP] AI 请求配置:")
	log.Printf("   Provider: %s", client.Provider)
	log.Printf("   BaseURL: %s", client.BaseURL)
	log.Printf("   Model: %s", client.Model)
	log.Printf("   UseFullURL: %v", client.UseFullURL)
	if len(client.APIKey) > 8 {
		log.Printf("   API Key: %s...%s", client.APIKey[:4], client.APIKey[len(client.APIKey)-4:])
	}

	// Google AI (Gemini) 使用不同的API格式
	if client.Provider == ProviderGoogleAI {
		return client.callGoogleAI(systemPrompt, userPrompt)
	}

	// OpenAI GPTs 使用 Assistant API
	if client.Provider == ProviderGPTs {
		return client.callGPTs(systemPrompt, userPrompt)
	}

	// 构建 messages 数组
	messages := []map[string]string{}

	// 如果有 system prompt，添加 system message
	if systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}

	// 添加 user message
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userPrompt,
	})

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       client.Model,
		"messages":    messages,
		"temperature": 0.5, // 降低temperature以提高JSON格式稳定性
		"max_tokens":  client.MaxTokens,
	}

	// ChatGPT 支持 response_format 参数
	if client.Provider == ProviderChatGPT {
		// OpenAI 支持结构化输出，但为了兼容性，我们仍然通过 prompt 来确保 JSON 格式
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建HTTP请求
	var url string
	if client.UseFullURL {
		// 使用完整URL，不添加/chat/completions
		url = client.BaseURL
	} else {
		// 默认行为：添加/chat/completions
		url = fmt.Sprintf("%s/chat/completions", client.BaseURL)
	}
	log.Printf("📡 [MCP] 请求 URL: %s", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 根据不同的Provider设置认证方式
	switch client.Provider {
	case ProviderDeepSeek:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
	case ProviderQwen:
		// 阿里云Qwen使用API-Key认证
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
	case ProviderChatGPT:
		// OpenAI ChatGPT使用Bearer认证
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
	default:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
	}

	// 发送请求
	httpClient := &http.Client{Timeout: client.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("API返回空响应")
	}

	return result.Choices[0].Message.Content, nil
}

// callGoogleAI 调用Google AI (Gemini) API
func (client *Client) callGoogleAI(systemPrompt, userPrompt string) (string, error) {
	// Google AI (Gemini) 使用不同的API格式
	// URL格式: https://generativeai.googleapis.com/v1/models/{model}:generateContent?key={API_KEY}
	// 注意：如果 BaseURL 已经包含完整路径，直接使用；否则构建完整路径
	var url string
	if strings.Contains(client.BaseURL, "/models/") {
		// BaseURL 已经包含完整路径
		url = fmt.Sprintf("%s:generateContent?key=%s", client.BaseURL, client.APIKey)
	} else {
		// 构建完整路径
		url = fmt.Sprintf("%s/models/%s:generateContent?key=%s", client.BaseURL, client.Model, client.APIKey)
	}
	log.Printf("📡 [MCP] Google AI 请求 URL: %s", url)

	// 构建请求体 - Google AI 使用 contents 数组
	contents := []map[string]interface{}{}

	// 添加 user message
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{"text": userPrompt},
		},
	})

	requestBody := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     0.5,
			"maxOutputTokens": client.MaxTokens,
		},
	}

	// 如果有 system prompt，使用 systemInstruction 字段（Gemini 1.5+ 支持）
	if systemPrompt != "" {
		requestBody["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": systemPrompt},
			},
		}
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	httpClient := &http.Client{Timeout: client.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 解析 Google AI 响应格式
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("API返回空响应")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// callGPTs 调用OpenAI GPTs (Assistant API)
func (client *Client) callGPTs(systemPrompt, userPrompt string) (string, error) {
	// OpenAI GPTs 使用 Assistant API
	// 流程：1. 创建或获取Thread 2. 添加消息 3. 运行Assistant 4. 获取响应

	if client.AssistantID == "" {
		return "", fmt.Errorf("GPTs Assistant ID 未设置")
	}

	httpClient := &http.Client{Timeout: client.Timeout}

	// 1. 创建或获取Thread
	threadID := client.ThreadID
	if threadID == "" {
		// 创建新Thread（可以同时添加第一条消息）
		createThreadURL := fmt.Sprintf("%s/threads", client.BaseURL)
		
		// 构建创建Thread的请求体
		createThreadBody := map[string]interface{}{}
		
		// 如果有system prompt，将其添加到消息中作为第一条消息（GPTs的instructions在Assistant配置中）
		// 如果没有system prompt，只添加user prompt
		messageContent := userPrompt
		if systemPrompt != "" {
			messageContent = fmt.Sprintf("System Instructions: %s\n\nUser Request: %s", systemPrompt, userPrompt)
		}
		
		createThreadBody["messages"] = []map[string]interface{}{
			{
				"role":    "user",
				"content": messageContent,
			},
		}

		jsonData, err := json.Marshal(createThreadBody)
		if err != nil {
			return "", fmt.Errorf("序列化Thread创建请求失败: %w", err)
		}

		req, err := http.NewRequest("POST", createThreadURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("创建Thread请求失败: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
		req.Header.Set("OpenAI-Beta", "assistants=v2")

		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("发送Thread创建请求失败: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取Thread创建响应失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("创建Thread失败 (status %d): %s\n请求URL: %s\n请求体: %s", resp.StatusCode, string(body), createThreadURL, string(jsonData))
		}

		var threadResult struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &threadResult); err != nil {
			return "", fmt.Errorf("解析Thread创建响应失败: %w\n响应体: %s", err, string(body))
		}

		threadID = threadResult.ID
		log.Printf("📡 [MCP] GPTs 创建新Thread: %s", threadID)
	} else {
		// 使用现有Thread，添加消息
		addMessageURL := fmt.Sprintf("%s/threads/%s/messages", client.BaseURL, threadID)
		
		// 构建消息内容
		messageContent := userPrompt
		if systemPrompt != "" {
			messageContent = fmt.Sprintf("System Instructions: %s\n\nUser Request: %s", systemPrompt, userPrompt)
		}
		
		addMessageBody := map[string]interface{}{
			"role":    "user",
			"content": messageContent,
		}

		jsonData, err := json.Marshal(addMessageBody)
		if err != nil {
			return "", fmt.Errorf("序列化消息添加请求失败: %w", err)
		}

		req, err := http.NewRequest("POST", addMessageURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("创建消息添加请求失败: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
		req.Header.Set("OpenAI-Beta", "assistants=v2")

		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("发送消息添加请求失败: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取消息添加响应失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("添加消息失败 (status %d): %s\n请求URL: %s\n请求体: %s", resp.StatusCode, string(body), addMessageURL, string(jsonData))
		}

		log.Printf("📡 [MCP] GPTs 向现有Thread添加消息: %s", threadID)
	}

	// 2. 运行Assistant
	runURL := fmt.Sprintf("%s/threads/%s/runs", client.BaseURL, threadID)
	runBody := map[string]interface{}{
		"assistant_id": client.AssistantID,
	}

	jsonData, err := json.Marshal(runBody)
	if err != nil {
		return "", fmt.Errorf("序列化Run创建请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", runURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建Run请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送Run创建请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取Run创建响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("创建Run失败 (status %d): %s\n请求URL: %s\n请求体: %s\nAssistant ID: %s", resp.StatusCode, string(body), runURL, string(jsonData), client.AssistantID)
	}

	var runResult struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &runResult); err != nil {
		return "", fmt.Errorf("解析Run创建响应失败: %w", err)
	}

	runID := runResult.ID
	log.Printf("📡 [MCP] GPTs 创建Run: %s, 状态: %s", runID, runResult.Status)

	// 3. 等待Run完成（轮询）
	maxWaitTime := client.Timeout - 10*time.Second // 留10秒缓冲
	pollInterval := 2 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		checkRunURL := fmt.Sprintf("%s/threads/%s/runs/%s", client.BaseURL, threadID, runID)
		req, err := http.NewRequest("GET", checkRunURL, nil)
		if err != nil {
			return "", fmt.Errorf("创建Run检查请求失败: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
		req.Header.Set("OpenAI-Beta", "assistants=v2")

		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("发送Run检查请求失败: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("读取Run检查响应失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("检查Run状态失败 (status %d): %s", resp.StatusCode, string(body))
		}

		var runStatus struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(body, &runStatus); err != nil {
			return "", fmt.Errorf("解析Run状态响应失败: %w", err)
		}

		log.Printf("📡 [MCP] GPTs Run状态: %s", runStatus.Status)

		if runStatus.Status == "completed" {
			break
		} else if runStatus.Status == "failed" || runStatus.Status == "cancelled" || runStatus.Status == "expired" {
			return "", fmt.Errorf("Run失败或取消: %s", runStatus.Status)
		}

		// 等待后继续轮询
		time.Sleep(pollInterval)
	}

	// 4. 获取响应消息（按创建时间倒序，取第一条assistant消息）
	messagesURL := fmt.Sprintf("%s/threads/%s/messages?order=desc&limit=10", client.BaseURL, threadID)
	req, err = http.NewRequest("GET", messagesURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建消息获取请求失败: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err = httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送消息获取请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取消息获取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取消息失败 (status %d): %s\n请求URL: %s", resp.StatusCode, string(body), messagesURL)
	}

	var messagesResult struct {
		Data []struct {
			ID        string `json:"id"`
			Role      string `json:"role"`
			CreatedAt int64  `json:"created_at"`
			Content   []struct {
				Type string `json:"type"`
				Text struct {
					Value string `json:"value"`
				} `json:"text"`
			} `json:"content"`
		} `json:"data"`
		FirstID string `json:"first_id"`
		LastID  string `json:"last_id"`
	}

	if err := json.Unmarshal(body, &messagesResult); err != nil {
		return "", fmt.Errorf("解析消息响应失败: %w\n响应体: %s", err, string(body))
	}

	// 找到Assistant的最新回复（第一条assistant角色的消息）
	for _, message := range messagesResult.Data {
		if message.Role == "assistant" && len(message.Content) > 0 {
			// 找到文本内容
			for _, content := range message.Content {
				if content.Type == "text" && content.Text.Value != "" {
					log.Printf("📡 [MCP] GPTs 获取到响应 (Thread: %s, Message ID: %s)", threadID, message.ID)
					return content.Text.Value, nil
				}
			}
		}
	}

	return "", fmt.Errorf("未找到Assistant的回复\n响应数据: %s", string(body))
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	errStr := strings.ToLower(err.Error())
	// 网络错误、超时、EOF等可以重试
	retryableErrors := []string{
		"eof",
		"timeout",
		"connection reset",
		"connection refused",
		"connection closed",
		"broken pipe",
		"temporary failure",
		"no such host",
		"stream error",      // HTTP/2 stream 错误
		"internal_error",    // 服务端内部错误
		"network is unreachable",
		"i/o timeout",
		"context deadline exceeded",
		"read: connection reset",
		"write: broken pipe",
	}
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	return false
}
