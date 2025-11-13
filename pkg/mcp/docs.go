package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tmc/langchaingo/textsplitter"
)

var (
	stripFrontmatterRegex = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)
)

// Context key and middlewares for embedding Prometheus' docs as an fs.FS into
// a context for use with tool/resource calls. Avoids the need for
// global/external state to maintain the docs FS otherwise.
type docsKey struct{}

func addDocsToContext(ctx context.Context, docsMW *docsLoaderMiddleware) context.Context {
	return context.WithValue(ctx, docsKey{}, docsMW)
}

func getDocsFsFromContext(ctx context.Context) (*docsLoaderMiddleware, error) {
	docs, ok := ctx.Value(docsKey{}).(*docsLoaderMiddleware)
	if !ok {
		return nil, errors.New("failed to get docs FS from context")
	}

	return docs, nil
}

// stripFrontmatter removes the frontmatter block from the beginning of the
// markdown block, if present.
// The frontmatter is the beginning block that start and end with a line `---`.
func stripFrontmatter(content string) string {
	return stripFrontmatterRegex.ReplaceAllString(content, "")
}

func getDocFileNames(fsys fs.FS) ([]string, error) {
	var names []string

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.ToLower(filepath.Ext(path)) == ".md" {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk docs directory: %w", err)
	}

	return names, nil
}

func getDocFileContent(fsys fs.FS, path string) (string, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return "", fmt.Errorf("error opening file from FS: %w", err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("error reading file from FS: %w", err)
	}

	return stripFrontmatter(string(content)), nil
}

const (
	docChunkSize           = 8 * 1024
	docChunkOverlap        = 1 * 1024
	defaultDocsSearchLimit = 25
)

type chunk struct {
	Id      int
	Name    string
	Content string
}

func (c *chunk) ID() string {
	return fmt.Sprintf("%s#%d", c.Name, c.Id)
}

type docsLoaderMiddleware struct {
	logger      *slog.Logger
	fsys        fs.FS
	chunkInfo   []chunk
	searchIndex bleve.Index
}

// newDocsLoaderMiddleware uses the provided FS to split the files into chunks
// and cache them for future indexing/searching.
func newDocsLoaderMiddleware(logger *slog.Logger, fsys fs.FS) *docsLoaderMiddleware {
	chunkInfo := make([]chunk, 0)
	docsMW := docsLoaderMiddleware{
		logger: logger,
		fsys:   fsys,
	}

	splitter := textsplitter.NewMarkdownTextSplitter(
		textsplitter.WithAllowedSpecial([]string{"all"}),
		textsplitter.WithChunkOverlap(docChunkOverlap),
		textsplitter.WithChunkSize(docChunkSize),
		textsplitter.WithCodeBlocks(true),
		textsplitter.WithHeadingHierarchy(true),
		textsplitter.WithJoinTableRows(true),
		textsplitter.WithKeepSeparator(true),
		textsplitter.WithReferenceLinks(true),
	)

	docFiles, err := getDocFileNames(docsMW.fsys)
	if err != nil {
		logger.Error("Failed listing files", "err", err)
		return &docsMW
	}

	bleve.SetLog(slog.NewLogLogger(logger.Handler(), slog.LevelDebug))
	mapping := bleve.NewIndexMapping()
	searchIndex, err := bleve.NewMemOnly(mapping)
	if err != nil {
		logger.Error("Failed to make in-memory search index for docs", "err", err)
		return &docsMW
	}

	// Read, chunk, and index files for docs search.
	for _, fn := range docFiles {
		content, err := getDocFileContent(docsMW.fsys, fn)
		if err != nil {
			logger.Error("Failed reading file", "err", err)
			continue
		}

		chunks, err := splitter.SplitText(content)
		if err != nil {
			logger.Error("Failed to split markdown document into chunks", "err", err)
			continue
		}

		for i, c := range chunks {
			newChunk := chunk{
				Id:      i + 1,
				Name:    fn,
				Content: c,
			}

			chunkInfo = append(chunkInfo, newChunk)
			newChunkID := newChunk.ID()
			if err := searchIndex.Index(newChunkID, newChunk); err != nil {
				logger.Error("Failed to index chunk", "err", err, "chunk_id", newChunkID)
				continue
			}
		}
	}

	docsMW.chunkInfo = chunkInfo
	docsMW.searchIndex = searchIndex
	return &docsMW
}

func (m *docsLoaderMiddleware) ToolMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return next(addDocsToContext(ctx, m), req)
	}
}

func (m *docsLoaderMiddleware) ResourceMiddleware(next server.ResourceHandlerFunc) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return next(addDocsToContext(ctx, m), request)
	}
}

// searchDocs takes a query and does analytical fuzzy matching against the
// index, and it returns a slice of chunk IDs that matched the query. The
// caller can determine how to proceed with the information/matches.
func (m *docsLoaderMiddleware) searchDocs(q string, limit int) ([]string, error) {
	var res []string
	if limit < 1 {
		limit = defaultDocsSearchLimit
	}

	query := bleve.NewMatchQuery(q)
	query.Fuzziness = 1

	req := bleve.NewSearchRequest(query)
	req.Size = limit
	req.Highlight = bleve.NewHighlight()
	req.Fields = []string{"*"}

	searchRes, err := m.searchIndex.Search(req)
	if err != nil {
		return res, fmt.Errorf("error searching index: %w", err)
	}

	hits := searchRes.Hits
	if len(hits) > limit {
		hits = hits[:limit]
	}

	for _, hit := range hits {
		res = append(res, hit.ID)
	}

	return res, nil
}
