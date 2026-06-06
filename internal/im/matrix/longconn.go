package matrix

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/im"
	"github.com/Tencent/WeKnora/internal/logger"
)

type LongConnClient struct {
	client     *Client
	botUserID  string
	msgHandler func(context.Context, *im.IncomingMessage) error
}

func NewLongConnClient(client *Client, botUserID string, msgHandler func(context.Context, *im.IncomingMessage) error) *LongConnClient {
	return &LongConnClient{
		client:     client,
		botUserID:  strings.TrimSpace(botUserID),
		msgHandler: msgHandler,
	}
}

func (c *LongConnClient) Start(ctx context.Context) error {
	logger.Infof(ctx, "[IM] Matrix long polling connecting...")

	var since string
	primed := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := c.client.Sync(ctx, since, 30000)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			logger.Errorf(ctx, "[Matrix] sync error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if !primed {
			since = resp.NextBatch
			primed = true
			logger.Infof(ctx, "[IM] Matrix long polling connected successfully")
			continue
		}

		for roomID, room := range resp.Rooms.Join {
			for _, evt := range room.Timeline.Events {
				evt.RoomID = roomID
				msg := parseMatrixEvent(&evt, c.botUserID)
				if msg == nil {
					continue
				}
				if replyEventID := extractReplyEventID(evt.Content); replyEventID != "" {
					quotedEvent, err := c.client.GetEvent(ctx, roomID, replyEventID)
					if err != nil {
						logger.Warnf(ctx, "[Matrix] fetch quoted event failed: room=%s event=%s err=%v", roomID, replyEventID, err)
					} else {
						msg.Quote = buildQuotedMessage(quotedEvent, c.botUserID)
					}
				}
				if err := c.msgHandler(ctx, msg); err != nil {
					logger.Errorf(ctx, "[Matrix] Handle message error: %v", err)
				}
			}
		}

		if resp.NextBatch == "" {
			return fmt.Errorf("matrix sync returned empty next_batch")
		}
		since = resp.NextBatch
	}
}
