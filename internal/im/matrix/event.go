package matrix

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Tencent/WeKnora/internal/im"
)

type event struct {
	Type    string         `json:"type"`
	Sender  string         `json:"sender"`
	EventID string         `json:"event_id"`
	RoomID  string         `json:"room_id"`
	Content map[string]any `json:"content"`
}

var matrixReplyFallbackRe = regexp.MustCompile(`(?s)\A(?:>.*\n)+\n`)

func parseMatrixEvent(evt *event, botUserID string) *im.IncomingMessage {
	if evt == nil || evt.Type != "m.room.message" || evt.EventID == "" || evt.RoomID == "" {
		return nil
	}
	if botUserID != "" && evt.Sender == botUserID {
		return nil
	}

	msgType, _ := evt.Content["msgtype"].(string)
	if msgType == "m.notice" {
		return nil
	}
	if relatesTo, ok := evt.Content["m.relates_to"].(map[string]any); ok {
		if relType, _ := relatesTo["rel_type"].(string); relType == "m.replace" {
			return nil
		}
	}

	msg := &im.IncomingMessage{
		Platform:  im.PlatformMatrix,
		UserID:    evt.Sender,
		UserName:  evt.Sender,
		ChatID:    evt.RoomID,
		ChatType:  im.ChatTypeGroup,
		MessageID: evt.EventID,
		Extra: map[string]string{
			extraKeyRoomID: evt.RoomID,
		},
	}

	if threadRoot := extractThreadRoot(evt.Content); threadRoot != "" {
		msg.ThreadID = threadRoot
		msg.Extra[extraKeyThreadRoot] = threadRoot
	}

	body, _ := evt.Content["body"].(string)
	switch msgType {
	case "", "m.text":
		msg.MessageType = im.MessageTypeText
		msg.Content = stripMatrixReplyFallback(body, evt.Content)
		if strings.TrimSpace(msg.Content) == "" {
			return nil
		}
	case "m.image":
		msg.MessageType = im.MessageTypeImage
		msg.Content = stripMatrixReplyFallback(body, evt.Content)
		msg.FileKey = extractFileURL(evt.Content)
		msg.FileName = extractFileName(evt.Content, body)
		msg.FileSize = extractFileSize(evt.Content)
	case "m.file":
		msg.MessageType = im.MessageTypeFile
		msg.Content = stripMatrixReplyFallback(body, evt.Content)
		msg.FileKey = extractFileURL(evt.Content)
		msg.FileName = extractFileName(evt.Content, body)
		msg.FileSize = extractFileSize(evt.Content)
	default:
		return nil
	}

	if (msg.MessageType == im.MessageTypeFile || msg.MessageType == im.MessageTypeImage) && msg.FileKey == "" {
		return nil
	}
	return msg
}

func extractReplyEventID(content map[string]any) string {
	relatesTo, ok := content["m.relates_to"].(map[string]any)
	if !ok {
		return ""
	}
	inReplyTo, ok := relatesTo["m.in_reply_to"].(map[string]any)
	if !ok {
		return ""
	}
	eventID, _ := inReplyTo["event_id"].(string)
	return eventID
}

func buildQuotedMessage(evt *event, botUserID string) *im.QuotedMessage {
	if evt == nil || evt.Type != "m.room.message" {
		return nil
	}

	msgType, _ := evt.Content["msgtype"].(string)
	body, _ := evt.Content["body"].(string)
	quote := &im.QuotedMessage{
		MessageID:    evt.EventID,
		SenderID:     evt.Sender,
		IsBotMessage: botUserID != "" && evt.Sender == botUserID,
	}

	switch msgType {
	case "", "m.text":
		quote.Content = strings.TrimSpace(stripMatrixReplyFallback(body, evt.Content))
	case "m.image":
		quote.NonTextType = "image"
	case "m.file":
		quote.NonTextType = "file"
	default:
		quote.NonTextType = msgType
	}

	if quote.Content == "" && quote.NonTextType == "" {
		quote.NonTextType = "file"
	}
	if quote.Content == "" && quote.NonTextType == "" {
		return nil
	}
	return quote
}

func extractThreadRoot(content map[string]any) string {
	relatesTo, ok := content["m.relates_to"].(map[string]any)
	if !ok {
		return ""
	}
	relType, _ := relatesTo["rel_type"].(string)
	eventID, _ := relatesTo["event_id"].(string)
	if relType == "m.thread" {
		return eventID
	}
	return ""
}

func stripMatrixReplyFallback(body string, content map[string]any) string {
	relatesTo, ok := content["m.relates_to"].(map[string]any)
	if !ok {
		return body
	}
	if _, ok := relatesTo["m.in_reply_to"].(map[string]any); !ok {
		return body
	}
	return strings.TrimSpace(matrixReplyFallbackRe.ReplaceAllString(body, ""))
}

func extractFileURL(content map[string]any) string {
	if fileURL, _ := content["url"].(string); fileURL != "" {
		return fileURL
	}
	fileObj, ok := content["file"].(map[string]any)
	if !ok {
		return ""
	}
	fileURL, _ := fileObj["url"].(string)
	return fileURL
}

func extractFileName(content map[string]any, fallback string) string {
	if name, _ := content["filename"].(string); name != "" {
		return name
	}
	if fallback != "" {
		return fallback
	}
	return "matrix-file"
}

func extractFileSize(content map[string]any) int64 {
	info, ok := content["info"].(map[string]any)
	if !ok {
		return 0
	}
	switch size := info["size"].(type) {
	case float64:
		return int64(size)
	case int64:
		return size
	case int:
		return int64(size)
	default:
		return 0
	}
}

func parseMXCURL(raw string) (serverName string, mediaID string, err error) {
	const prefix = "mxc://"
	if !strings.HasPrefix(raw, prefix) {
		return "", "", fmt.Errorf("invalid matrix media URL: %s", raw)
	}
	parts := strings.SplitN(strings.TrimPrefix(raw, prefix), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid matrix media URL: %s", raw)
	}
	return parts[0], parts[1], nil
}
