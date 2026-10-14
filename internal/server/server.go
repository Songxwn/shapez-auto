package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"shapez-auto/internal/llm"
	"shapez-auto/internal/plan"
)

// Config 是控制面保存的设置。
type Config struct {
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
	Model    string `json:"model"`
	Warp     int    `json:"warp"`
	MaxLevel int    `json:"maxLevel"`
	Freeplay bool   `json:"freeplay"`
}

// Service 是本机控制面。
type Service struct {
	mu       sync.Mutex
	cfg      Config
	cfgPath  string
	running  bool
	logs     []string
	lastReq  plan.Request
	lastPlan plan.Plan
	index    []byte
	mod      []byte
}

// New 创建控制面并读取已保存的设置。
func New(index, modJS []byte) *Service {
	cfg := Config{
		BaseURL:  "https://api.openai.com/v1",
		Model:    "gpt-4o-mini",
		Warp:     10,
		MaxLevel: 26,
	}
	path := filepath.Join(configDir(), "config.json")
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &cfg)
	}
	if cfg.Warp < 1 {
		cfg.Warp = 10
	}
	if cfg.MaxLevel < 1 {
		cfg.MaxLevel = 26
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	s := &Service{cfg: cfg, cfgPath: path, index: index, mod: modJS}
	s.log("控制面已启动")
	return s
}

// Handler 提供页面和接口。
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.page)
	mux.HandleFunc("/api/status", s.status)
	mux.HandleFunc("/api/config", s.saveConfig)
	mux.HandleFunc("/api/install-mod", s.installMod)
	mux.HandleFunc("/api/start", s.start)
	mux.HandleFunc("/api/stop", s.stop)
	mux.HandleFunc("/api/tick", s.tick)
	return cors(mux)
}

func (s *Service) page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(s.index)
}

func (s *Service) status(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, map[string]any{
		"running":  s.running,
		"baseUrl":  s.cfg.BaseURL,
		"model":    s.cfg.Model,
		"hasKey":   strings.TrimSpace(s.cfg.APIKey) != "",
		"warp":     s.cfg.Warp,
		"maxLevel": s.cfg.MaxLevel,
		"freeplay": s.cfg.Freeplay,
		"level":    s.lastReq.Level,
		"shape":    s.lastReq.Shape,
		"required": s.lastReq.Required,
		"delivered": s.lastReq.Delivered,
		"source":   s.lastPlan.Source,
		"comment":  s.lastPlan.Comment,
		"action":   s.lastPlan.Action,
		"error":    s.lastPlan.Error,
		"modsPath": modsDir(),
		"logs":     append([]string(nil), s.logs...),
	})
}

func (s *Service) saveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var next Config
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&next); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(next.APIKey) == "" {
		next.APIKey = s.cfg.APIKey
	}
	if next.Warp < 1 {
		next.Warp = 10
	}
	if next.MaxLevel < 1 {
		next.MaxLevel = 26
	}
	if strings.TrimSpace(next.BaseURL) == "" {
		next.BaseURL = "https://api.openai.com/v1"
	}
	if strings.TrimSpace(next.Model) == "" {
		next.Model = "gpt-4o-mini"
	}
	s.cfg = next
	if err := s.persist(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.log("已保存设置")
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Service) installMod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	dir := modsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	target := filepath.Join(dir, "shapez-auto.js")
	if err := os.WriteFile(target, s.mod, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.log("模组已写入 " + target + "。请完全退出并重新打开 shapez.io")
	s.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true, "path": target})
}

func (s *Service) start(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.running = true
	s.lastPlan = plan.Plan{}
	s.log("开始自动通关。进入一局游戏后，模组会上报关卡")
	s.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Service) stop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.running = false
	s.log("已停止。游戏里已摆下的工厂会保留")
	s.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Service) tick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req plan.Request
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.lastReq = req
	if !s.running {
		s.mu.Unlock()
		writeJSON(w, plan.Plan{Action: "idle", Comment: "控制面尚未开始"})
		return
	}
	if req.DemoEnded || req.Required == 0 {
		s.running = false
		s.log("游戏停在演示终点，自动通关结束")
		s.mu.Unlock()
		writeJSON(w, plan.Plan{Action: "done", Comment: "已到演示终点"})
		return
	}
	if req.Freeplay && !s.cfg.Freeplay {
		s.running = false
		s.log(fmt.Sprintf("主线第 %d 关已完成，进入自由模式", s.cfg.MaxLevel))
		s.mu.Unlock()
		writeJSON(w, plan.Plan{Action: "done", Comment: "主线已完成"})
		return
	}
	if !req.Freeplay && s.cfg.MaxLevel > 0 && req.Level > s.cfg.MaxLevel {
		s.running = false
		s.log(fmt.Sprintf("已超过设定的第 %d 关", s.cfg.MaxLevel))
		s.mu.Unlock()
		writeJSON(w, plan.Plan{Action: "done", Comment: "已达到设定关卡"})
		return
	}
	if s.lastPlan.Action == "build" && s.lastPlan.Level == req.Level && s.lastPlan.Shape == req.Shape && s.lastPlan.ID != "" {
		cached := s.lastPlan
		s.mu.Unlock()
		writeJSON(w, cached)
		return
	}
	cfg := s.cfg
	s.log(fmt.Sprintf("第 %d 关 %s，开始规划", req.Level, req.Shape))
	s.mu.Unlock()

	built := s.build(req, cfg)

	s.mu.Lock()
	s.lastPlan = built
	if built.Error != "" {
		s.log("规划失败: " + built.Error)
	} else {
		s.log(fmt.Sprintf("第 %d 关使用%s，放置 %d 个建筑", req.Level, sourceName(built.Source), len(built.Placements)))
	}
	s.mu.Unlock()
	writeJSON(w, built)
}

func (s *Service) build(req plan.Request, cfg Config) plan.Plan {
	var override *plan.Program
	if strings.TrimSpace(cfg.APIKey) != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 22*time.Second)
		defer cancel()
		program, err := llm.Complete(ctx, cfg.BaseURL, cfg.APIKey, cfg.Model, req.Shape, req.Rewards, resourceSummary(req.Resources))
		if err != nil {
			s.mu.Lock()
			s.log("外部模型不可用，改用内置求解: " + err.Error())
			s.mu.Unlock()
		} else {
			override = program
		}
	}
	built, err := plan.Compile(req, cfg.Warp, override)
	if err != nil {
		return plan.Plan{Action: "error", Error: err.Error(), Level: req.Level, Shape: req.Shape}
	}
	return built
}

func resourceSummary(resources []plan.Resource) []string {
	seen := map[string]int{}
	var order []string
	for _, r := range resources {
		key := r.Kind + ":" + r.Key
		if _, ok := seen[key]; !ok {
			order = append(order, key)
		}
		seen[key]++
	}
	out := make([]string, 0, len(order))
	for _, key := range order {
		out = append(out, fmt.Sprintf("%s x%d", key, seen[key]))
	}
	return out
}

func sourceName(source string) string {
	if source == "llm" {
		return "外部模型方案"
	}
	return "内置求解"
}

func (s *Service) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.cfgPath), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.cfgPath, raw, 0o600)
}

func (s *Service) log(line string) {
	stamp := time.Now().Format("15:04:05")
	s.logs = append(s.logs, stamp+"  "+line)
	if len(s.logs) > 80 {
		s.logs = s.logs[len(s.logs)-80:]
	}
}

func configDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
	}
	return filepath.Join(base, "shapez-auto")
}

func modsDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
	}
	return filepath.Join(base, "shapez.io", "mods")
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
