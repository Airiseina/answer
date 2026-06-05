package parser

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type DocxParser struct{}

func (p *DocxParser) SupportedExtensions() []string {
	return []string{"doc", "docx"}
}

func (p *DocxParser) Parse(filePath string) (*ParsedDocument, error) {
	rc, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("解析DOCX文件失败: %w", err)
	}
	defer func() { _ = rc.Close() }()

	paragraphs := p.extractParagraphs(&rc.Reader)
	title := ""
	for _, para := range paragraphs {
		if para.isHeading && para.level == 1 {
			title = para.text
			break
		}
	}
	if title == "" {
		for _, para := range paragraphs {
			if para.text != "" {
				title = para.text
				break
			}
		}
	}

	sections := p.buildSections(paragraphs)
	var fullContent string
	for _, sec := range sections {
		fullContent += sec.Content + "\n\n"
	}
	return &ParsedDocument{
		Title:    title,
		Content:  strings.TrimSpace(fullContent),
		Sections: sections,
	}, nil
}

type docxParagraph struct {
	text      string
	isHeading bool
	level     int
	style     string
}

func (p *DocxParser) extractParagraphs(zr *zip.Reader) []docxParagraph {
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			defer func() { _ = rc.Close() }()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil
			}
			return p.parseDocumentXML(data)
		}
	}
	return nil
}

func (p *DocxParser) parseDocumentXML(data []byte) []docxParagraph {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var paragraphs []docxParagraph
	var currentPara docxParagraph
	var currentText strings.Builder
	inParagraph := false
	inRun := false
	inText := false
	inPPr := false
	var styleVal string

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
			switch t.Name.Local {
			case "p":
				inParagraph = true
				currentPara = docxParagraph{}
				currentText.Reset()
				styleVal = ""
			case "pPr":
				inPPr = true
			case "pStyle":
				if inPPr {
					for _, attr := range t.Attr {
						if attr.Name.Local == "val" {
							styleVal = attr.Value
						}
					}
				}
			case "r":
				inRun = true
			case "t":
				if inRun {
					inText = true
				}
			}
		case xml.CharData:
			if inText {
				currentText.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if inParagraph {
					currentPara.text = strings.TrimSpace(currentText.String())
					currentPara.style = styleVal
					currentPara.isHeading = strings.HasPrefix(strings.ToLower(styleVal), "heading")
					if currentPara.isHeading {
						levelStr := strings.TrimPrefix(strings.ToLower(styleVal), "heading")
						levelStr = strings.TrimSpace(levelStr)
						var level int
						if _, err := fmt.Sscanf(levelStr, "%d", &level); err == nil && level > 0 {
							if level > 4 {
								level = 4
							}
							currentPara.level = level
						} else {
							currentPara.level = 2
						}
					}
					paragraphs = append(paragraphs, currentPara)
				}
				inParagraph = false
			case "pPr":
				inPPr = false
			case "r":
				inRun = false
			case "t":
				inText = false
			}
		}
	}
	return paragraphs
}

func (p *DocxParser) buildSections(paragraphs []docxParagraph) []Section {
	var sections []Section
	var current *Section
	for _, para := range paragraphs {
		if para.text == "" {
			continue
		}
		if para.isHeading {
			if current != nil && strings.TrimSpace(current.Content) != "" {
				sections = append(sections, *current)
			}
			current = &Section{
				Heading:     para.text,
				Level:       para.level,
				Content:     fmt.Sprintf("%s %s\n", strings.Repeat("#", para.level), para.text),
				ContentType: "heading",
			}
		} else {
			if current == nil {
				current = &Section{
					Heading:     "",
					Level:       0,
					Content:     para.text + "\n",
					ContentType: "content",
				}
			} else {
				current.Content += para.text + "\n"
			}
		}
	}
	if current != nil && strings.TrimSpace(current.Content) != "" {
		sections = append(sections, *current)
	}
	return sections
}
