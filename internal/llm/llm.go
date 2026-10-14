package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"shapez-auto/internal/plan"
)

// Complete 调用 OpenAI 兼容的对话接口，请模型给出一份加工步骤。
func Complete(ctx context.Context, baseURL, apiKey, model, shape string, rewards []string, resources []string) (*plan.Program, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("未配置 API Key")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	user, _ := json.Marshal(map[string]any{
		"target":    shape,
		"rewards":   rewards,
		"resources": resources,
	})
	body, _ := json.Marshal(map[string]any{
		"model":       model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": string(user)},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("模型接口 %s: %s", res.Status, trim(raw))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("%s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("模型没有返回内容")
	}
	obj := extractJSON(parsed.Choices[0].Message.Content)
	var program plan.Program
	if err := json.Unmarshal([]byte(obj), &program); err != nil {
		return nil, err
	}
	return &program, nil
}

func trim(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) > 300 {
		return text[:300]
	}
	return text
}

func extractJSON(text string) string {
	start := strings.Index(text, "{")
	if start < 0 {
		return text
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return text[start:]
}

const systemPrompt = `你是 shapez.io 的工厂规划器。只输出一个 JSON 对象，不要写解释。
形状短码的象限顺序是右上、右下、左下、左上，每象限两个字符（形状 C/R/S/W 或 -，颜色 r/g/b/y/p/c/w/u 或 -）。层从底到顶，用冒号分隔。
加工节点字段：id、op、in、lower、upper、shape、color、n。
op 只能是 mine-shape、mine-color、cut-left、cut-right、rot-cw、paint、stack、mix、deliver。
mine-shape 的 shape 是 C/R/S/W。mine-color 的 color 是 red/green/blue。
cut-left 是切割器朝北时的左出口（左下+左上），cut-right 是右出口（右上+右下）。
rot-cw 的 n 是顺时针 90 度的次数。paint 用 in 表示形状、lower 表示染料节点。
stack 的 lower 是下层、upper 是上层。mix 的 lower 和 upper 是两种染料。
最后用 deliver，in 指向成品。染料按光的加色混合：红+绿=黄，红+蓝=紫，绿+蓝=青，红+绿+蓝=白。
如果目标可以用整块、半块或单个象限做出来，不要拆得更碎。JSON 形如 {"comment":"...","nodes":[...]}。`
