package until

// ChunkText 文本切片函数
// text: 原始长文本
// chunkSize: 每个切片的最大字数（建议 500 - 800）
// overlap: 切片之间的重叠字数（建议 100 - 200）
func ChunkText(text string, chunkSize, overlap int) []string {
	if chunkSize <= overlap {
		chunkSize = overlap + 1 // 防止死循环
	}
	// 核心：转为 rune 数组，完美处理中英文混合，按“字”切分而不是按“字节”
	runes := []rune(text)
	totalLen := len(runes)
	var chunks []string
	if totalLen == 0 {
		return chunks
	}
	step := chunkSize - overlap
	for i := 0; i < totalLen; {
		end := i + chunkSize
		if end > totalLen {
			end = totalLen // 防止越界
		}
		// 截取这一段并存入数组
		chunks = append(chunks, string(runes[i:end]))
		// 如果已经切到了最后，退出循环
		if end == totalLen {
			break
		}
		// 滑动窗���向前走 (步长 = 块大小 - 重叠部分)
		i += step
	}
	return chunks
}
