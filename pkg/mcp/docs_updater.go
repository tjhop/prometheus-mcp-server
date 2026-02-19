package mcp

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing/fstest"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/prometheus/client_golang/prometheus"
	promconfig "github.com/prometheus/common/config"

	"github.com/tjhop/prometheus-mcp-server/internal/metrics"
	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

const (
	// DocsUpdateInterval is the default interval between documentation update checks.
	DocsUpdateInterval = 24 * time.Hour

	docsRepoURL    = "https://github.com/prometheus/docs.git"
	docsArchiveURL = "https://github.com/prometheus/docs/archive/refs/heads/main.tar.gz"

	// maxArchiveSize is the maximum size of the downloaded compressed archive (50 MB).
	maxArchiveSize = 50 * 1024 * 1024

	// maxDecompressedSize is the maximum total size of extracted content (200 MB).
	// This prevents decompression bombs where a small compressed archive expands
	// to an unexpectedly large size.
	maxDecompressedSize = 200 * 1024 * 1024

	// maxFileCount is the maximum number of files to extract from the archive.
	// This prevents file count bombs where an archive contains many small files.
	maxFileCount = 10000

	// httpClientTimeout is the timeout for HTTP requests to download archives.
	httpClientTimeout = 300 * time.Second
)

var (
	metricDocsUpdateTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prometheus.BuildFQName(metrics.MetricNamespace, "docs", "last_update_timestamp_seconds"),
		Help: "Unix timestamp of the last successful documentation update.",
	})

	metricDocsUpdateFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(metrics.MetricNamespace, "docs", "update_failures_total"),
		Help: "Total number of documentation update failures.",
	})
)

func init() {
	metrics.Registry.MustRegister(
		metricDocsUpdateTimestamp,
		metricDocsUpdateFailures,
	)
}

// remoteRefLister abstracts the ability to list remote Git references.
// This interface enables testing without network access.
type remoteRefLister interface {
	// ListRefs returns the remote references for the configured repository.
	// It should return a map of reference names to their SHA hashes.
	ListRefs(ctx context.Context) (map[string]string, error)
}

// gitRemoteLister implements remoteRefLister using go-git.
type gitRemoteLister struct {
	repoURL string
}

// ListRefs queries the remote repository and returns all references.
func (g *gitRemoteLister) ListRefs(ctx context.Context) (map[string]string, error) {
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{g.repoURL},
	})

	refs, err := rem.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	result := make(map[string]string, len(refs))
	for _, ref := range refs {
		result[ref.Name().String()] = ref.Hash().String()
	}
	return result, nil
}

// DocsUpdater periodically checks the official prometheus/docs repository for
// changes and updates the embedded documentation at runtime. It uses go-git
// to query remote refs (similar to `git ls-remote`) for efficient change
// detection without rate limits, and downloads the repository archive to
// extract docs into an in-memory fs.FS.
type DocsUpdater struct {
	mu          sync.Mutex
	logger      *slog.Logger
	container   *ServerContainer
	httpClient  *http.Client
	currentHash string          // Last known commit hash (starts as embedded).
	repoURL     string          // Git repository URL for checking latest commit.
	archiveURL  string          // GitHub archive URL for downloading docs tarball.
	refLister   remoteRefLister // Interface for listing remote refs (for testability).
}

// NewDocsUpdater creates a new DocsUpdater that can check for documentation
// updates and apply them to the given ServerContainer.
// embeddedHash is the git commit hash of the docs submodule embedded at build time.
func NewDocsUpdater(logger *slog.Logger, container *ServerContainer, embeddedHash string) *DocsUpdater {
	return &DocsUpdater{
		logger:    logger.With("component", "docs_updater"),
		container: container,
		httpClient: &http.Client{
			Timeout:   httpClientTimeout,
			Transport: promconfig.NewUserAgentRoundTripper(version.UserAgent(), http.DefaultTransport),
		},
		currentHash: embeddedHash,
		repoURL:     docsRepoURL,
		archiveURL:  docsArchiveURL,
		refLister:   &gitRemoteLister{repoURL: docsRepoURL},
	}
}

// Update performs a single documentation update check and conditionally applies
// changes if a newer version is available upstream. It is context-cancellable
// and safe to call concurrently from multiple goroutines.
//
// Returns nil if docs are already up to date or if the update succeeded.
// Returns an error if the update check or application failed.
func (u *DocsUpdater) Update(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	newCommit, changed, err := u.checkForUpdate(ctx)
	if err != nil {
		metricDocsUpdateFailures.Inc()
		return fmt.Errorf("docs update check failed: %w", err)
	}
	if !changed {
		return nil
	}

	oldCommit := u.currentHash
	if err := u.fetchAndUpdateDocsFS(ctx); err != nil {
		metricDocsUpdateFailures.Inc()
		return fmt.Errorf("failed to update docs to new commit <%s>: %w", newCommit, err)
	}

	u.currentHash = newCommit
	metricDocsUpdateTimestamp.SetToCurrentTime()
	u.logger.Info("docs updated successfully", "old_commit", oldCommit, "new_commit", newCommit)
	return nil
}

