package mcp

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
)

// Test SHA constants for consistency across tests.
const (
	testSHANew     = "abc123def456789012345678901234567890abcd"
	testSHAOld     = "oldcommit0000000000000000000000000000000"
	testSHACurrent = "currentabc123456789012345678901234567890"
)

// createTestArchive builds a gzip+tar archive with the given files.
// The archiveRoot is the top-level directory name (e.g., "docs-main").
func createTestArchive(t *testing.T, archiveRoot string, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for path, content := range files {
		fullPath := archiveRoot + "/" + path
		hdr := &tar.Header{
			Name:     fullPath,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestExtractDocsFromArchive(t *testing.T) {
	t.Parallel()

	t.Run("extracts markdown files from docs directory", func(t *testing.T) {
		t.Parallel()

		archive := createTestArchive(t, "docs-main", map[string]string{
			"docs/querying/basics.md":      "# Querying Basics\n\nSome content.",
			"docs/alerting/overview.md":    "# Alerting\n\nAlerts info.",
			"docs/configuration/config.md": "# Configuration\n\nConfig info.",
			// Non-docs files should be excluded.
			"README.md":        "# README",
			"content/index.md": "# Index",
			"docs/image.png":   "binary-data",
		})

		memFS, err := extractDocsFromArchive(bytes.NewReader(archive))
		require.NoError(t, err)

		// Verify expected files are present.
		assertFileContent(t, memFS, "querying/basics.md", "# Querying Basics")
		assertFileContent(t, memFS, "alerting/overview.md", "# Alerting")
		assertFileContent(t, memFS, "configuration/config.md", "# Configuration")

		// Verify excluded files are not present.
		_, err = fs.ReadFile(memFS, "README.md")
		require.Error(t, err)
		_, err = fs.ReadFile(memFS, "image.png")
		require.Error(t, err)
	})

	t.Run("returns error for empty archive", func(t *testing.T) {
		t.Parallel()

		archive := createTestArchive(t, "docs-main", map[string]string{
			"README.md": "# README",
		})

		_, err := extractDocsFromArchive(bytes.NewReader(archive))
		require.Error(t, err)
		require.Contains(t, err.Error(), "no markdown files found")
	})

	t.Run("returns error for invalid gzip data", func(t *testing.T) {
		t.Parallel()

		_, err := extractDocsFromArchive(bytes.NewReader([]byte("not gzip data")))
		require.Error(t, err)
	})

	t.Run("handles different archive root names", func(t *testing.T) {
		t.Parallel()

		// GitHub uses the ref name as root, e.g., "docs-abc123/"
		archive := createTestArchive(t, "docs-abc123def", map[string]string{
			"docs/querying/basics.md": "# Basics",
		})

		memFS, err := extractDocsFromArchive(bytes.NewReader(archive))
		require.NoError(t, err)
		assertFileContent(t, memFS, "querying/basics.md", "# Basics")
	})

	t.Run("returns error for corrupt tar inside valid gzip", func(t *testing.T) {
		t.Parallel()

		// Create valid gzip but with invalid tar data inside.
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		_, err := gw.Write([]byte("this is not valid tar data"))
		require.NoError(t, err)
		require.NoError(t, gw.Close())

		_, err = extractDocsFromArchive(bytes.NewReader(buf.Bytes()))
		require.Error(t, err)
		// The error should be about tar reading failure.
	})

	t.Run("rejects archive exceeding max decompressed size", func(t *testing.T) {
		t.Parallel()

		// Create an archive with a single file whose declared size exceeds
		// the maxDecompressedSize limit (200 MB). The extraction should fail
		// before reading the file content.
		archive := createTestArchiveWithLargeFile(t, maxDecompressedSize+1)

		_, err := extractDocsFromArchive(bytes.NewReader(archive))
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds maximum decompressed size")
	})

	t.Run("rejects archive exceeding max file count", func(t *testing.T) {
		t.Parallel()

		// Create an archive with more files than the maxFileCount limit (10000).
		// We use maxFileCount+1 to just exceed the limit.
		archive := createTestArchiveWithNFiles(t, maxFileCount+1)

		_, err := extractDocsFromArchive(bytes.NewReader(archive))
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds maximum file count")
	})

	t.Run("rejects paths with directory traversal", func(t *testing.T) {
		t.Parallel()

		// Create an archive with a path traversal attempt.
		archive := createTestArchive(t, "docs-main", map[string]string{
			"docs/../../../etc/passwd":  "malicious",
			"docs/querying/basics.md":   "# Valid",
			"docs/alerting/../admin.md": "malicious", // traversal in the middle
		})

		memFS, err := extractDocsFromArchive(bytes.NewReader(archive))
		require.NoError(t, err)

		// Valid file should be present.
		assertFileContent(t, memFS, "querying/basics.md", "# Valid")

		// Malicious files should be rejected.
		_, err = fs.ReadFile(memFS, "../../../etc/passwd")
		require.Error(t, err)
		_, err = fs.ReadFile(memFS, "alerting/../admin.md")
		require.Error(t, err)
	})
}

// createTestArchiveWithLargeFile builds a gzip+tar archive containing a docs
// markdown file whose tar header declares the given size. Since
// extractDocsFromArchive checks hdr.Size before reading the file data, the
// archive doesn't need to contain the actual data -- only a valid tar header.
// The gzip stream is finalized without closing the tar writer (which would
// fail due to the incomplete entry), producing a truncated but parseable
// archive that triggers the size check.
func createTestArchiveWithLargeFile(t *testing.T, declaredSize int64) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     "docs-main/docs/large.md",
		Mode:     0o644,
		Size:     declaredSize,
		Typeflag: tar.TypeReg,
	}
	require.NoError(t, tw.WriteHeader(hdr))
	// Don't write data or close the tar writer -- the size check fires
	// before any data is read, so the truncated stream is sufficient.
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// createTestArchiveWithNFiles builds a gzip+tar archive containing n markdown
// files under the docs/ directory. Each file has minimal content.
func createTestArchiveWithNFiles(t *testing.T, n int) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("# Doc\n")
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("docs-main/docs/file%d.md", i)
		hdr := &tar.Header{
			Name:     path,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// mockRefLister implements remoteRefLister for testing.
// Note: refs and err fields must not be modified after the mock is passed to a DocsUpdater.
type mockRefLister struct {
	mu      sync.Mutex
	refs    map[string]string
	err     error
	calls   int
	blockCh chan struct{} // if set, blocks until closed
}

func (m *mockRefLister) ListRefs(ctx context.Context) (map[string]string, error) {
	m.mu.Lock()
	m.calls++
	blockCh := m.blockCh
	refs := m.refs
	err := m.err
	m.mu.Unlock()

	// Support blocking for context cancellation tests.
	if blockCh != nil {
		select {
		case <-blockCh:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}
	return refs, nil
}

func (m *mockRefLister) getCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestDocsUpdaterCheckForUpdate(t *testing.T) {
	t.Parallel()

	t.Run("detects new commit", func(t *testing.T) {
		t.Parallel()

		mock := &mockRefLister{
			refs: map[string]string{
				"refs/heads/main": testSHANew,
			},
		}

		updater := newTestUpdater(testUpdaterOpts{refLister: mock, currentHash: testSHAOld})
		sha, changed, err := updater.checkForUpdate(context.Background())
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, testSHANew, sha)
		require.Equal(t, 1, mock.getCalls())
	})

	t.Run("no change when same commit", func(t *testing.T) {
		t.Parallel()

		mock := &mockRefLister{
			refs: map[string]string{
				"refs/heads/main": testSHANew,
			},
		}

		updater := newTestUpdater(testUpdaterOpts{refLister: mock, currentHash: testSHANew})
		sha, changed, err := updater.checkForUpdate(context.Background())
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, testSHANew, sha)
	})

	t.Run("returns error when main branch not found", func(t *testing.T) {
		t.Parallel()

		mock := &mockRefLister{
			refs: map[string]string{
				"refs/heads/master": testSHANew, // no main branch
			},
		}

		updater := newTestUpdater(testUpdaterOpts{refLister: mock, currentHash: testSHAOld})
		_, _, err := updater.checkForUpdate(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "refs/heads/main not found")
	})

	t.Run("returns error when refs is empty", func(t *testing.T) {
		t.Parallel()

		mock := &mockRefLister{
			refs: map[string]string{}, // empty refs
		}

		updater := newTestUpdater(testUpdaterOpts{refLister: mock, currentHash: testSHAOld})
		_, _, err := updater.checkForUpdate(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "refs/heads/main not found")
	})

	t.Run("remote error returns wrapped error", func(t *testing.T) {
		t.Parallel()

		mock := &mockRefLister{
			err: errors.New("network error"),
		}

		updater := newTestUpdater(testUpdaterOpts{refLister: mock, currentHash: testSHANew})
		_, _, err := updater.checkForUpdate(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to query remote refs")
		require.Contains(t, err.Error(), "network error")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		blockCh := make(chan struct{})
		mock := &mockRefLister{
			refs: map[string]string{
				"refs/heads/main": testSHANew,
			},
			blockCh: blockCh,
		}

		updater := newTestUpdater(testUpdaterOpts{refLister: mock, currentHash: testSHAOld})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, _, err := updater.checkForUpdate(ctx)
			done <- err
		}()

		// Cancel context while request is in flight.
		cancel()

		err := <-done
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestDocsUpdaterFetchAndApply(t *testing.T) {
	t.Parallel()

	t.Run("successfully fetches and applies docs", func(t *testing.T) {
		t.Parallel()

		archive := createTestArchive(t, "docs-main", map[string]string{
			"docs/querying/basics.md":   "# Updated Basics\n\nNew content.",
			"docs/alerting/overview.md": "# Updated Alerting\n\nNew alerts.",
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(archive)
		}))
		defer srv.Close()

		container := newTestContainer(nil)
		// Store initial docs state.
		container.docs.Store(&docsState{fs: testDocsFS()})

		updater := newTestUpdater(testUpdaterOpts{archiveURL: srv.URL, currentHash: "old-hash", container: container})
		err := updater.fetchAndUpdateDocsFS(context.Background())
		require.NoError(t, err)

		// Verify the docs were updated.
		ds := container.getDocsState()
		require.NotNil(t, ds)
		require.NotNil(t, ds.fs)
		require.NotNil(t, ds.searchIndex)

		// Verify the new content is available.
		content, err := container.GetDocFileContent("querying/basics.md")
		require.NoError(t, err)
		require.Contains(t, content, "Updated Basics")
	})

	t.Run("handles archive download failure", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		container := newTestContainer(nil)
		container.docs.Store(&docsState{fs: testDocsFS()})

		updater := newTestUpdater(testUpdaterOpts{archiveURL: srv.URL, currentHash: "old-hash", container: container})
		err := updater.fetchAndUpdateDocsFS(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "status 500")

		// Original docs should still be accessible.
		names, err := container.GetDocFileNames()
		require.NoError(t, err)
		require.NotEmpty(t, names)
	})

	t.Run("handles invalid archive content", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not a valid archive"))
		}))
		defer srv.Close()

		container := newTestContainer(nil)
		container.docs.Store(&docsState{fs: testDocsFS()})

		updater := newTestUpdater(testUpdaterOpts{archiveURL: srv.URL, currentHash: "old-hash", container: container})
		err := updater.fetchAndUpdateDocsFS(context.Background())
		require.Error(t, err)
	})
}

