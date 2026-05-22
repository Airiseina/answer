package handle

import (
	"answer_pkg/storage"
	"api_gateway/middleware"
	"api_gateway/response"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

const maxUploadSize = 50 * 1024 * 1024

func Upload(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, "参数错误", "请选择要上传的文件")
		return
	}
	if file.Size > maxUploadSize {
		response.Error(c, "文件过大", "文件大小不能超过50MB")
		return
	}
	src, err := file.Open()
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	defer src.Close()
	contentType := storage.GetContentType(file.Filename)
	objectName := storage.GenerateObjectName(userId, file.Filename)
	result, err := storage.UploadFile(ctx, userId, objectName, src, file.Size, contentType)
	if err != nil {
		response.Error(c, "上传失败", "请稍后重试")
		return
	}
	mediaType := storage.DetectMediaType(contentType)
	response.Success(c, map[string]interface{}{
		"url":        result.URL,
		"file_name":  result.FileName,
		"file_size":  result.FileSize,
		"media_type": mediaType,
	})
}
