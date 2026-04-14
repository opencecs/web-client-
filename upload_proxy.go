package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ContainerUploadProxy 容器文件上传代理
type ContainerUploadProxy struct {
	auth *AuthService
	hub  *WSHub
}

// containerAPIPort 计算容器内部 API 端口
func containerAPIPort(hub *WSHub, containerName string) int {
	hub.containerMu.RLock()
	parsed := hub.parsedContainers
	hub.containerMu.RUnlock()

	for _, ct := range parsed {
		if ct.Name == containerName {
			return 30000 + (ct.IndexNum-1)*100 + 1
		}
	}
	return 0
}

// HandleUpload 处理文件上传转发到容器 POST /upload
func (p *ContainerUploadProxy) HandleUpload(w http.ResponseWriter, r *http.Request) {
	containerName := chi.URLParam(r, "name")
	if msg := p.checkAccess(r, containerName); msg != "" {
		jsonError(w, msg, 403)
		return
	}
	port := containerAPIPort(p.hub, containerName)
	if port == 0 {
		jsonError(w, "找不到容器", 404)
		return
	}
	p.forwardMultipart(w, r, fmt.Sprintf("http://127.0.0.1:%d/upload", port), containerName)
}

// HandleCert 处理证书上传转发到容器 POST /uploadcert
func (p *ContainerUploadProxy) HandleCert(w http.ResponseWriter, r *http.Request) {
	containerName := chi.URLParam(r, "name")
	if msg := p.checkAccess(r, containerName); msg != "" {
		jsonError(w, msg, 403)
		return
	}
	port := containerAPIPort(p.hub, containerName)
	if port == 0 {
		jsonError(w, "找不到容器", 404)
		return
	}
	p.forwardMultipart(w, r, fmt.Sprintf("http://127.0.0.1:%d/uploadcert", port), containerName)
}

// checkAccess 检查用户对容器的坑位权限
func (p *ContainerUploadProxy) checkAccess(r *http.Request, containerName string) string {
	claims := r.Context().Value(userContextKey).(*Claims)
	if claims.Role == "admin" {
		return ""
	}
	perms := p.auth.GetUserPermissions(claims.UserID)
	if perms == nil || !perms.ContainerStart {
		return "无权限"
	}
	// 检查坑位权限
	p.hub.containerMu.RLock()
	parsed := p.hub.parsedContainers
	p.hub.containerMu.RUnlock()
	for _, ct := range parsed {
		if ct.Name == containerName {
			for _, s := range perms.Slots {
				if s == ct.IndexNum {
					return ""
				}
			}
			return "无权操作该坑位"
		}
	}
	return "找不到容器"
}

// forwardMultipart 流式转发 multipart 文件到目标 URL（不缓存整个文件到内存）
func (p *ContainerUploadProxy) forwardMultipart(w http.ResponseWriter, r *http.Request, targetURL, containerName string) {
	// 直接从请求体流式读取 multipart，不调用 ParseMultipartForm
	reader, err := r.MultipartReader()
	if err != nil {
		jsonError(w, "文件解析失败: "+err.Error(), 400)
		return
	}

	// 找到 file 字段
	var part *multipart.Part
	for {
		p, err := reader.NextPart()
		if err != nil {
			jsonError(w, "缺少文件", 400)
			return
		}
		if p.FormName() == "file" {
			part = p
			break
		}
		p.Close()
	}
	defer part.Close()

	filename := part.FileName()
	log.Printf("[Upload] 容器 %s: 上传 %s → %s", containerName, filename, targetURL)

	// 构建转发请求，流式 pipe
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		dst, err := writer.CreateFormFile("file", filename)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(dst, part); err != nil {
			pw.CloseWithError(err)
			return
		}
		writer.Close()
	}()

	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest("POST", targetURL, pr)
	if err != nil {
		jsonError(w, "请求创建失败", 500)
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		jsonError(w, "容器未响应: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()

	// 透传响应
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	// APK 自动安装
	if strings.HasSuffix(strings.ToLower(filename), ".apk") && resp.StatusCode < 400 {
		go p.autoInstallAPK(containerName, filename)
	}
}

// autoInstallAPK 上传完成后通过 exec 在容器内执行 pm install
func (p *ContainerUploadProxy) autoInstallAPK(containerName, filename string) {
	srcPath := "/sdcard/upload/" + filename
	tmpPath := "/data/local/tmp/" + filename
	log.Printf("[Upload] 容器 %s: 自动安装 APK %s", containerName, srcPath)

	execURL := fmt.Sprintf("http://%s/android/exec", p.hub.deviceAddr)
	client := &http.Client{Timeout: 120 * time.Second}

	// 先复制到 /data/local/tmp/（pm install 需要该目录权限）
	cpBody, _ := json.Marshal(map[string]interface{}{
		"name":    containerName,
		"command": []string{"sd", "-c", "cp " + srcPath + " " + tmpPath + " && chmod 644 " + tmpPath},
	})
	cpResp, err := client.Post(execURL, "application/json", strings.NewReader(string(cpBody)))
	if err != nil {
		log.Printf("[Upload] 复制 APK 失败: %v", err)
		return
	}
	cpResp.Body.Close()

	// 执行安装
	installBody, _ := json.Marshal(map[string]interface{}{
		"name":    containerName,
		"command": []string{"sd", "-c", "pm install -r " + tmpPath + " 2>&1; rm -f " + tmpPath},
	})
	resp, err := client.Post(execURL, "application/json", strings.NewReader(string(installBody)))
	if err != nil {
		log.Printf("[Upload] APK 安装请求失败: %v", err)
		return
	}
	defer resp.Body.Close()
	result, _ := io.ReadAll(resp.Body)
	log.Printf("[Upload] APK 安装结果: %s", string(result))
}
