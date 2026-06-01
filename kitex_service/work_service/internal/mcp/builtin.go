package mcp

import (
	"github.com/Airiseina/answer/kitex_service/work_service/internal/config"
)

type BuiltinServer struct {
	Name      string
	URL       string
	Transport string
}

func GetBuiltinServers() []BuiltinServer {
	v := config.V
	return []BuiltinServer{
		{
			Name:      "mem0",
			URL:       v.GetString("mcp.mem0_url"),
			Transport: "sse",
		},
		{
			Name:      "knowledge",
			URL:       v.GetString("mcp.knowledge_url"),
			Transport: "sse",
		},
		{
			Name:      "searxng",
			URL:       v.GetString("mcp.searxng_url"),
			Transport: "sse",
		},
		{
			Name:      "weather",
			URL:       v.GetString("mcp.weather_url"),
			Transport: "sse",
		},
		{
			Name:      "timeserver",
			URL:       v.GetString("mcp.timeserver_url"),
			Transport: "sse",
		},
	}
}

func GetBuiltinServerConfigs() []ServerConfig {
	builtins := GetBuiltinServers()
	configs := make([]ServerConfig, 0, len(builtins))
	for _, b := range builtins {
		if b.URL == "" {
			continue
		}
		configs = append(configs, ServerConfig{
			Name:      b.Name,
			URL:       b.URL,
			Transport: b.Transport,
		})
	}
	return configs
}

func FilterAgentServers(servers []ServerConfig) []ServerConfig {
	internalNames := map[string]bool{
		"mem0":      true,
		"knowledge": true,
	}
	filtered := make([]ServerConfig, 0, len(servers))
	for _, s := range servers {
		if !internalNames[s.Name] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
