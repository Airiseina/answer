package ai

import "github.com/volcengine/volcengine-go-sdk/service/arkruntime"

const (
	ZhiPuBaseURL   = "https://open.bigmodel.cn/api/paas/v4"
	ZhiPuModel     = "glm-4.7-flash"
	DouBaoModel    = "doubao-embedding-vision-251215"
	CollectionName = "company_knowledge"
)

var douBaoKey string
var zhiPuKey string
var douBaoClient *arkruntime.Client

func AiInit() {
	douBaoKey = "f5d13525-ce49-47ef-a26f-d42cf7cc0127"
	zhiPuKey = "43f29e6afa964c199fbde7a6d7b57794.768b8xPe3IMpGUB7"
	douBaoClient = arkruntime.NewClientWithApiKey(douBaoKey)
}