func TestDocsUpdaterAtomicSwap(t *testing.T) {
	t.Parallel()

	t.Run("swap replaces docs state atomically", func(t *testing.T) {
		t.Parallel()

		container := newTestContainer(nil)

		// Set initial state.
		initialFS := fstest.MapFS{
			"old.md": &fstest.MapFile{Data: []byte("# Old")},
		}
		initialState, err := buildDocsState(slog.Default(), initialFS)
		require.NoError(t, err)
		container.docs.Store(initialState)

		// Verify initial state.
		content, err := container.GetDocFileContent("old.md")
		require.NoError(t, err)
		require.Contains(t, content, "Old")

		// Swap to new state.
		newFS := fstest.MapFS{
			"new.md": &fstest.MapFile{Data: []byte("# New")},
		}
		newState, err := buildDocsState(slog.Default(), newFS)
		require.NoError(t, err)
		container.swapDocsState(newState)

		// Verify new state.
		content, err = container.GetDocFileContent("new.md")
		require.NoError(t, err)
		require.Contains(t, content, "New")

		// Old file should not exist.
		_, err = container.GetDocFileContent("old.md")
		require.Error(t, err)
	})

	t.Run("concurrent reads during swap", func(t *testing.T) {
		t.Parallel()

		container := newTestContainer(nil)

		// Set initial state.
		initialFS := fstest.MapFS{
			"test.md": &fstest.MapFile{Data: []byte("# Initial")},
		}
		initialState, err := buildDocsState(slog.Default(), initialFS)
		require.NoError(t, err)
		container.docs.Store(initialState)

		// Start multiple concurrent readers.
		const numReaders = 10
		var wg sync.WaitGroup
		wg.Add(numReaders)

		readErrors := make(chan error, numReaders)
		for i := 0; i < numReaders; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					_, err := container.GetDocFileNames()
					if err != nil {
						readErrors <- err
						return
					}
					time.Sleep(time.Microsecond)
				}
			}()
		}

		// Perform swaps while readers are running.
		for i := 0; i < 10; i++ {
			newFS := fstest.MapFS{
				"test.md": &fstest.MapFile{Data: []byte("# Updated")},
			}
			newState, err := buildDocsState(slog.Default(), newFS)
			require.NoError(t, err)
			container.swapDocsState(newState)
			time.Sleep(time.Millisecond)
		}

		wg.Wait()
		close(readErrors)

		// No errors should have occurred during reads.
		for err := range readErrors {
			require.NoError(t, err)
		}
	})
}

