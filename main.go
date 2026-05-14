package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gopkg.in/natefinch/lumberjack.v2"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

//go:embed all:webplayer
var webplayerFS embed.FS

// 版本信息（构建时通过 -ldflags 注入）
var Version = "dev"
var Device = "r1s"                                         // 设备型号（构建时通过 -ldflags 注入）
var UpdateURL = "https://newapi.moyunteng.com/api/v1"      // 更新服务器地址

func main() {
	// ARM 大核绑定（自动检测）
	bindBigCores()

	port := flag.Int("port", 8181, "TCP/UDP server port")
	updateURL := flag.String("update-url", "", "自定义更新服务器地址")
	flag.Parse()

	if *updateURL != "" {
		UpdateURL = *updateURL
	}

	deviceAddr := "127.0.0.1:8000"

	// 初始化数据库
	db := InitDB("users.db")
	defer db.Close()

	// 服务
	authService := NewAuthService(db)
	deviceService := NewDeviceService(deviceAddr)
	mytAuthService := NewMytAuthService(db, deviceAddr, deviceService)
	sdkProxy := NewSDKProxy(deviceAddr)
	aliasService := NewContainerAliasService(db)
	wsHub := NewWSHub(authService, aliasService, deviceService, mytAuthService, deviceAddr)
	authService.wsHub = wsHub
	go wsHub.Run()
	go wsHub.PollContainers(5 * time.Second)

	// 投屏代理
	udpRegistry := NewSessionRegistry()
	projectionProxy := NewProjectionProxy(authService, wsHub, udpRegistry)
	wsHub.projProxy = projectionProxy

	// 投屏预热连接池已禁用：容器只允许单 WS 连接，预热池会增加连接延迟
	// go projectionProxy.StartWarmPool()

	// UDP 媒体流复用器（与 TCP 共用端口号）
	if err := StartUDPMux(*port, udpRegistry); err != nil {
		log.Fatal("UDP Mux 启动失败:", err)
	}

	// 截图轮询
	screenshotCache := NewScreenshotCache()
	go wsHub.PollScreenshots(screenshotCache)

	// 设备状态轮询
	go deviceService.PollStatus(wsHub, 5*time.Second)

	// 用户过期检查
	go authService.CheckExpiry(wsHub, 30*time.Second)

	// 自动更新（仅正式版）
	go StartAutoUpdate(wsHub)

	// 路由
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5, "text/html", "text/css", "text/javascript", "application/javascript", "application/json", "image/svg+xml"))
	r.Use(corsMiddleware)

	// 登录/登出（无需 JWT）
	r.Post("/api/auth/login", authService.HandleLogin)
	r.Post("/api/auth/logout", authService.HandleLogout)
	r.Get("/api/version", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]interface{}{"version": Version})
	})

	// 投屏代理（token 在 query 参数中，处理器自行验证）
	r.Get("/lgcloud", projectionProxy.HandleProjection)

	// 需要 JWT 的路由
	r.Group(func(r chi.Router) {
		r.Use(authService.JWTMiddleware)

		// 仅保留 auth/me（前端路由守卫需要）
		r.Get("/api/auth/me", authService.HandleMe)

		// 容器文件上传代理（所有用户可用，内部检查坑位权限）
		uploadProxy := &ContainerUploadProxy{auth: authService, hub: wsHub}
		r.Post("/api/container/{name}/upload", uploadProxy.HandleUpload)
		r.Post("/api/container/{name}/push-upload", uploadProxy.HandlePushUpload)
		r.Post("/api/container/{name}/cert", uploadProxy.HandleCert)
		r.Post("/api/container/{name}/keybox", uploadProxy.HandleKeybox)

			// 文件管理（上传到宿主机 mmc/upload 目录）
			fileManage := &FileManageHandler{auth: authService}
			r.Post("/api/file/upload", fileManage.HandleUpload)
			r.Get("/api/file/list", fileManage.HandleList)
			r.Delete("/api/file/delete", fileManage.HandleDelete)
			r.Get("/api/file/download", fileManage.HandleDownload)
		// WebSocket（所有业务走这里）
		r.Get("/ws", wsHub.HandleWS)

		// admin only
		r.Group(func(r chi.Router) {
			r.Use(authService.AdminOnly)
			// SSH WebSocket 代理
			r.Get("/ws/ssh", deviceService.HandleSSHProxy)
		})
		// SDK WebSocket 代理（容器终端 exec 等）- 所有登录用户可用
		r.HandleFunc("/api/sdk/*", sdkProxy.HandleProxy)
	})

	// webplayer 静态文件（ETag + 长缓存，避免重复传输）
	wpFS, _ := fs.Sub(webplayerFS, "webplayer")
	r.Handle("/webplayer/*", http.StripPrefix("/webplayer/", etagCacheHandler(wpFS, 86400*7)))

	// 前端 SPA
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatal("Failed to load frontend:", err)
	}
	fileServer := staticCacheHandler(http.FileServer(http.FS(distFS)), 86400*365)
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		f, err := distFS.Open(r.URL.Path[1:])
		if err != nil {
			// SPA fallback: index.html 不缓存
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			indexFile, _ := distFS.Open("index.html")
			if indexFile != nil {
				defer indexFile.Close()
				stat, _ := indexFile.Stat()
				http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(interface{ Read([]byte) (int, error); Seek(int64, int) (int64, error) }).(readSeeker))
				return
			}
			http.NotFound(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})

	listenAddr := fmt.Sprintf(":%d", *port)
	log.Printf("[魔云互联] Starting on %s", listenAddr)

	if err := http.ListenAndServe(listenAddr, r); err != nil {
		log.Fatal(err)
	}
}

type readSeeker interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// staticCacheHandler 为静态资源添加 Cache-Control 头
func staticCacheHandler(h http.Handler, maxAge int) http.Handler {
	cacheVal := fmt.Sprintf("public, max-age=%d, immutable", maxAge)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheVal)
		h.ServeHTTP(w, r)
	})
}

// etagCacheHandler 为 embed.FS 静态资源提供 ETag + If-None-Match 304 支持
func etagCacheHandler(fsys fs.FS, maxAge int) http.Handler {
	cacheVal := fmt.Sprintf("public, max-age=%d, immutable", maxAge)
	var etagCache sync.Map // path → etag string
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		// 计算或取缓存的 ETag
		etag, ok := etagCache.Load(path)
		if !ok {
			f, err := fsys.Open(path)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			h := sha256.New()
			io.Copy(h, f)
			f.Close()
			etag = `"` + hex.EncodeToString(h.Sum(nil))[:16] + `"`
			etagCache.Store(path, etag)
		}
		etagStr := etag.(string)
		w.Header().Set("ETag", etagStr)
		w.Header().Set("Cache-Control", cacheVal)
		// 检查 If-None-Match → 304
		if match := r.Header.Get("If-None-Match"); match == etagStr {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		// 首次请求，正常响应
		http.FileServer(http.FS(fsys)).ServeHTTP(w, r)
	})
}

func init() {
	// 日志输出到文件 + 轮转（程序所在目录下的 logs/）
	exePath, _ := os.Executable()
	logDir := filepath.Join(filepath.Dir(exePath), "logs")
	os.MkdirAll(logDir, 0755)
	log.SetOutput(&lumberjack.Logger{
		Filename:   filepath.Join(logDir, "myt-panel.log"),
		MaxSize:    50, // MB
		MaxBackups: 5,
		MaxAge:     30, // 天
		Compress:   true,
	})
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}
