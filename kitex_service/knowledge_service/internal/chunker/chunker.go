package chunker

type Chunk struct {
	Content    string
	Index      int
	Source     string
	PageNumber int
	Heading    string
}

type Chunker interface {
	Chunk(text string, opts ChunkOptions) []Chunk
}

type ChunkOptions struct {
	ChunkSize    int
	ChunkOverlap int
	Source       string
	PageNumber   int
	Heading      string
}

const (
	DefaultChunkSize    = 800
	DefaultChunkOverlap = 150
)
