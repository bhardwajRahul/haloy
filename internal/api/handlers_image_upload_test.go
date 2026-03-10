package api

import (
	"bytes"
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haloydev/haloy/internal/apitypes"
	"github.com/haloydev/haloy/internal/logging"
)

func newTestAPIServerForImages() *APIServer {
	return &APIServer{
		logBroker: logging.NewLogBroker(),
		logLevel:  slog.LevelInfo,
	}
}

func TestHandleImageUpload_InsufficientDiskSpaceReturns507(t *testing.T) {
	s := newTestAPIServerForImages()
	s.uploadDiskSpaceCheck = func(context.Context, int64) error {
		return &insufficientDiskSpaceError{
			Path:           "/tmp",
			RequiredBytes:  2048,
			AvailableBytes: 1024,
		}
	}

	req := newImageUploadRequest(t, "image.tar", []byte("tar-data"))
	rr := httptest.NewRecorder()

	s.handleImageUpload().ServeHTTP(rr, req)

	if rr.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInsufficientStorage)
	}
	if !strings.Contains(rr.Body.String(), "insufficient disk space") {
		t.Fatalf("body = %q, want disk space error", rr.Body.String())
	}
}

func TestHandleImageAssemble_InsufficientDiskSpaceReturns507(t *testing.T) {
	s := newTestAPIServerForImages()
	s.assembleDiskSpaceCheck = func(context.Context, apitypes.ImageAssembleRequest) error {
		return &insufficientDiskSpaceError{
			Path:           "/var/lib/docker",
			RequiredBytes:  4096,
			AvailableBytes: 1024,
		}
	}

	body := `{"imageRef":"app:123","config":"Y29uZmln","manifest":{"Config":"config.json","Layers":["sha256:abc/layer.tar"]}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/images/layers/assemble", strings.NewReader(body))
	rr := httptest.NewRecorder()

	s.handleImageAssemble().ServeHTTP(rr, req)

	if rr.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInsufficientStorage)
	}
	if !strings.Contains(rr.Body.String(), "insufficient disk space") {
		t.Fatalf("body = %q, want disk space error", rr.Body.String())
	}
}

func newImageUploadRequest(t *testing.T, filename string, contents []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(contents); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/images/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
