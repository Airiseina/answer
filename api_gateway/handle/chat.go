package handle

//type UploadController struct {
//	Storage storage.Storage
//	Mq      mq.Mq
//}
//
//func NewUploadController(s storage.Storage, m mq.Mq) *UploadController {
//	return &UploadController{s, m}
//}
//
//type mqMessage struct {
//	SessionId  uint   `json:"session_id"`
//	ObjectName string `json:"object_name"`
//}
//
//func (controller *UploadController) UploadFile(ctx context.Context, c *app.RequestContext) {
//	sessionIdStr := c.Query("session_id")
//	if sessionIdStr == "" {
//		response.Error(c, "参数缺失", "请重新输入参数")
//		return
//	}
//	sessionId, err := strconv.ParseUint(sessionIdStr, 10, 64)
//	if err != nil {
//		response.Error(c, "参数格式错误", "请重新输入参数")
//		return
//	}
//	Identity, _ := c.Get(middleware.IdentityKey)
//	userInfo := Identity.(*middleware.Resp)
//	id := userInfo.Id
//	file, err := c.FormFile("file")
//	if err != nil {
//		response.Error(c, "文件缺失", "请重新上传文件")
//		return
//	}
//	const MaxUploadSize = 10 * 1024 * 1024
//	if file.Size > MaxUploadSize {
//		response.Error(c, "文件超过限制", "请将文件限制在10mb以内")
//		return
//	}
//	ext := filepath.Ext(file.Filename)
//	if ext != ".txt" && ext != ".md" {
//		response.Error(c, "格式错误", "请上传对应的文件格式")
//		return
//	}
//	fileInfo, err := file.Open()
//	if err != nil {
//		response.Error(c, "文件缺失", "请重新上传文件")
//		return
//	}
//	defer fileInfo.Close()
//	objectName := fmt.Sprintf("file:%d:%s:%s", id, uuid.New().String(), ext)
//	err = controller.Storage.PutObject(ctx, storage.BucketName, objectName, fileInfo, file.Size)
//	if err != nil {
//		response.Error(c, "系统繁忙", "请稍后再试")
//		return
//	}
//	//数据库创建标文件以及修改状态为排队
//	err = controller.Mq.WriteMessage(ctx, mqMessage{
//		SessionId:  uint(sessionId),
//		ObjectName: objectName,
//	})
//	if err != nil {
//		response.Error(c, "系统繁忙", "请稍后再试")
//		return
//	}
//	response.Success(c, "上传成功")
//}
