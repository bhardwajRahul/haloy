package api

import (
	"net/http"

	"github.com/haloydev/haloy/internal/apitypes"
)

func (s *APIServer) handleImageDiskSpaceCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req apitypes.ImageDiskSpaceCheckRequest
		if err := decodeJSON(r.Body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fullUploadMode := req.UploadSizeBytes > 0
		layeredMode := req.AssembledImageSizeBytes > 0

		if fullUploadMode == layeredMode {
			http.Error(w, "exactly one of uploadSizeBytes or assembledImageSizeBytes is required", http.StatusBadRequest)
			return
		}

		if fullUploadMode && req.LayerUploadBytes > 0 {
			http.Error(w, "layerUploadBytes is only valid with assembledImageSizeBytes", http.StatusBadRequest)
			return
		}

		var (
			result diskSpaceCheckResult
			err    error
		)

		if s.imageDiskSpaceCheck != nil {
			result, err = s.imageDiskSpaceCheck(r.Context(), req)
		} else if fullUploadMode {
			result, err = checkDiskSpaceForUpload(r.Context(), osDiskSpaceProbe{}, req.UploadSizeBytes)
		} else {
			result, err = checkDiskSpaceForLayeredUpload(r.Context(), osDiskSpaceProbe{}, req.LayerUploadBytes, req.AssembledImageSizeBytes)
		}
		if err != nil {
			writeImageHandlerError(w, "Failed disk space preflight", err)
			return
		}

		resp := apitypes.ImageDiskSpaceCheckResponse{
			OK:             result.OK,
			Path:           result.Path,
			RequiredBytes:  result.RequiredBytes,
			AvailableBytes: result.AvailableBytes,
		}

		if err := encodeJSON(w, http.StatusOK, resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}
