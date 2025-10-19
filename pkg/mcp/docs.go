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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tmc/langchaingo/textsplitter"
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
	re := regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)
	return re.ReplaceAllString(content, "")
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
	docChunkSize    = 8 * 1024
	docChunkOverlap = 1 * 1024
)

type chunk struct {
	id      int
	name    string
	content string
}

type docsLoaderMiddleware struct {
	logger    *slog.Logger
	fsys      fs.FS
	chunkInfo []chunk
}

// newDocsLoaderMiddleware uses the provided FS to split the file into chunks
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
			chunkInfo = append(chunkInfo, chunk{
				id:      i,
				name:    fn,
				content: c,
			})
		}
	}

	docsMW.chunkInfo = chunkInfo
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
