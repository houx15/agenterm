package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"nhooyr.io/websocket"
)

const (
	volcASRURL        = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel"
	volcASRResourceID = "volc.bigasr.sauc.duration"

	protocolVersionV1 = 0x1
	headerWords       = 0x1

	msgTypeClientFull  = 0x1
	msgTypeClientAudio = 0x2
	msgTypeServerFull  = 0x9
	msgTypeServerError = 0xF

	flagPosSequence     = 0x1
	flagNegWithSequence = 0x3

	serializationJSON = 0x1
	compressionGzip   = 0x1
)

type volcASRTranscriber struct {
	dial func(ctx context.Context, url string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error)
}

func newVolcASRTranscriber() *volcASRTranscriber {
	return &volcASRTranscriber{dial: websocket.Dial}
}

type volcASRResponse struct {
	Code          int
	IsLastPackage bool
	Payload       map[string]any
}

func (v *volcASRTranscriber) Transcribe(ctx context.Context, params asrTranscribeParams) (string, error) {
	if len(params.AudioPCM) == 0 {
		return "", fmt.Errorf("audio data is required")
	}
	if params.SampleRate <= 0 {
		params.SampleRate = 16000
	}

	reqID, err := randomRequestID()
	if err != nil {
		return "", fmt.Errorf("generate request id: %w", err)
	}

	headers := http.Header{}
	headers.Set("X-Api-Resource-Id", volcASRResourceID)
	headers.Set("X-Api-Request-Id", reqID)
	headers.Set("X-Api-Access-Key", params.AccessKey)
	headers.Set("X-Api-App-Key", params.AppID)

	conn, _, err := v.dial(ctx, volcASRURL, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		return "", fmt.Errorf("connect volc asr: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	seq := int32(1)

	initPacket, err := buildFullClientRequest(seq, params.SampleRate)
	if err != nil {
		return "", err
	}
	if err := conn.Write(ctx, websocket.MessageBinary, initPacket); err != nil {
		return "", fmt.Errorf("send init packet: %w", err)
	}
	seq++

	initResp, err := readVolcASRBinary(ctx, conn)
	if err != nil {
		return "", err
	}
	if initResp.Code != 0 {
		return "", fmt.Errorf("asr init failed: %v", initResp.Payload)
	}

	audioPacket, err := buildAudioOnlyRequest(seq, params.AudioPCM, false)
	if err != nil {
		return "", err
	}
	if err := conn.Write(ctx, websocket.MessageBinary, audioPacket); err != nil {
		return "", fmt.Errorf("send audio packet: %w", err)
	}
	seq++

	endPacket, err := buildAudioOnlyRequest(seq, nil, true)
	if err != nil {
		return "", err
	}
	if err := conn.Write(ctx, websocket.MessageBinary, endPacket); err != nil {
		return "", fmt.Errorf("send final packet: %w", err)
	}

	var latestText string
	var finalText string

	for {
		resp, err := readVolcASRBinary(ctx, conn)
		if err != nil {
			return "", err
		}
		if resp.Code != 0 {
			return "", fmt.Errorf("asr error: %v", resp.Payload)
		}

		if text, isFinal, ok := extractASRText(resp.Payload); ok {
			latestText = text
			if isFinal {
				finalText = text
			}
		}

		if resp.IsLastPackage {
			break
		}
	}

	if finalText != "" {
		return finalText, nil
	}
	return latestText, nil
}

func buildHeader(messageType byte, flags byte) []byte {
	return []byte{
		byte((protocolVersionV1 << 4) | headerWords),
		byte((messageType << 4) | flags),
		byte((serializationJSON << 4) | compressionGzip),
		0x00,
	}
}

func buildFullClientRequest(seq int32, sampleRate int) ([]byte, error) {
	payload := map[string]any{
		"user": map[string]any{"uid": "agenterm-user"},
		"audio": map[string]any{
			"format":  "pcm",
			"codec":   "raw",
			"rate":    sampleRate,
			"bits":    16,
			"channel": 1,
		},
		"request": map[string]any{
			"model_name":       "bigmodel",
			"enable_itn":       true,
			"enable_punc":      true,
			"enable_ddc":       true,
			"show_utterances":  true,
			"enable_nonstream": false,
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal init payload: %w", err)
	}
	compressed, err := gzipBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("compress init payload: %w", err)
	}

	packet := bytes.NewBuffer(make([]byte, 0, 4+4+4+len(compressed)))
	packet.Write(buildHeader(msgTypeClientFull, flagPosSequence))
	_ = binary.Write(packet, binary.BigEndian, seq)
	_ = binary.Write(packet, binary.BigEndian, uint32(len(compressed)))
	packet.Write(compressed)
	return packet.Bytes(), nil
}

func buildAudioOnlyRequest(seq int32, pcm []byte, isLast bool) ([]byte, error) {
	flags := byte(flagPosSequence)
	writeSeq := seq
	if isLast {
		flags = flagNegWithSequence
		writeSeq = -seq
	}

	compressed, err := gzipBytes(pcm)
	if err != nil {
		return nil, fmt.Errorf("compress audio payload: %w", err)
	}

	packet := bytes.NewBuffer(make([]byte, 0, 4+4+4+len(compressed)))
	packet.Write(buildHeader(msgTypeClientAudio, flags))
	_ = binary.Write(packet, binary.BigEndian, writeSeq)
	_ = binary.Write(packet, binary.BigEndian, uint32(len(compressed)))
	packet.Write(compressed)
	return packet.Bytes(), nil
}

func readVolcASRBinary(ctx context.Context, conn *websocket.Conn) (volcASRResponse, error) {
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return volcASRResponse{}, fmt.Errorf("read asr response: %w", err)
		}
		if typ != websocket.MessageBinary {
			continue
		}
		resp, err := parseVolcASRResponse(data)
		if err != nil {
			return volcASRResponse{}, err
		}
		return resp, nil
	}
}