// checkForUpdate queries the configured ref lister to get the latest commit hash.
// Returns the latest commit SHA, whether it differs from the current hash, and any error.
func (u *DocsUpdater) checkForUpdate(ctx context.Context) (string, bool, error) {
	u.logger.Debug("checking for documentation updates", "repo_url", u.repoURL)

	// List remote references using the configured ref lister.
	refs, err := u.refLister.ListRefs(ctx)
	if err != nil {
		return "", false, fmt.Errorf("failed to query remote refs: %w", err)
	}

	// Find refs/heads/main and get its hash.
	mainBranch := "main"
	mainRefName := plumbing.NewBranchReferenceName(mainBranch).String()
	sha, ok := refs[mainRefName]
	if !ok {
		return "", false, fmt.Errorf("refs/heads/%s not found in remote repository", mainBranch)
	}

	changed := sha != u.currentHash
	if changed {
		u.logger.Debug("new documentation commit found", "current", u.currentHash, "new", sha)
	}
	return sha, changed, nil
}

// fetchAndUpdateDocsFS downloads the docs archive from GitHub, extracts markdown
// files into an in-memory filesystem, builds a new search index, and
// atomically swaps the documentation state in the ServerContainer.
func (u *DocsUpdater) fetchAndUpdateDocsFS(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.archiveURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for archive: %w", err)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read a portion of the error response body for debugging context.
		archiveDownloadErrMsg := "archive download returned status " + strconv.Itoa(resp.StatusCode)

		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if len(errBody) > 0 {
			archiveDownloadErrMsg = fmt.Sprintf("%s: %s", archiveDownloadErrMsg, string(errBody))
		}
		return errors.New(archiveDownloadErrMsg)
	}

	// Limit the download size to prevent unbounded memory usage.
	memFS, err := extractDocsFromArchive(io.LimitReader(resp.Body, maxArchiveSize))
	if err != nil {
		return fmt.Errorf("failed to extract docs from archive: %w", err)
	}

	newState, err := buildDocsState(u.logger, memFS)
	if err != nil {
		return fmt.Errorf("failed to build docs state: %w", err)
	}

	u.container.swapDocsState(newState)
	return nil
}

// extractDocsFromArchive decompresses a gzip+tar archive and extracts markdown
// files under the docs/ directory into an in-memory fs.FS. The archive root
// directory (e.g., "docs-main/") is stripped, and only files matching
// docs/**/*.md are included.
//
// Security: The function validates against path traversal attacks and enforces
// a maximum decompressed size to prevent decompression bombs.
func extractDocsFromArchive(r io.Reader) (fs.FS, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	memFS := make(fstest.MapFS)
	var totalExtracted int64

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header entry: %w", err)
		}

		// Archive root is "docs-{ref}/" -- strip it and look for "docs/" prefix.
		// e.g., "docs-main/docs/alerting/alerting-rules.md" -> "alerting/alerting-rules.md"
		parts := strings.SplitN(hdr.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		relPath := parts[1]

		if !strings.HasPrefix(relPath, "docs/") {
			continue
		}
		docPath := strings.TrimPrefix(relPath, "docs/")
		if docPath == "" {
			continue
		}

		// Reject non-local paths for security (ie, path traversals, etc).
		if !filepath.IsLocal(docPath) {
			continue
		}

		// Only process regular files; skip directories, symlinks, etc.
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(docPath), ".md") {
			continue
		}

		// Guard against negative sizes (malformed tar entry).
		if hdr.Size < 0 {
			continue
		}
		// Guard against int64 overflow in comparison by reframing as
		// subtraction.
		if hdr.Size > maxDecompressedSize-totalExtracted {
			return nil, fmt.Errorf("archive exceeds maximum decompressed size: extracted %d bytes, next file (%s, %d bytes) would exceed limit of %d bytes", totalExtracted, hdr.Name, hdr.Size, maxDecompressedSize)
		}
		if len(memFS) >= maxFileCount {
			return nil, fmt.Errorf("archive exceeds maximum file count of %d", maxFileCount)
		}

		data, err := io.ReadAll(io.LimitReader(tr, hdr.Size))
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry %s: %w", hdr.Name, err)
		}
		totalExtracted += int64(len(data))
		memFS[docPath] = &fstest.MapFile{Data: data}
	}

	if len(memFS) == 0 {
		return nil, errors.New("no markdown files found in archive")
	}
	return memFS, nil
}
