package config

// ServerConfig holds network bindings and game metadata for the local backend.
type ServerConfig struct {
	BindHost      string
	AdvertiseHost string
	MainHTTPPort  int
	ReportPort    int
	WordCheckPort int
	LoginTCPPort  int
	GameTCPPort   int
	AppVersion    string
	PatchVersion  int
	Channel       string
}

// GameServerInfo represents a single server entry returned by /servers.
type GameServerInfo struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Status    int    `json:"status"`
	Recommend bool   `json:"recommend"`
	IsNew     bool   `json:"is_new"`
}

func Default() ServerConfig {
	return ServerConfig{
		BindHost:      "0.0.0.0",
		AdvertiseHost: "127.0.0.1",
		MainHTTPPort:  18080,
		ReportPort:    1104,
		WordCheckPort: 1144,
		LoginTCPPort:  16666,
		GameTCPPort:   10888,
		AppVersion:    "2.1.0.7bfcc041a7",
		PatchVersion:  46,
		Channel:       "shuma",
	}
}
