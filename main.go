package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	TMDB_BASE_URL    = "https://api.themoviedb.org"
	CACHE_DURATION   = 10 * time.Minute
	MAX_CACHE_SIZE   = 1000
	CLEANUP_INTERVAL = 10 * time.Minute
)

// 缓存条目结构
type CacheEntry struct {
	Data   []byte
	Expiry time.Time
}

// 缓存管理器
type CacheManager struct {
	cache map[string]*CacheEntry
	mu    sync.RWMutex
}

// 创建新的缓存管理器
func NewCacheManager() *CacheManager {
	cm := &CacheManager{
		cache: make(map[string]*CacheEntry),
	}
	// 启动定期清理协程
	go cm.startCleanup()
	return cm
}

// 获取缓存
func (cm *CacheManager) Get(key string) ([]byte, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	entry, exists := cm.cache[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.Expiry) {
		return nil, false
	}

	return entry.Data, true
}

// 设置缓存
func (cm *CacheManager) Set(key string, data []byte) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查缓存大小
	cm.checkCacheSize()

	cm.cache[key] = &CacheEntry{
		Data:   data,
		Expiry: time.Now().Add(CACHE_DURATION),
	}
}

// 检查并清理超出大小限制的缓存
func (cm *CacheManager) checkCacheSize() {
	if len(cm.cache) <= MAX_CACHE_SIZE {
		return
	}

	// 找出最旧的条目
	type entry struct {
		key    string
		expiry time.Time
	}
	entries := make([]entry, 0, len(cm.cache))

	for key, val := range cm.cache {
		entries = append(entries, entry{key: key, expiry: val.Expiry})
	}

	// 按过期时间排序
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].expiry.After(entries[j].expiry) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// 删除最旧的条目
	deleteCount := len(cm.cache) - MAX_CACHE_SIZE
	for i := 0; i < deleteCount; i++ {
		delete(cm.cache, entries[i].key)
	}

	log.Printf("Cleaned %d old cache entries", deleteCount)
}

// 清理过期缓存
func (cm *CacheManager) cleanExpiredCache() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for key, entry := range cm.cache {
		if now.After(entry.Expiry) {
			delete(cm.cache, key)
		}
	}
}

// 启动定期清理
func (cm *CacheManager) startCleanup() {
	ticker := time.NewTicker(CLEANUP_INTERVAL)
	defer ticker.Stop()

	for range ticker.C {
		cm.cleanExpiredCache()
	}
}

// 全局缓存管理器
var cacheManager = NewCacheManager()

// 处理请求的主函数
func handler(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// 处理 OPTIONS 请求
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 获取完整路径
	fullPath := r.URL.RequestURI()
	cacheKey := fullPath

	// 检查缓存
	if cachedData, found := cacheManager.Get(cacheKey); found {
		log.Printf("Cache hit: %s", fullPath)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cachedData)
		return
	}

	// 构建 TMDB 请求 URL
	tmdbURL := TMDB_BASE_URL + fullPath

	// 创建新请求
	req, err := http.NewRequest(r.Method, tmdbURL, r.Body)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 复制 Authorization header
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("TMDB API error: %v", err)
		sendErrorResponse(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		sendErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 只有响应状态码为 200 时才缓存
	if resp.StatusCode == http.StatusOK {
		cacheManager.Set(cacheKey, body)
		log.Printf("Cache miss and stored: %s", fullPath)
	} else {
		log.Printf("Response not cached due to non-200 status: %d", resp.StatusCode)
	}

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// 发送错误响应
func sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func main() {
	// 解析命令行参数
	port := flag.String("port", "8080", "Port to run the server on")
	flag.Parse()

	http.HandleFunc("/", handler)

	log.Printf("Server starting on port %s", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatal(err)
	}
}
