package rpc

import (
	"github.com/cloudwego/hertz/pkg/common/hlog"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func Connect() {
	etcdAddr := viper.GetString("etcd.Addr")
	r, err := etcd.NewEtcdResolver([]string{etcdAddr})
	if err != nil {
		hlog.Fatalf("连接etcd出错:%v", err)
	}
	ConnectUserService(r)
	ConnectGroupService(r)
	ConnectChatService(r)
}
