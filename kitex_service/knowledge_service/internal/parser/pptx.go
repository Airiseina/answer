package parser

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type PptxParser struct{}

func (p *PptxParser) SupportedExtensions() []string {
	return []string{"ppt", "pptx"}
}

func (p *PptxParser) Parse(filePath string) (*ParsedDocument, error) {
	rc, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("解析PPTX文件失败: %w", err)
	}
	defer rc.Close()

	slideTexts := p.extractAllSlideTexts(&rc.Reader)
	title := ""
	if len(slideTexts) > 0 && len(slideTexts[0]) > 0 {
		title = slideTexts[0][0]
	}

	var sections []Section
	for i, texts := range slideTexts {
		content := strings.Join(texts, "\n")
		content = strings.TrimSpace(content)
		if content != "" {
			sections = append(sections, Section{
				Heading:     fmt.Sprintf("幻灯片 %d", i+1),
				Level:       2,
				Content:     fmt.Sprintf("## 幻灯片 %d\n%s\n", i+1, content),
				PageNumber:  i + 1,
				ContentType: "slide",
			})
		}
	}

	var fullContent string
	for _, sec := range sections {
		fullContent += sec.Content + "\n\n"
	}
	return &ParsedDocument{
		Title:    title,
		Content:  fullContent,
		Sections: sections,
	}, nil
}

func (p *PptxParser) extractAllSlideTexts(zr *zip.Reader) [][]string {
	type slideEntry struct {
		idx  int
		name string
	}
	var slides []slideEntry
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			numStr := strings.TrimPrefix(f.Name, "ppt/slides/slide")
			numStr = strings.TrimSuffix(numStr, ".xml")
			num, err := strconv.Atoi(numStr)
			if err != nil {
				continue
			}
			slides = append(slides, slideEntry{idx: num, name: f.Name})
		}
	}
	sort.Slice(slides, func(i, j int) bool {
		return slides[i].idx < slides[j].idx
	})

	fileMap := make(map[string]*zip.File)
	for _, f := range zr.File {
		fileMap[f.Name] = f
	}

	var result [][]string
	for _, se := range slides {
		f, ok := fileMap[se.name]
		if !ok {
			continue
		}
		texts := p.extractTextsFromSlideZip(f)
		result = append(result, texts)
	}
	return result
}

func (p *PptxParser) extractTextsFromSlideZip(f *zip.File) []string {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}
	return p.extractTextsFromXML(data)
}

func (p *PptxParser) extractTextsFromXML(data []byte) []string {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var texts []string
	var currentText strings.Builder
	inT := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inT = true
				currentText.Reset()
			}
		case xml.CharData:
			if inT {
				currentText.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "t" && inT {
				text := strings.TrimSpace(currentText.String())
				if text != "" {
					texts = append(texts, text)
				}
				inT = false
			}
		}
	}
	return texts
}
