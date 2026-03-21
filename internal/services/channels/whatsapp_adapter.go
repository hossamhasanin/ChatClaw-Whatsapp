package channels

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"chatclaw/internal/define"
	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
	"github.com/wailsapp/wails/v3/pkg/application"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

func init() {
	// Custom name for the WhatsApp Linked Devices screen
	store.DeviceProps.Os = proto.String("ChatClaw")

	RegisterAdapter(PlatformWhatsApp, func() PlatformAdapter {
		return &WhatsAppAdapter{}
	})
}

type WhatsAppAdapter struct {
	connected atomic.Bool
	channelID int64
	handler   MessageHandler
	client    *whatsmeow.Client
}

func (a *WhatsAppAdapter) Platform() string {
	return PlatformWhatsApp
}

func (a *WhatsAppAdapter) Connect(ctx context.Context, channelID int64, configJSON string, handler MessageHandler) error {
	a.channelID = channelID
	a.handler = handler

	dbLog := waLog.Stdout("Database", "DEBUG", true)

	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get user config dir: %w", err)
	}

	dir := filepath.Join(cfgDir, define.AppID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	dbPath := filepath.Join(dir, "whatsapp_sessions.db")
	dbURI := fmt.Sprintf("file:%s?_foreign_keys=on", dbPath)

	// Using a separate sqlite file for WhatsApp sessions to prevent conflicts with the main bun DB lock
	container, err := sqlstore.New(context.Background(), "sqlite3", dbURI, dbLog)
	if err != nil {
		return fmt.Errorf("failed to connect to whatsapp database: %w", err)
	}

	// Each channel gets its own device store
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get device store: %w", err)
	}

	clientLog := waLog.Stdout("Client", "DEBUG", true)
	a.client = whatsmeow.NewClient(deviceStore, clientLog)
	a.client.AddEventHandler(a.eventHandler)

	if a.client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := a.client.GetQRChannel(context.Background())
		err = a.client.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect to WhatsApp: %w", err)
		}

		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					// Generate QR code and send to frontend
					png, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
					if err == nil {
						base64Image := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
						// Emit the Wails event
						app := application.Get()
						if app != nil {
							app.Event.Emit("channel:whatsapp:qr", map[string]interface{}{
								"channel_id": channelID,
								"qr_code":    base64Image,
							})
						}
					}
				} else {
					slog.Info("WhatsApp Login event", "event", evt.Event)
				}
			}
		}()
	} else {
		// Already logged in, just connect
		err = a.client.Connect()
		if err != nil {
			return fmt.Errorf("failed to connect to WhatsApp: %w", err)
		}
		a.connected.Store(true)
	}

	return nil
}

func (a *WhatsAppAdapter) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsFromMe {
			return
		}

		var content string
		var msgType = "text"

		// Very basic message parsing - text only for now
		if v.Message.GetConversation() != "" {
			content = v.Message.GetConversation()
		} else if v.Message.ExtendedTextMessage != nil {
			content = v.Message.ExtendedTextMessage.GetText()
		}

		if content != "" && a.handler != nil {
			msgID := v.Info.ID
			senderID := v.Info.Sender.String()

			// Chat ID helps distinguish DMs from groups
			chatID := v.Info.Chat.String()

			// Send to AI Agent
			a.handler(IncomingMessage{
				ChannelID:  a.channelID,
				Platform:   PlatformWhatsApp,
				MessageID:  msgID,
				SenderID:   senderID,
				SenderName: v.Info.PushName, // WhatsApp push name
				ChatID:     chatID,
				ChatName:   "", // Could fetch group info if needed
				IsGroup:    v.Info.IsGroup,
				Content:    content,
				MsgType:    msgType,
				RawData:    "",
			})
		}
	case *events.Connected:
		a.connected.Store(true)
		app := application.Get()
		if app != nil {
			app.Event.Emit("channel:whatsapp:connected", map[string]interface{}{
				"channel_id": a.channelID,
			})
		}
	case *events.Disconnected:
		a.connected.Store(false)
	}
}

func (a *WhatsAppAdapter) Disconnect(ctx context.Context) error {
	a.connected.Store(false)
	if a.client != nil {
		a.client.Disconnect()
	}
	return nil
}

func (a *WhatsAppAdapter) IsConnected() bool {
	return a.connected.Load()
}

func (a *WhatsAppAdapter) SendMessage(ctx context.Context, targetID string, content string) error {
	if a.client == nil || !a.client.IsConnected() {
		return fmt.Errorf("whatsapp client not connected")
	}

	// targetID here is typically the ChatID string we gave to the handler
	jid, err := types.ParseJID(targetID) // Note: need to import types as well
	if err != nil {
		// attempt to append suffix if just number
		if !strings.Contains(targetID, "@") {
			jid, err = types.ParseJID(targetID + "@s.whatsapp.net")
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	_, err = a.client.SendMessage(ctx, jid, &waE2E.Message{ // wait, need to construct message properly
		Conversation: &content,
	})
	return err
}
