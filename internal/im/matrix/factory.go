package matrix

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/im"
	"github.com/Tencent/WeKnora/internal/logger"
)

func NewFactory() im.AdapterFactory {
	return func(factoryCtx context.Context, channel *im.IMChannel, msgHandler func(context.Context, *im.IncomingMessage) error) (im.Adapter, context.CancelFunc, error) {
		creds, err := im.ParseCredentials(channel.Credentials)
		if err != nil {
			return nil, nil, fmt.Errorf("parse matrix credentials: %w", err)
		}

		mode := im.ResolveMode(channel, "websocket")
		if mode != "websocket" && mode != "longpoll" {
			return nil, nil, fmt.Errorf("unsupported matrix mode: %s (only websocket/longpoll is supported)", mode)
		}

		client, err := NewClient(
			im.GetString(creds, "homeserver_url"),
			im.GetString(creds, "access_token"),
		)
		if err != nil {
			return nil, nil, err
		}

		botUserID := strings.TrimSpace(im.GetString(creds, "user_id"))
		if botUserID == "" {
			botUserID, err = client.WhoAmI(factoryCtx)
			if err != nil {
				return nil, nil, fmt.Errorf("matrix whoami: %w", err)
			}
		}

		longPollClient := NewLongConnClient(client, botUserID, msgHandler)
		pollCtx, pollCancel := context.WithCancel(context.Background())
		go func() {
			if err := longPollClient.Start(pollCtx); err != nil && pollCtx.Err() == nil {
				logger.Errorf(context.Background(), "[IM] Matrix long polling stopped for channel %s: %v", channel.ID, err)
			}
		}()

		return NewAdapter(client, botUserID), pollCancel, nil
	}
}
