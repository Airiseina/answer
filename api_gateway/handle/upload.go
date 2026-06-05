package handle

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/pkg/storage"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
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
	defer func() { _ = src.Close() }()
	contentType := storage.GetContentType(file.Filename)
	result, err := storage.UploadFile(ctx, userId, file.Filename, src, file.Size, contentType)
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

func FileProxy(ctx context.Context, c *app.RequestContext) {
	filepath := c.Param("filepath")
	if filepath == "" || filepath == "/" {
		c.SetStatusCode(http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(filepath, "/") {
		filepath = "/" + filepath
	}
	targetURL := storage.FilerURL + filepath
	hlog.CtxInfof(ctx, "FileProxy: %s -> %s", c.Request.URI().String(), targetURL)

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		hlog.CtxErrorf(ctx, "FileProxy创建请求失败: url=%s, err=%v", targetURL, err)
		c.SetStatusCode(http.StatusBadGateway)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		hlog.CtxErrorf(ctx, "FileProxy请求SeaweedFS失败: url=%s, err=%v", targetURL, err)
		c.SetStatusCode(http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		hlog.CtxErrorf(ctx, "FileProxy读取响应体失败: url=%s, err=%v", targetURL, err)
		c.SetStatusCode(http.StatusInternalServerError)
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(resp.StatusCode, contentType, body)
}
