// Copyright The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	ID      int
	Name    string
	Content string
}

func (c *chunk) String() string {
	return fmt.Sprintf("%s#%d", c.Name, c.ID)
}
