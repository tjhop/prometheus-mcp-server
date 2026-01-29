package mcp

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	stripFrontmatterRegex = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)
)

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
		return "", fmt.Errorf("failed to open file from FS: %w", err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read file from FS: %w", err)
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
