package parser

import (
	"fmt"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

type MarkdownParser struct{}

func (p *MarkdownParser) SupportedExtensions() []string {
	return []string{"md", "markdown"}
}

func (p *MarkdownParser) Parse(filePath string) (*ParsedDocument, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取Markdown文件失败: %w", err)
	}
	source := data
	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)
	title := p.extractTitle(doc, source)
	sections := p.walkAST(doc, source)
	var fullContent strings.Builder
	for _, sec := range sections {
		fullContent.WriteString(sec.Content)
		fullContent.WriteString("\n\n")
	}
	return &ParsedDocument{
		Title:    title,
		Content:  strings.TrimSpace(fullContent.String()),
		Sections: sections,
	}, nil
}

func nodeText(n ast.Node, source []byte) string {
	var buf strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		line := n.Lines().At(i)
		buf.Write(line.Value(source))
	}
	return buf.String()
}

func (p *MarkdownParser) extractTitle(doc ast.Node, source []byte) string {
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		if heading, ok := child.(*ast.Heading); ok && heading.Level == 1 {
			return strings.TrimSpace(nodeText(heading, source))
		}
	}
	return ""
}

func (p *MarkdownParser) walkAST(doc ast.Node, source []byte) []Section {
	var sections []Section
	var current *Section
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		heading, isHeading := child.(*ast.Heading)
		if isHeading && heading.Level <= 4 {
			if current != nil && strings.TrimSpace(current.Content) != "" {
				sections = append(sections, *current)
			}
			headingText := strings.TrimSpace(nodeText(heading, source))
			current = &Section{
				Heading:     headingText,
				Level:       heading.Level,
				Content:     fmt.Sprintf("%s %s\n", strings.Repeat("#", heading.Level), headingText),
				ContentType: "heading",
			}
		} else {
			content := strings.TrimSpace(nodeText(child, source))
			if content == "" {
				continue
			}
			richContent := p.renderNode(child, source)
			if current == nil {
				current = &Section{
					Heading:     "",
					Level:       0,
					Content:     richContent,
					ContentType: "content",
				}
			} else {
				current.Content += richContent + "\n"
			}
		}
	}
	if current != nil && strings.TrimSpace(current.Content) != "" {
		sections = append(sections, *current)
	}
	return sections
}

func (p *MarkdownParser) renderNode(node ast.Node, source []byte) string {
	switch node.Kind() {
	case ast.KindCodeBlock:
		code := nodeText(node, source)
		return fmt.Sprintf("```\n%s\n```", code)
	case ast.KindFencedCodeBlock:
		var lang string
		fenced := node.(*ast.FencedCodeBlock)
		if fenced.Info != nil {
			lang = nodeText(fenced.Info, source)
		}
		code := nodeText(node, source)
		return fmt.Sprintf("```%s\n%s\n```", lang, code)
	case east.KindTable:
		return p.renderTable(node, source)
	case ast.KindBlockquote:
		lines := nodeText(node, source)
		var quoted strings.Builder
		for _, line := range strings.Split(lines, "\n") {
			quoted.WriteString("> ")
			quoted.WriteString(line)
			quoted.WriteString("\n")
		}
		return strings.TrimSpace(quoted.String())
	default:
		return nodeText(node, source)
	}
}

func (p *MarkdownParser) renderTable(node ast.Node, source []byte) string {
	var buf strings.Builder
	for row := node.FirstChild(); row != nil; row = row.NextSibling() {
		if row.Kind() != east.KindTableRow && row.Kind() != east.KindTableHeader {
			continue
		}
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			if cell.Kind() == east.KindTableCell {
				cells = append(cells, strings.TrimSpace(nodeText(cell, source)))
			}
		}
		buf.WriteString("| ")
		buf.WriteString(strings.Join(cells, " | "))
		buf.WriteString(" |\n")
		if row.Kind() == east.KindTableHeader {
			buf.WriteString("| ")
			for i := 0; i < len(cells); i++ {
				buf.WriteString("---")
				if i < len(cells)-1 {
					buf.WriteString(" | ")
				}
			}
			buf.WriteString(" |\n")
		}
	}
	return strings.TrimSpace(buf.String())
}
