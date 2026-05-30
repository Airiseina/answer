package parser

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

type PDFParser struct{}

func (p *PDFParser) SupportedExtensions() []string {
	return []string{"pdf"}
}

func (p *PDFParser) Parse(filePath string) (*ParsedDocument, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开PDF文件失败: %w", err)
	}
	defer f.Close()

	var allText strings.Builder
	var sections []Section
	pageCount := r.NumPage()

	for i := 1; i <= pageCount; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		allText.WriteString(trimmed)
		allText.WriteString("\n\n")
		sections = append(sections, Section{
			Heading:     fmt.Sprintf("第%d页", i),
			Level:       0,
			Content:     trimmed,
			PageNumber:  i,
			ContentType: "page",
		})
	}

	content := strings.TrimSpace(allText.String())
	if content == "" {
		return nil, fmt.Errorf("PDF文本内容为空，可能是扫描件(图片PDF)，请提供文本版PDF")
	}

	return &ParsedDocument{
		Title:    "",
		Content:  content,
		Sections: sections,
	}, nil
}

func ExtractPDFPageCount(filePath string) (int, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("打开PDF文件失败: %w", err)
	}
	f.Close()
	return r.NumPage(), nil
}
