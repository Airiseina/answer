package parser

type ParsedDocument struct {
	Title    string
	Content  string
	Sections []Section
}

type Section struct {
	Heading     string
	Level       int
	Content     string
	PageNumber  int
	ContentType string
}

type Parser interface {
	Parse(filePath string) (*ParsedDocument, error)
	SupportedExtensions() []string
}

func GetParser(fileType string) Parser {
	switch fileType {
	case "pdf":
		return &PDFParser{}
	case "md", "markdown":
		return &MarkdownParser{}
	case "doc", "docx":
		return &DocxParser{}
	case "ppt", "pptx":
		return &PptxParser{}
	default:
		return nil
	}
}
