package ai

import (
	"os"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
)

const (
	DouBaoModel = "doubao-embedding-vision-251215"
)

var douBaoKey string
var douBaoClient *arkruntime.Client

func AiInit() {
	douBaoKey = os.Getenv("KNOWLEDGE_DOUBAO_EMBEDDING_API_KEY")
	douBaoClient = arkruntime.NewClientWithApiKey(douBaoKey)
}
