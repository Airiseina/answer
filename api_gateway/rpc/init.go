package rpc

import (
	"github.com/cloudwego/hertz/pkg/common/hlog"
	etcd "github.com/kitex-contrib/registry-etcd"
)

func Connect() {
	r, err := etcd.NewEtcdResolver([]string{"127.0.0.1:2379"})
	if err != nil {
		hlog.Fatalf("连接etcd出错:%v", err)
	}
	ConnectUserService(r)
	ConnectGroupService(r)
}
