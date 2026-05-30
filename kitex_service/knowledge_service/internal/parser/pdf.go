package parser

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type PDFParser struct{}

func (p *PDFParser) SupportedExtensions() []string {
	return []string{"pdf"}
}

func (p *PDFParser) Parse(filePath string) (*ParsedDocument, error) {
	text, err := p.extractText(filePath)
	if err != nil {
		return nil, fmt.Errorf("PDF文本提取失败: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("PDF文本内容为空，可能是扫描件(图片PDF)，请提供文本版PDF")
	}
	sections := p.splitByPage(text)
	return &ParsedDocument{
		Title:    "",
		Content:  text,
		Sections: sections,
	}, nil
}

func (p *PDFParser) extractText(filePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", filePath, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext命令执行失败: %w, stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func (p *PDFParser) splitByPage(text string) []Section {
	pages := strings.Split(text, "\f")
	var sections []Section
	for i, page := range pages {
		trimmed := strings.TrimSpace(page)
		if trimmed == "" {
			continue
		}
		sections = append(sections, Section{
			Heading:     fmt.Sprintf("第%d页", i+1),
			Level:       0,
			Content:     trimmed,
			PageNumber:  i + 1,
			ContentType: "page",
		})
	}
	return sections
}

func ExtractPDFPageCount(filePath string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pdfinfo", filePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("pdfinfo命令执行失败: %w", err)
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			countStr := strings.TrimSpace(strings.TrimPrefix(line, "Pages:"))
			count, err := strconv.Atoi(countStr)
			if err != nil {
				continue
			}
			return count, nil
		}
	}
	return 0, fmt.Errorf("无法从pdfinfo输出中解析页数")
}