// Helper functions

func assertFileContent(t *testing.T, fsys fs.FS, path string, expectedContent string) {
	t.Helper()
	content, err := fs.ReadFile(fsys, path)
	require.NoError(t, err, "failed to read file %s", path)
	require.Contains(t, string(content), expectedContent)
}

// testUpdaterOpts configures a DocsUpdater for testing.
type testUpdaterOpts struct {
	refLister   remoteRefLister
	archiveURL  string
	currentHash string
	container   *ServerContainer
}

// newTestUpdater creates a DocsUpdater configured for testing with the given options.
func newTestUpdater(opts testUpdaterOpts) *DocsUpdater {
	container := opts.container
	if container == nil {
		container = newTestContainer(nil)
	}

	refLister := opts.refLister
	if refLister == nil {
		refLister = &mockRefLister{}
	}

	archiveURL := opts.archiveURL
	if archiveURL == "" {
		archiveURL = docsArchiveURL
	}

	return &DocsUpdater{
		logger:      slog.Default(),
		container:   container,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		currentHash: opts.currentHash,
		repoURL:     docsRepoURL,
		archiveURL:  archiveURL,
		refLister:   refLister,
	}
}

func testDocsFS() fs.FS {
	return fstest.MapFS{
		"test.md": &fstest.MapFile{Data: []byte("# Test\n\nTest content.")},
	}
}
