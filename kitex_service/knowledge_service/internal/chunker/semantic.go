package chunker

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/Airiseina/answer/pkg/ai"
)

type SemanticChunker struct {
	similarityThreshold float64
	bufferSize          int
}

type SemanticChunkerOption func(*SemanticChunker)

func WithSimilarityThreshold(threshold float64) SemanticChunkerOption {
	return func(s *SemanticChunker) {
		s.similarityThreshold = threshold
	}
}

func WithBufferSize(size int) SemanticChunkerOption {
	return func(s *SemanticChunker) {
		s.bufferSize = size
	}
}

func NewSemanticChunker(opts ...SemanticChunkerOption) *SemanticChunker {
	s := &SemanticChunker{
		similarityThreshold: 0.5,
		bufferSize:          1,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *SemanticChunker) Chunk(text string, opts ChunkOptions) []Chunk {
	return s.ChunkWithEmbedding(context.Background(), text, opts, nil)
}

func (s *SemanticChunker) ChunkWithEmbedding(ctx context.Context, text string, opts ChunkOptions, embedFunc func(ctx context.Context, text string) ([]float32, error)) []Chunk {
	sentences := s.splitSentences(text)
	if len(sentences) <= 1 {
		return []Chunk{{
			Content:    text,
			Index:      0,
			Source:     opts.Source,
			PageNumber: opts.PageNumber,
			Heading:    opts.Heading,
		}}
	}
	if embedFunc == nil {
		embedFunc = func(ctx context.Context, t string) ([]float32, error) {
			return ai.GetEmbedding(ctx, t)
		}
	}
	embeddings, err := s.computeEmbeddings(ctx, sentences, embedFunc)
	if err != nil {
		recursiveChunker := NewRecursiveChunker()
		return recursiveChunker.Chunk(text, opts)
	}
	similarities := s.computeSimilarities(embeddings)
	breakpoints := s.findBreakpoints(similarities)
	return s.createChunks(sentences, breakpoints, opts)
}

func (s *SemanticChunker) splitSentences(text string) []string {
	var sentences []string
	var current []rune
	runes := []rune(text)
	for i, r := range runes {
		current = append(current, r)
		if r == '。' || r == '.' || r == '！' || r == '!' || r == '？' || r == '?' || r == '\n' {
			if i+1 < len(runes) && runes[i+1] == '\n' {
				current = append(current, '\n')
			}
			trimmed := strings.TrimSpace(string(current))
			if trimmed != "" {
				sentences = append(sentences, trimmed)
			}
			current = current[:0]
		}
	}
	if len(current) > 0 {
		trimmed := strings.TrimSpace(string(current))
		if trimmed != "" {
			sentences = append(sentences, trimmed)
		}
	}
	return sentences
}

func (s *SemanticChunker) computeEmbeddings(ctx context.Context, sentences []string, embedFunc func(ctx context.Context, text string) ([]float32, error)) ([][]float32, error) {
	embeddings := make([][]float32, 0, len(sentences))
	batchSize := 16
	for i := 0; i < len(sentences); i += batchSize {
		end := i + batchSize
		if end > len(sentences) {
			end = len(sentences)
		}
		batch := sentences[i:end]
		vectors, err := ai.GetEmbeddings(ctx, batch)
		if err != nil {
			return nil, err
		}
		embeddings = append(embeddings, vectors...)
	}
	return embeddings, nil
}

func (s *SemanticChunker) computeSimilarities(embeddings [][]float32) []float64 {
	similarities := make([]float64, len(embeddings)-1)
	for i := 0; i < len(embeddings)-1; i++ {
		similarities[i] = cosineSimilarity(embeddings[i], embeddings[i+1])
	}
	return similarities
}

func (s *SemanticChunker) findBreakpoints(similarities []float64) []int {
	if len(similarities) == 0 {
		return nil
	}
	mean := 0.0
	for _, sim := range similarities {
		mean += sim
	}
	mean /= float64(len(similarities))
	stdDev := 0.0
	for _, sim := range similarities {
		diff := sim - mean
		stdDev += diff * diff
	}
	stdDev = math.Sqrt(stdDev / float64(len(similarities)))
	threshold := mean - stdDev
	var breakpoints []int
	for i, sim := range similarities {
		if sim < threshold {
			breakpoints = append(breakpoints, i+1)
		}
	}
	return breakpoints
}

func (s *SemanticChunker) createChunks(sentences []string, breakpoints []int, opts ChunkOptions) []Chunk {
	if len(breakpoints) == 0 {
		return []Chunk{{
			Content:    strings.Join(sentences, " "),
			Index:      0,
			Source:     opts.Source,
			PageNumber: opts.PageNumber,
			Heading:    opts.Heading,
		}}
	}
	var chunks []Chunk
	start := 0
	chunkIndex := 0
	breakpoints = append(breakpoints, len(sentences))
	for _, bp := range breakpoints {
		if bp <= start {
			continue
		}
		content := strings.Join(sentences[start:bp], " ")
		if utf8.RuneCountInString(content) > DefaultChunkSize*2 {
			recursiveChunker := NewRecursiveChunker()
			subOpts := opts
			subChunks := recursiveChunker.Chunk(content, subOpts)
			for j := range subChunks {
				subChunks[j].Index = chunkIndex
				chunkIndex++
			}
			chunks = append(chunks, subChunks...)
		} else {
			chunks = append(chunks, Chunk{
				Content:    content,
				Index:      chunkIndex,
				Source:     opts.Source,
				PageNumber: opts.PageNumber,
				Heading:    opts.Heading,
			})
			chunkIndex++
		}
		start = bp
	}
	return chunks
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
