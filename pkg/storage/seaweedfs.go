package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/Airiseina/answer/pkg/logger"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	Client    *SeaweedFSClient
	BasePath  string
	FilerURL  string
	PublicURL string
)

type SeaweedFSClient struct {
	httpClient *http.Client
	filerURL   string
	basePath   string
}

func InitSeaweedFS(v *viper.Viper) {
	FilerURL = v.GetString("seaweedfs.filer_url")
	BasePath = v.GetString("seaweedfs.base_path")
	PublicURL = v.GetString("seaweedfs.public_url")
	if FilerURL == "" {
		FilerURL = "http://127.0.0.1:8888"
	}
	if BasePath == "" {
		BasePath = "/chat"
	}
	if PublicURL == "" {
		PublicURL = "/files"
	}
	Client = &SeaweedFSClient{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		filerURL:   FilerURL,
		basePath:   BasePath,
	}
	var lastErr error
	for i := 0; i < 10; i++ {
		if HealthCheck() {
			lastErr = nil
			break
		}
		lastErr = fmt.Errorf("filer未就绪")
		logger.Warn("SeaweedFS未就绪，3秒后重试...", zap.Int("attempt", i+1))
		time.Sleep(3 * time.Second)
	}
	if lastErr != nil {
		logger.Fatal("SeaweedFS连接超时，请确认Filer已启动")
	}
	logger.Info("SeaweedFS初始化成功", zap.String("filerURL", FilerURL), zap.String("basePath", BasePath))
}

func (s *SeaweedFSClient) PutObject(ctx context.Context, objectName string, reader io.Reader, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	uploadURL := s.filerURL + s.basePath + "/" + objectName
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(objectName))
	if err != nil {
		logger.Error("创建multipart表单失败", zap.Error(err))
		return fmt.Errorf("创建multipart表单失败: %w", err)
	}
	if _, err = io.Copy(part, reader); err != nil {
		logger.Error("写入文件数据失败", zap.Error(err))
		return fmt.Errorf("写入文件数据失败: %w", err)
	}
	writer.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		logger.Error("创建上传请求失败", zap.Error(err))
		return fmt.Errorf("创建上传请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Error("SeaweedFS上传文件失败", zap.Error(err))
		return fmt.Errorf("上传文件失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Error("SeaweedFS上传失败", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return fmt.Errorf("上传失败, status=%d", resp.StatusCode)
	}
	return nil
}

func (s *SeaweedFSClient) GetObject(ctx context.Context, objectName string) ([]byte, error) {
	downloadURL := s.filerURL + s.basePath + "/" + objectName

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建下载请求失败: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Error("SeaweedFS获取对象失败", zap.Error(err))
		return nil, fmt.Errorf("获取对象失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取对象失败, status=%d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("SeaweedFS读取对象失败", zap.Error(err))
		return nil, fmt.Errorf("读取对象失败: %w", err)
	}
	return data, nil
}

func (s *SeaweedFSClient) DeleteObject(ctx context.Context, objectName string) error {
	deleteURL := s.filerURL + s.basePath + "/" + objectName

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("创建删除请求失败: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Error("SeaweedFS删除对象失败", zap.Error(err))
		return fmt.Errorf("删除对象失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("删除对象失败, status=%d", resp.StatusCode)
	}
	return nil
}

func (s *SeaweedFSClient) PresignedGetObject(ctx context.Context, objectName string) (string, error) {
	return s.filerURL + s.basePath + "/" + objectName, nil
}

func (s *SeaweedFSClient) ObjectExists(ctx context.Context, objectName string) bool {
	url := s.filerURL + s.basePath + "/" + objectName
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type UploadResult struct {
	URL      string `json:"url"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
}

func ComputeContentHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func GenerateObjectName(contentHash string, ext string) string {
	return fmt.Sprintf("data/%s/%s%s", contentHash[:2], contentHash, ext)
}

func UploadFile(ctx context.Context, userId int64, originalName string, reader io.Reader, fileSize int64, contentType string) (*UploadResult, error) {
	if Client == nil {
		return nil, fmt.Errorf("SeaweedFS客户端未初始化")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取文件数据失败: %w", err)
	}

	contentHash := ComputeContentHash(data)
	ext := filepath.Ext(originalName)
	objectName := GenerateObjectName(contentHash, ext)

	if !Client.ObjectExists(ctx, objectName) {
		err = Client.PutObject(ctx, objectName, bytes.NewReader(data), contentType)
		if err != nil {
			return nil, err
		}
		logger.Info("文件上传成功", zap.String("hash", contentHash), zap.Int64("size", fileSize))
	} else {
		logger.Info("文件已存在，跳过上传", zap.String("hash", contentHash))
	}

	fileURL := PublicURL + BasePath + "/" + objectName
	return &UploadResult{
		URL:      fileURL,
		FileName: originalName,
		FileSize: fileSize,
	}, nil
}

func GetContentType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".aac":
		return "audio/aac"
	case ".m4a":
		return "audio/mp4"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".zip":
		return "application/zip"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

func IsImageContentType(contentType string) bool {
	return contentType == "image/jpeg" || contentType == "image/png" ||
		contentType == "image/gif" || contentType == "image/webp" ||
		contentType == "image/bmp" || contentType == "image/svg+xml"
}

func IsAudioContentType(contentType string) bool {
	return contentType == "audio/mpeg" || contentType == "audio/wav" ||
		contentType == "audio/ogg" || contentType == "audio/aac" ||
		contentType == "audio/mp4"
}

func DetectMediaType(contentType string) string {
	if IsImageContentType(contentType) {
		return "image"
	}
	if IsAudioContentType(contentType) {
		return "voice"
	}
	return "file"
}

func HealthCheck() bool {
	if Client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, Client.filerURL+"/", nil)
	if err != nil {
		return false
	}
	resp, err := Client.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func NormalizeURL(rawURL string) string {
	if strings.HasPrefix(rawURL, FilerURL) {
		return strings.Replace(rawURL, FilerURL, PublicURL, 1)
	}
	return rawURL
}

func NormalizeContentURLs(content string) string {
	if FilerURL == "" || PublicURL == "" {
		return content
	}
	return strings.Replace(content, FilerURL, PublicURL, -1)
}

var _ Storage = (*SeaweedFSClient)(nil)
