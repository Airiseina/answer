package rpc

import (
	"github.com/cloudwego/kitex/pkg/klog"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

func Connect() {
	etcdAddr := viper.GetString("etcd.Addr")
	r, err := etcd.NewEtcdResolver([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("连接etcd出错:%v", err)
	}
	ConnectChatService(r)
}
