package login

import (
	"encoding/json"
	"fmt"
	"log"

	"shuma-server/internal/config"
)

func Handle(cfg config.ServerConfig, route string, req any) map[string]any {
	return HandleWithAccounts(cfg, nil, route, req)
}

func HandleWithAccounts(cfg config.ServerConfig, accounts *AccountStore, route string, req any) map[string]any {
	switch route {
	case "/login/check":
		name := "shuma_100001"
		if m, ok := req.(map[string]any); ok {
			if n, ok := m["name"].(string); ok && n != "" {
				name = n
			}
		}
		var (
			rec   = AccountRecord{ID: 100001, Name: name, Channel: cfg.Channel, RoleInfos: map[string]RoleSummary{}}
			isNew bool
		)
		if accounts != nil {
			r, n, err := accounts.CheckOrRegister(name, cfg.Channel)
			if err != nil {
				return map[string]any{"ret": false, "err": err.Error()}
			}
			rec, isNew = r, n
		}
		roleInfos := make(map[string]any, len(rec.RoleInfos))
		for k, v := range rec.RoleInfos {
			roleInfos[k] = map[string]any{
				"name":   v.Name,
				"level":  v.Level,
				"figure": v.Figure,
			}
		}
		return map[string]any{
			"ret":         true,
			"name":        rec.Name,
			"session_pwd": fmt.Sprintf("sess_%d", rec.ID),
			"is_new":      isNew,
			"account": map[string]any{
				"id":         rec.ID,
				"name":       rec.Name,
				"channel":    cfg.Channel,
				"role_infos": roleInfos,
			},
		}
	case "/login/confirm":
		if accounts != nil {
			if m, ok := req.(map[string]any); ok {
				if n, ok := m["name"].(string); ok && n != "" {
					_, _, _ = accounts.CheckOrRegister(n, cfg.Channel)
				}
			}
		}
		servers := []map[string]any{
			{
				"id":     1,
				"name":   "1区 本地测试服",
				"key":    "game.cn.1",
				"status": 2,
				"addr":   fmt.Sprintf("%s:%d", cfg.AdvertiseHost, cfg.GameTCPPort),
			},
		}
		raw, _ := json.Marshal(servers)
		return map[string]any{
			"ret":     true,
			"servers": string(raw),
		}
	case "/login/enter_server":
		servKey := "game.cn.1"
		if m, ok := req.(map[string]any); ok {
			if k, ok := m["key"].(string); ok && k != "" {
				servKey = k
			}
		}
		if accounts != nil {
			accounts.BeginEnterServer(servKey)
		}
		return map[string]any{
			"ret":      true,
			"serv_key": servKey,
		}
	case "/login/ok":
		return map[string]any{"ret": true}
	default:
		log.Printf("[login] Unhandled route %s", route)
		return map[string]any{"ret": true}
	}
}
