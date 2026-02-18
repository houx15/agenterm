package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	maxASRAudioBytes = 8 << 20
)

type asrTranscribeParams struct {
	AppID      string
	AccessKey  string
	SampleRate int
	AudioPCM   []byte
}

type asrTranscriber interface {
	Transcribe(ctx context.Context, params asrTranscribeParams) (string, error)
}

func (h *handler) transcribeASR(w http.ResponseWriter, r *http.Request) {
	if h.asrTranscriber == nil {
		jsonError(w, http.StatusServiceUnavailable, "asr unavailable")
		return
	}
	if err := r.ParseMultipartForm(maxASRAudioBytes); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	appID := strings.TrimSpace(r.FormValue("app_id"))
	if appID == "" {
		appID = strings.TrimSpace(os.Getenv("VOLCENGINE_APP_ID"))
	}
	accessKey := strings.TrimSpace(r.FormValue("access_key"))
	if accessKey == "" {
		accessKey = strings.TrimSpace(os.Getenv("VOLCENGINE_ACCESS_KEY"))
	}
	if appID == "" || accessKey == "" {
		jsonError(w, http.StatusBadRequest, "app_id and access_key are required")
		return
	}

	sampleRate := 16000
	if rawRate := strings.TrimSpace(r.FormValue("sample_rate")); rawRate != "" {
		parsed, err := strconv.Atoi(rawRate)
		if err != nil || parsed <= 0 {
			jsonError(w, http.StatusBadRequest, "sample_rate must be a positive integer")
			return
		}
		sampleRate = parsed
	}

	file, _, err := r.FormFile("audio")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "audio file is required")
		return
	}
	defer file.Close()

	audio, err := io.ReadAll(io.LimitReader(file, maxASRAudioBytes+1))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "failed to read audio")
		return
	}
	if len(audio) == 0 {
		jsonError(w, http.StatusBadRequest, "audio file is empty")
		return
	}
	if len(audio) > maxASRAudioBytes {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("audio file exceeds %d bytes", maxASRAudioBytes))
		return
	}

	text, err := h.asrTranscriber.Transcribe(r.Context(), asrTranscribeParams{
		AppID:      appID,
		AccessKey:  accessKey,
		SampleRate: sampleRate,
		AudioPCM:   audio,
	})
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{"text": strings.TrimSpace(text)})
}
