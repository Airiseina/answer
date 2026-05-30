package chunker

import (
	"strings"
	"unicode/utf8"
)

type RecursiveChunker struct {
	chunkSize    int
	chunkOverlap int
	separators   []string
}

type RecursiveChunkerOption func(*RecursiveChunker)

func WithChunkSize(size int) RecursiveChunkerOption {
	return func(r *RecursiveChunker) {
		r.chunkSize = size
	}
}

func WithChunkOverlap(overlap int) RecursiveChunkerOption {
	return func(r *RecursiveChunker) {
		r.chunkOverlap = overlap
	}
}

func NewRecursiveChunker(opts ...RecursiveChunkerOption) *RecursiveChunker {
	r := &RecursiveChunker{
		chunkSize:    DefaultChunkSize,
		chunkOverlap: DefaultChunkOverlap,
		separators:   []string{"\n\n", "\n", "。", ".", "！", "!", "？", "?", "；", ";", " ", ""},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *RecursiveChunker) Chunk(text string, opts ChunkOptions) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if utf8.RuneCountInString(text) <= r.chunkSize {
		return []Chunk{{
			Content:    text,
			Index:      0,
			Source:     opts.Source,
			PageNumber: opts.PageNumber,
			Heading:    opts.Heading,
		}}
	}
	splits := r.splitText(text)
	return r.mergeSplits(splits, opts)
}

func (r *RecursiveChunker) splitText(text string) []string {
	return r.recursiveSplit(text, 0)
}

func (r *RecursiveChunker) recursiveSplit(text string, sepIdx int) []string {
	if utf8.RuneCountInString(text) <= r.chunkSize {
		return []string{text}
	}
	if sepIdx >= len(r.separators) {
		return r.splitByChars(text)
	}
	sep := r.separators[sepIdx]
	if sep == "" {
		return r.splitByChars(text)
	}
	parts := strings.Split(text, sep)
	var result []string
	var current strings.Builder
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if utf8.RuneCountInString(part) > r.chunkSize {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			subSplits := r.recursiveSplit(part, sepIdx+1)
			result = append(result, subSplits...)
		} else if current.Len() == 0 {
			current.WriteString(part)
		} else if utf8.RuneCountInString(current.String())+utf8.RuneCountInString(part)+utf8.RuneCountInString(sep) > r.chunkSize {
			result = append(result, current.String())
			current.Reset()
			current.WriteString(part)
		} else {
			current.WriteString(sep)
			current.WriteString(part)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func (r *RecursiveChunker) splitByChars(text string) []string {
	runes := []rune(text)
	var result []string
	for i := 0; i < len(runes); i += r.chunkSize - r.chunkOverlap {
		end := i + r.chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		if chunk != "" {
			result = append(result, chunk)
		}
		if end >= len(runes) {
			break
		}
	}
	return result
}

func (r *RecursiveChunker) mergeSplits(splits []string, opts ChunkOptions) []Chunk {
	var chunks []Chunk
	var current strings.Builder
	index := 0
	for _, split := range splits {
		split = strings.TrimSpace(split)
		if split == "" {
			continue
		}
		if current.Len() > 0 && utf8.RuneCountInString(current.String())+utf8.RuneCountInString(split)+1 > r.chunkSize {
			chunks = append(chunks, Chunk{
				Content:    strings.TrimSpace(current.String()),
				Index:      index,
				Source:     opts.Source,
				PageNumber: opts.PageNumber,
				Heading:    opts.Heading,
			})
			index++
			overlapText := r.getOverlapText(current.String())
			current.Reset()
			current.WriteString(overlapText)
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(split)
	}
	if current.Len() > 0 {
		chunks = append(chunks, Chunk{
			Content:    strings.TrimSpace(current.String()),
			Index:      index,
			Source:     opts.Source,
			PageNumber: opts.PageNumber,
			Heading:    opts.Heading,
		})
	}
	return chunks
}

func (r *RecursiveChunker) getOverlapText(text string) string {
	runes := []rune(text)
	if len(runes) <= r.chunkOverlap {
		return text
	}
	return string(runes[len(runes)-r.chunkOverlap:])
}
