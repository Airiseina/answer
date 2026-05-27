package mcp

import (
	"github.com/spf13/viper"
)

type BuiltinServer struct {
	Name      string
	URL       string
	Transport string
}

func GetBuiltinServers() []BuiltinServer {
	return []BuiltinServer{
		{
			Name:      "mem0",
			URL:       viper.GetString("mcp.mem0_url"),
			Transport: "sse",
		},
		{
			Name:      "weather",
			URL:       viper.GetString("mcp.weather_url"),
			Transport: "sse",
		},
		{
			Name:      "brave-search",
			URL:       viper.GetString("mcp.brave_search_url"),
			Transport: "http",
		},
		{
			Name:      "translate",
			URL:       viper.GetString("mcp.translate_url"),
			Transport: "sse",
		},
		{
			Name:      "timeserver",
			URL:       viper.GetString("mcp.timeserver_url"),
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
