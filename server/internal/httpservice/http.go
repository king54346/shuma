package httpservice

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"shuma-server/internal/config"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"ret":false,"code":500}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func readBodyPreview(r *http.Request, limit int64) string {
	if r.Body == nil {
		return ""
	}
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, limit))
	if err != nil {
		return ""
	}
	return string(data)
}

// NewMainHTTPMux builds the HTTP handler for port 18080 (/version, /servers, /notice, SDK).
func NewMainHTTPMux(cfg config.ServerConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := readBodyPreview(r, 4096)
		log.Printf("[18080] %s %s query=%q body=%q", r.Method, r.URL.Path, r.URL.RawQuery, body)

		path := strings.TrimRight(r.URL.Path, "/")
		if path == "" {
			path = "/"
		}

		switch path {
		case "/version":
			writeJSON(w, http.StatusOK, map[string]any{
				"ret":          true,
				"code":         0,
				"app_version":  cfg.AppVersion,
				"patch":        cfg.PatchVersion,
				"update_close": true,
				"patch_url":    "",
				"forShenhe":    false,
			})
		case "/servers":
			writeJSON(w, http.StatusOK, map[string]any{
				"ret":  true,
				"code": 0,
				"servers": []config.GameServerInfo{
					{
						ID:        1,
						Name:      "1区 本地测试服",
						IP:        cfg.AdvertiseHost,
						Port:      cfg.LoginTCPPort,
						Status:    1,
						Recommend: true,
						IsNew:     true,
					},
				},
			})
		case "/notice":
			writeJSON(w, http.StatusOK, map[string]any{
				"ret":  true,
				"code": 0,
				"notice": map[string]any{
					"id":      1,
					"title":   "本地服务器公告",
					"content": "已连接至 127.0.0.1 Go 本地测试后端。",
				},
			})
		default:
			writeJSON(w, http.StatusOK, map[string]any{
				"ret":    true,
				"code":   0,
				"status": "ok",
				"uid":    "100001",
				"token":  "local_dev_token",
				"path":   r.URL.Path,
			})
		}
	})
	return mux
}

// NewReportHTTPMux builds the HTTP handler for port 1104 (client reports & /feedback).
func NewReportHTTPMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := readBodyPreview(r, 4096)
		log.Printf("[1104] %s %s query=%q body=%q", r.Method, r.URL.Path, r.URL.RawQuery, body)
		writeJSON(w, http.StatusOK, map[string]any{
			"ret":    true,
			"code":   0,
			"status": "ok",
		})
	})
	return mux
}

// NewWordCheckHTTPMux builds the HTTP handler for port 1144 (/check).
func NewWordCheckHTTPMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := readBodyPreview(r, 4096)
		log.Printf("[1144] %s %s query=%q body=%q", r.Method, r.URL.Path, r.URL.RawQuery, body)
		writeJSON(w, http.StatusOK, map[string]any{
			"ret":   true,
			"code":  0,
			"valid": true,
			"word":  "",
		})
	})
	return mux
}
