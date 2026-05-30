package chunker

import (
	"strings"
	"unicode/utf8"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/parser"
)

type StructuralChunker struct{}

func NewStructuralChunker() *StructuralChunker {
	return &StructuralChunker{}
}

func (c *StructuralChunker) ChunkFromSections(sections []parser.Section, opts ChunkOptions) []Chunk {
	var chunks []Chunk
	chunkIndex := 0
	for _, sec := range sections {
		content := strings.TrimSpace(sec.Content)
		if content == "" {
			continue
		}
		if utf8.RuneCountInString(content) <= opts.ChunkSize {
			chunks = append(chunks, Chunk{
				Content:    content,
				Index:      chunkIndex,
				Source:     opts.Source,
				PageNumber: sec.PageNumber,
				Heading:    sec.Heading,
			})
			chunkIndex++
		} else {
			subChunks := c.splitLongSection(sec, opts, chunkIndex)
			chunks = append(chunks, subChunks...)
			chunkIndex += len(subChunks)
		}
	}
	return chunks
}

func (c *StructuralChunker) Chunk(text string, opts ChunkOptions) []Chunk {
	return []Chunk{{
		Content:    text,
		Index:      0,
		Source:     opts.Source,
		PageNumber: opts.PageNumber,
		Heading:    opts.Heading,
	}}
}

func (c *StructuralChunker) splitLongSection(sec parser.Section, opts ChunkOptions, startIndex int) []Chunk {
	content := sec.Content
	if sec.Heading != "" {
		prefix := strings.Repeat("#", sec.Level) + " " + sec.Heading + "\n"
		if strings.HasPrefix(content, prefix) {
			content = strings.TrimPrefix(content, prefix)
		}
	}
	recursiveChunker := NewRecursiveChunker(WithChunkSize(opts.ChunkSize), WithChunkOverlap(opts.ChunkOverlap))
	subChunks := recursiveChunker.Chunk(content, opts)
	for i := range subChunks {
		if sec.Heading != "" {
			subChunks[i].Content = strings.Repeat("#", sec.Level) + " " + sec.Heading + "\n" + subChunks[i].Content
		}
		subChunks[i].Index = startIndex + i
		subChunks[i].PageNumber = sec.PageNumber
		subChunks[i].Heading = sec.Heading
	}
	return subChunks
}