func parseVolcASRResponse(msg []byte) (volcASRResponse, error) {
	if len(msg) < 4 {
		return volcASRResponse{}, fmt.Errorf("invalid asr response header")
	}
	headerWords := int(msg[0] & 0x0F)
	headerBytes := headerWords * 4
	if headerWords <= 0 || len(msg) < headerBytes {
		return volcASRResponse{}, fmt.Errorf("invalid asr response header size")
	}

	messageType := msg[1] >> 4
	flags := msg[1] & 0x0F
	serialization := msg[2] >> 4
	compression := msg[2] & 0x0F

	payload := msg[headerBytes:]
	resp := volcASRResponse{}

	if flags&0x01 != 0 {
		if len(payload) < 4 {
			return volcASRResponse{}, fmt.Errorf("invalid asr response sequence")
		}
		payload = payload[4:]
	}
	if flags&0x02 != 0 {
		resp.IsLastPackage = true
	}
	if flags&0x04 != 0 {
		if len(payload) < 4 {
			return volcASRResponse{}, fmt.Errorf("invalid asr response event")
		}
		payload = payload[4:]
	}

	switch messageType {
	case msgTypeServerFull:
		if len(payload) < 4 {
			return volcASRResponse{}, fmt.Errorf("invalid asr full response payload")
		}
		size := int(binary.BigEndian.Uint32(payload[:4]))
		payload = payload[4:]
		if len(payload) < size {
			return volcASRResponse{}, fmt.Errorf("invalid asr full response body")
		}
		payload = payload[:size]
	case msgTypeServerError:
		if len(payload) < 8 {
			return volcASRResponse{}, fmt.Errorf("invalid asr error response payload")
		}
		resp.Code = int(int32(binary.BigEndian.Uint32(payload[:4])))
		size := int(binary.BigEndian.Uint32(payload[4:8]))
		payload = payload[8:]
		if len(payload) < size {
			return volcASRResponse{}, fmt.Errorf("invalid asr error response body")
		}
		payload = payload[:size]
	default:
		return volcASRResponse{}, fmt.Errorf("unsupported asr response type: %d", messageType)
	}

	if compression == compressionGzip {
		decoded, err := ungzipBytes(payload)
		if err != nil {
			return volcASRResponse{}, fmt.Errorf("decompress asr payload: %w", err)
		}
		payload = decoded
	}

	if len(payload) == 0 {
		return resp, nil
	}

	if serialization == serializationJSON {
		var body map[string]any
		if err := json.Unmarshal(payload, &body); err != nil {
			return volcASRResponse{}, fmt.Errorf("decode asr json: %w", err)
		}
		resp.Payload = body
	}
	return resp, nil
}

func extractASRText(payload map[string]any) (text string, isFinal bool, ok bool) {
	if payload == nil {
		return "", false, false
	}
	result, ok := payload["result"].(map[string]any)
	if !ok {
		return "", false, false
	}
	textValue, ok := result["text"].(string)
	if !ok {
		return "", false, false
	}
	finalValue, _ := result["is_final"].(bool)
	return textValue, finalValue, true
}

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(data); err != nil {
		_ = gzw.Close()
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ungzipBytes(data []byte) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	return io.ReadAll(gzr)
}

func randomRequestID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	now := make([]byte, 8)
	binary.BigEndian.PutUint64(now, uint64(time.Now().UnixNano()))
	return hex.EncodeToString(raw) + "-" + hex.EncodeToString(now), nil
}
