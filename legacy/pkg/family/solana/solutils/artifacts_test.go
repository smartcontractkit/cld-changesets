package solutils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
)

func TestDownloadProgramArtifacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupClient func(*testing.T) *http.Client
		wantFiles   []string
		wantErr     string
	}{
		{
			name: "successful download and extraction",
			setupClient: func(t *testing.T) *http.Client {
				t.Helper()

				return newTestHTTPClient(func(_ *http.Request) (*http.Response, error) {
					testFiles := map[string]string{
						"program1.so": "fake program 1 content",
						"program2.so": "fake program 2 content",
						"config.json": `{"test": "config"}`,
					}

					return gzipTarballResponse(t, testFiles), nil
				})
			},
			wantFiles: []string{"program1.so", "program2.so", "config.json"},
		},
		{
			name: "server returns 404",
			setupClient: func(t *testing.T) *http.Client {
				t.Helper()
				return newTestHTTPClient(func(_ *http.Request) (*http.Response, error) {
					return httpResponse(http.StatusNotFound, nil), nil
				})
			},
			wantErr: "download failed with status 404",
		},
		{
			name: "server returns 500",
			setupClient: func(t *testing.T) *http.Client {
				t.Helper()
				return newTestHTTPClient(func(_ *http.Request) (*http.Response, error) {
					return httpResponse(http.StatusInternalServerError, nil), nil
				})
			},
			wantErr: "download failed with status 500",
		},
		{
			name: "invalid gzip content",
			setupClient: func(t *testing.T) *http.Client {
				t.Helper()
				return newTestHTTPClient(func(_ *http.Request) (*http.Response, error) {
					return httpResponse(http.StatusOK, []byte("invalid gzip content")), nil
				})
			},
			wantErr: "gzip",
		},
		{
			name: "empty tar archive",
			setupClient: func(t *testing.T) *http.Client {
				t.Helper()
				return newTestHTTPClient(func(_ *http.Request) (*http.Response, error) {
					return gzipTarballResponse(t, map[string]string{}), nil
				})
			},
			wantFiles: []string{}, // No files expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			client := tt.setupClient(t)

			// Create temporary directory for extraction
			tempDir := t.TempDir()

			// Execute
			err := downloadProgramArtifactsWithClient(
				t.Context(), "https://example.test/artifacts.tar.gz", tempDir, logger.Test(t), client,
			)

			// Assert
			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)

				// Check that expected files were created
				for _, expectedFile := range tt.wantFiles {
					filePath := filepath.Join(tempDir, expectedFile)
					assert.FileExists(t, filePath, "Expected file %s to exist", expectedFile)

					// Verify file is not empty (except for empty archive test)
					if len(tt.wantFiles) > 0 {
						info, err := os.Stat(filePath)
						require.NoError(t, err)
						assert.Positive(t, info.Size(), "File %s should not be empty", expectedFile)
					}
				}

				// Check that no unexpected files were created
				entries, err := os.ReadDir(tempDir)
				require.NoError(t, err)
				assert.Len(t, entries, len(tt.wantFiles), "Unexpected number of files extracted")
			}
		})
	}
}

func TestDownloadProgramArtifacts_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create a context that gets cancelled immediately
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	client := newTestHTTPClient(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})

	err := downloadProgramArtifactsWithClient(
		ctx,
		"https://example.test/artifacts.tar.gz",
		t.TempDir(),
		logger.Test(t),
		client,
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "context canceled")
}

func TestDownloadProgramArtifacts_InvalidURL(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	err := downloadProgramArtifacts(t.Context(), "://bad-url", tempDir, logger.Test(t))
	require.Error(t, err)
}

func TestDownloadProgramArtifacts_NonExistentTargetDir(t *testing.T) {
	t.Parallel()

	// Use a non-existent directory path
	nonExistentDir := filepath.Join(t.TempDir(), "missing-parent", "target")
	client := newTestHTTPClient(func(_ *http.Request) (*http.Response, error) {
		return gzipTarballResponse(t, map[string]string{"test.so": "test content"}), nil
	})

	err := downloadProgramArtifactsWithClient(
		t.Context(),
		"https://example.test/artifacts.tar.gz",
		nonExistentDir,
		logger.Test(t),
		client,
	)
	require.NoError(t, err) // Should succeed because MkdirAll creates parent directories

	// Verify the file was created
	assert.FileExists(t, filepath.Join(nonExistentDir, "test.so"))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func gzipTarballResponse(t *testing.T, files map[string]string) *http.Response {
	t.Helper()

	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)

	for filename, content := range files {
		header := &tar.Header{
			Name:     filename,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}

		err := tarWriter.WriteHeader(header)
		require.NoError(t, err)

		_, err = tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	return httpResponse(http.StatusOK, archive.Bytes())
}

func httpResponse(statusCode int, body []byte) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}
