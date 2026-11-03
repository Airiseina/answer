package config

import (
	pkgconfig "github.com/Airiseina/answer/pkg/config"
	"github.com/spf13/viper"
)

var V *viper.Viper

// AdminBootstrap 描述配置文件中引导创建的初始管理员。
type AdminBootstrap struct {
	Account  string `mapstructure:"account"`
	Name     string `mapstructure:"name"`
	Role     string `mapstructure:"role"`
	Password string `mapstructure:"password"`
}

func GetConfig() {
	V = pkgconfig.LoadConfig()
	V.SetDefault("mysql.host", "localhost")
	V.SetDefault("mysql.port", "3306")
	V.SetDefault("mysql.user", "root")
	V.SetDefault("mysql.password", "123456")
	V.SetDefault("mysql.name", "answer")
	V.SetDefault("etcd.Addr", "127.0.0.1:2379")
	V.SetDefault("otel.Addr", "localhost:4317")
}

// BootstrapAdmins 读取配置中的初始管理员列表。
// 配置缺失时返回空列表，不阻塞服务启动。
func BootstrapAdmins() []AdminBootstrap {
	var list []AdminBootstrap
	if V == nil {
		return nil
	}
	_ = V.UnmarshalKey("admin.bootstrap", &list)
	return list
}
