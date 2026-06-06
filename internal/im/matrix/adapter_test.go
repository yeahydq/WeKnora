package matrix

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/im"
)

func TestParseMatrixEvent_TextReplyFallback(t *testing.T) {
	evt := &event{
		Type:    "m.room.message",
		Sender:  "@alice:example.com",
		EventID: "$evt1",
		RoomID:  "!room:example.com",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "> <@bob:example.com> old message\n\nhello matrix",
			"m.relates_to": map[string]any{
				"m.in_reply_to": map[string]any{
					"event_id": "$old",
				},
			},
		},
	}

	msg := parseMatrixEvent(evt, "@bot:example.com")
	if msg == nil {
		t.Fatal("expected message")
	}
	if msg.Platform != im.PlatformMatrix {
		t.Fatalf("unexpected platform: %s", msg.Platform)
	}
	if msg.Content != "hello matrix" {
		t.Fatalf("unexpected content: %q", msg.Content)
	}
}

func TestBuildQuotedMessage_Text(t *testing.T) {
	evt := &event{
		Type:    "m.room.message",
		Sender:  "@bot:example.com",
		EventID: "$evtq1",
		RoomID:  "!room:example.com",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "quoted text",
		},
	}

	quote := buildQuotedMessage(evt, "@bot:example.com")
	if quote == nil {
		t.Fatal("expected quote")
	}
	if quote.Content != "quoted text" {
		t.Fatalf("unexpected quote content: %q", quote.Content)
	}
	if !quote.IsBotMessage {
		t.Fatal("expected bot quote")
	}
}

func TestBuildQuotedMessage_Image(t *testing.T) {
	evt := &event{
		Type:    "m.room.message",
		Sender:  "@alice:example.com",
		EventID: "$evtq2",
		RoomID:  "!room:example.com",
		Content: map[string]any{
			"msgtype": "m.image",
			"body":    "photo.jpg",
			"url":     "mxc://example.com/abc",
		},
	}

	quote := buildQuotedMessage(evt, "@bot:example.com")
	if quote == nil {
		t.Fatal("expected quote")
	}
	if quote.NonTextType != "image" {
		t.Fatalf("unexpected non_text_type: %q", quote.NonTextType)
	}
}

func TestParseMatrixEvent_FileMessage(t *testing.T) {
	evt := &event{
		Type:    "m.room.message",
		Sender:  "@alice:example.com",
		EventID: "$evt2",
		RoomID:  "!room:example.com",
		Content: map[string]any{
			"msgtype":  "m.file",
			"body":     "report.pdf",
			"url":      "mxc://example.com/abc123",
			"filename": "report.pdf",
			"info": map[string]any{
				"size": float64(42),
			},
		},
	}

	msg := parseMatrixEvent(evt, "")
	if msg == nil {
		t.Fatal("expected file message")
	}
	if msg.MessageType != im.MessageTypeFile {
		t.Fatalf("unexpected type: %s", msg.MessageType)
	}
	if msg.FileKey != "mxc://example.com/abc123" {
		t.Fatalf("unexpected file key: %q", msg.FileKey)
	}
	if msg.FileName != "report.pdf" {
		t.Fatalf("unexpected file name: %q", msg.FileName)
	}
	if msg.FileSize != 42 {
		t.Fatalf("unexpected file size: %d", msg.FileSize)
	}
}

func TestParseMXCURL(t *testing.T) {
	serverName, mediaID, err := parseMXCURL("mxc://example.com/xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serverName != "example.com" || mediaID != "xyz" {
		t.Fatalf("unexpected parts: %s %s", serverName, mediaID)
	}
}

func TestBuildMatrixTextContent_AddsFormattedBody(t *testing.T) {
	content := buildMatrixTextContent("**加粗**\n\n- 列表项", false)

	if got := content["msgtype"]; got != "m.text" {
		t.Fatalf("unexpected msgtype: %#v", got)
	}
	if got := content["format"]; got != "org.matrix.custom.html" {
		t.Fatalf("unexpected format: %#v", got)
	}
	html, ok := content["formatted_body"].(string)
	if !ok || html == "" {
		t.Fatalf("expected formatted_body, got %#v", content["formatted_body"])
	}
	if !(contains(html, "<strong>加粗</strong>") && contains(html, "<li>列表项</li>")) {
		t.Fatalf("unexpected formatted_body: %q", html)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}

func TestDownloadMXC_FallsBackToLegacyMediaEndpoint(t *testing.T) {
	var clientV1Hits int
	var mediaV3Hits int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_matrix/client/v1/media/download/example.com/abc123":
			clientV1Hits++
			http.Error(w, `{"errcode":"M_NOT_FOUND","error":"Not found"}`, http.StatusNotFound)
		case "/_matrix/media/v3/download/example.com/abc123":
			mediaV3Hits++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("image-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL, "test-token")
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	reader, err := client.DownloadMXC(context.Background(), "mxc://example.com/abc123")
	if err != nil {
		t.Fatalf("DownloadMXC error: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("unexpected body: %q", string(data))
	}
	if clientV1Hits != 1 {
		t.Fatalf("expected client v1 endpoint hit once, got %d", clientV1Hits)
	}
	if mediaV3Hits != 1 {
		t.Fatalf("expected media v3 endpoint hit once, got %d", mediaV3Hits)
	}
}
