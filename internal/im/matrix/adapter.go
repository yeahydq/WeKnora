package matrix

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yuin/goldmark"

	"github.com/Tencent/WeKnora/internal/im"
)

var (
	_ im.Adapter        = (*Adapter)(nil)
	_ im.StreamSender   = (*Adapter)(nil)
	_ im.FileDownloader = (*Adapter)(nil)
)

const (
	extraKeyRoomID     = "room_id"
	extraKeyThreadRoot = "thread_root"
	typingTimeout      = 15
)

type Adapter struct {
	client    *Client
	botUserID string
}

func NewAdapter(client *Client, botUserID string) *Adapter {
	return &Adapter{
		client:    client,
		botUserID: strings.TrimSpace(botUserID),
	}
}

func (a *Adapter) Platform() im.Platform {
	return im.PlatformMatrix
}

func (a *Adapter) HandleURLVerification(c *gin.Context) bool {
	return false
}

func (a *Adapter) VerifyCallback(c *gin.Context) error {
	return fmt.Errorf("matrix adapter does not support webhook callbacks")
}

func (a *Adapter) ParseCallback(c *gin.Context) (*im.IncomingMessage, error) {
	return nil, fmt.Errorf("matrix adapter does not support webhook callbacks")
}

func (a *Adapter) SendReply(ctx context.Context, incoming *im.IncomingMessage, reply *im.ReplyMessage) error {
	roomID := incoming.ChatID
	if roomID == "" && incoming.Extra != nil {
		roomID = incoming.Extra[extraKeyRoomID]
	}
	if roomID == "" {
		return fmt.Errorf("missing matrix room_id")
	}

	content := buildMatrixTextContent(reply.Content, false)
	if threadRoot := threadRootFromIncoming(incoming); threadRoot != "" {
		content["m.relates_to"] = map[string]any{
			"rel_type": "m.thread",
			"event_id": threadRoot,
		}
	}

	_, err := a.client.SendMessage(ctx, roomID, content)
	return err
}

type matrixStreamState struct {
	roomID     string
	eventID    string
	threadRoot string
	content    string
}

var (
	matrixStreamsMu sync.Mutex
	matrixStreams   = map[string]*matrixStreamState{}
)

func (a *Adapter) StartStream(ctx context.Context, incoming *im.IncomingMessage) (string, error) {
	roomID := incoming.ChatID
	if roomID == "" && incoming.Extra != nil {
		roomID = incoming.Extra[extraKeyRoomID]
	}
	if roomID == "" {
		return "", fmt.Errorf("missing matrix room_id")
	}

	threadRoot := threadRootFromIncoming(incoming)
	_ = a.client.SetTyping(ctx, roomID, a.botUserID, true, typingTimeout*time.Second)
	content := buildMatrixTextContent("正在思考，请稍候...", false)
	if threadRoot != "" {
		content["m.relates_to"] = map[string]any{
			"rel_type": "m.thread",
			"event_id": threadRoot,
		}
	}

	eventID, err := a.client.SendMessage(ctx, roomID, content)
	if err != nil {
		return "", fmt.Errorf("matrix start stream: %w", err)
	}

	streamID := uuid.NewString()
	matrixStreamsMu.Lock()
	matrixStreams[streamID] = &matrixStreamState{
		roomID:     roomID,
		eventID:    eventID,
		threadRoot: threadRoot,
	}
	matrixStreamsMu.Unlock()

	return streamID, nil
}

func (a *Adapter) SendStreamChunk(ctx context.Context, incoming *im.IncomingMessage, streamID string, content string) error {
	matrixStreamsMu.Lock()
	state, ok := matrixStreams[streamID]
	if ok {
		state.content += content
	}
	matrixStreamsMu.Unlock()
	if !ok {
		return fmt.Errorf("unknown matrix stream_id: %s", streamID)
	}

	return a.sendEdit(ctx, state.roomID, state.eventID, state.threadRoot, state.content)
}

func (a *Adapter) EndStream(ctx context.Context, incoming *im.IncomingMessage, streamID string) error {
	matrixStreamsMu.Lock()
	state, ok := matrixStreams[streamID]
	if ok {
		delete(matrixStreams, streamID)
	}
	matrixStreamsMu.Unlock()
	if !ok {
		return fmt.Errorf("unknown matrix stream_id: %s", streamID)
	}
	defer func() {
		_ = a.client.SetTyping(ctx, state.roomID, a.botUserID, false, 0)
	}()

	finalContent := strings.TrimSpace(state.content)
	if finalContent == "" {
		finalContent = "已完成"
	}
	if err := a.sendEdit(ctx, state.roomID, state.eventID, state.threadRoot, finalContent); err != nil {
		return err
	}
	return nil
}

func (a *Adapter) sendEdit(ctx context.Context, roomID, targetEventID, threadRoot, content string) error {
	editContent := buildMatrixTextContent("* "+content, false)
	editContent["m.new_content"] = buildMatrixTextContent(content, false)
	editContent["m.relates_to"] = map[string]any{
		"rel_type": "m.replace",
		"event_id": targetEventID,
	}
	if threadRoot != "" {
		editContent["m.relates_to"] = map[string]any{
			"rel_type": "m.replace",
			"event_id": targetEventID,
			"m.in_reply_to": map[string]any{
				"event_id": threadRoot,
			},
		}
	}
	_, err := a.client.SendMessage(ctx, roomID, editContent)
	return err
}

func (a *Adapter) DownloadFile(ctx context.Context, msg *im.IncomingMessage) (io.ReadCloser, string, error) {
	if msg.FileKey == "" {
		return nil, "", fmt.Errorf("missing matrix file URL")
	}
	reader, err := a.client.DownloadMXC(ctx, msg.FileKey)
	if err != nil {
		return nil, "", err
	}
	name := msg.FileName
	if name == "" {
		name = "matrix-file"
	}
	return reader, name, nil
}

func threadRootFromIncoming(incoming *im.IncomingMessage) string {
	if incoming == nil {
		return ""
	}
	if incoming.ThreadID != "" {
		return incoming.ThreadID
	}
	if incoming.Extra != nil {
		return incoming.Extra[extraKeyThreadRoot]
	}
	return ""
}

func buildMatrixTextContent(body string, notice bool) map[string]any {
	body = normalizeMatrixMarkdown(body)

	msgType := "m.text"
	if notice {
		msgType = "m.notice"
	}
	content := map[string]any{
		"msgtype": msgType,
		"body":    body,
	}
	if html := markdownToMatrixHTML(body); html != "" {
		content["format"] = "org.matrix.custom.html"
		content["formatted_body"] = html
	}
	return content
}

func markdownToMatrixHTML(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	var out bytes.Buffer
	if err := goldmark.Convert([]byte(body), &out); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func normalizeMatrixMarkdown(body string) string {
	replacer := strings.NewReplacer(
		"$$", "",
		`\\[`, "",
		`\\]`, "",
		`\\(`, "",
		`\\)`, "",
		`\[`, "",
		`\]`, "",
		`\(`, "",
		`\)`, "",
		`\rho`, "rho",
		`\times`, "×",
		`\cdot`, "·",
		`\frac`, "frac",
		`\left`, "",
		`\right`, "",
		`{`, "",
		`}`, "",
	)
	body = replacer.Replace(body)
	body = strings.ReplaceAll(body, "frac", "/")
	return body
}
