package api

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeASRTranscriber struct {
	text string
	err  error
}

func (f *fakeASRTranscriber) Transcribe(_ context.Context, _ asrTranscribeParams) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.text, nil
}

func newASRMultipartRequest(t *testing.T, appID, accessKey string, audio []byte, sampleRate string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if appID != "" {
		if err := writer.WriteField("app_id", appID); err != nil {
			t.Fatalf("write app_id: %v", err)
		}
	}
	if accessKey != "" {
		if err := writer.WriteField("access_key", accessKey); err != nil {
			t.Fatalf("write access_key: %v", err)
		}
	}
	if sampleRate != "" {
		if err := writer.WriteField("sample_rate", sampleRate); err != nil {
			t.Fatalf("write sample_rate: %v", err)
		}
	}

	if audio != nil {
		part, err := writer.CreateFormFile("audio", "audio.pcm")
		if err != nil {
			t.Fatalf("create audio part: %v", err)
		}
		if _, err := io.Copy(part, bytes.NewReader(audio)); err != nil {
			t.Fatalf("write audio: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/asr/transcribe", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestTranscribeASRSuccess(t *testing.T) {
	h := &handler{asrTranscriber: &fakeASRTranscriber{text: "hello world"}}
	req := newASRMultipartRequest(t, "app-id", "access-key", []byte{1, 2, 3}, "16000")
	rr := httptest.NewRecorder()

	h.transcribeASR(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "hello world") {
		t.Fatalf("response missing text: %s", rr.Body.String())
	}
}

func TestTranscribeASRValidation(t *testing.T) {
	h := &handler{asrTranscriber: &fakeASRTranscriber{text: "ok"}}

	t.Run("missing credentials", func(t *testing.T) {
		req := newASRMultipartRequest(t, "", "", []byte{1}, "")
		rr := httptest.NewRecorder()
		h.transcribeASR(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("missing audio", func(t *testing.T) {
		req := newASRMultipartRequest(t, "a", "b", nil, "")
		rr := httptest.NewRecorder()
		h.transcribeASR(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid sample rate", func(t *testing.T) {
		req := newASRMultipartRequest(t, "a", "b", []byte{1, 2}, "abc")
		rr := httptest.NewRecorder()
		h.transcribeASR(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
	})
}
