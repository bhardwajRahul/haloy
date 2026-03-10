package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haloydev/haloy/internal/apitypes"
	"github.com/haloydev/haloy/internal/constants"
)

func TestHandleImageDiskSpaceCheck_ReturnsPreflightResult(t *testing.T) {
	s := newTestAPIServerForImages()
	s.imageDiskSpaceCheck = func(context.Context, apitypes.ImageDiskSpaceCheckRequest) (diskSpaceCheckResult, error) {
		return diskSpaceCheckResult{
			OK:             true,
			Path:           "/var/lib/docker",
			RequiredBytes:  1024,
			AvailableBytes: 2048,
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/images/disk-space-check", strings.NewReader(`{"uploadSizeBytes":512}`))
	rr := httptest.NewRecorder()

	s.handleImageDiskSpaceCheck().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), `"ok":true`) {
		t.Fatalf("body = %q, want ok response", rr.Body.String())
	}
}

func TestHandleImageDiskSpaceCheck_RejectsMixedModes(t *testing.T) {
	s := newTestAPIServerForImages()
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/images/disk-space-check",
		strings.NewReader(`{"uploadSizeBytes":512,"assembledImageSizeBytes":1024}`),
	)
	rr := httptest.NewRecorder()

	s.handleImageDiskSpaceCheck().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleVersion_ReportsImagePreflightCapability(t *testing.T) {
	s := newTestAPIServerForImages()
	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	rr := httptest.NewRecorder()

	s.handleVersion().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), constants.CapabilityImagePreflight) {
		t.Fatalf("body = %q, want %q capability", rr.Body.String(), constants.CapabilityImagePreflight)
	}
}
