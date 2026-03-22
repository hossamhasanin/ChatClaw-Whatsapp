package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"chatclaw/internal/eino/tools"
	"chatclaw/internal/services/channels"
	"chatclaw/internal/services/i18n"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func selectDesc(eng, zh string) string {
	if i18n.GetLocale() == i18n.LocaleZhCN {
		return zh
	}
	return eng
}

// WhatsAppSenderConfig configures the whatsapp_sender tool.
type WhatsAppSenderConfig struct {
	Gateway          *channels.Gateway
	DefaultChannelID int64  // Auto-filled from channel source context (0 = not set)
	DefaultTargetID  string // Auto-filled from channel source context ("" = not set)
}

// NewWhatsAppSenderTool creates a tool that sends messages via a connected WhatsApp channel.
func NewWhatsAppSenderTool(config *WhatsAppSenderConfig) (tool.BaseTool, error) {
	if config == nil || config.Gateway == nil {
		return nil, fmt.Errorf("Gateway is required for whatsapp_sender tool")
	}
	return &whatsappSenderTool{
		gateway:          config.Gateway,
		defaultChannelID: config.DefaultChannelID,
		defaultTargetID:  config.DefaultTargetID,
	}, nil
}

type whatsappSenderTool struct {
	gateway          *channels.Gateway
	defaultChannelID int64
	defaultTargetID  string
}

type whatsappSenderInput struct {
	ChannelID int64  `json:"channel_id"`
	TargetID  string `json:"target_id"`
	Content   string `json:"content"`
	FilePath  string `json:"file_path"`
}

func (t *whatsappSenderTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	descEN := "Send a message or file to WhatsApp via a connected channel. " +
		"Supports sending text messages or media files (images, videos, audio, documents). " +
		"For text: provide content as plain text. " +
		"For files: provide file_path with a local file path; the file will be uploaded to WhatsApp automatically."
	descZH := "通过已连接的 WhatsApp 渠道发送消息或文件。支持发送文本消息或媒体文件（图片、视频、音频、文档）。" +
		"发送文本：提供纯文本 content。" +
		"发送文件：提供 file_path 本地文件路径，工具会自动上传到 WhatsApp 服务器后发送。"

	channelIDDescEN := "The channel ID of the connected WhatsApp channel to use for sending."
	channelIDDescZH := "用于发送的已连接 WhatsApp 渠道 ID。"
	targetIDDescEN := "WhatsApp recipient ID (e.g., 1234567890 or 1234567890@s.whatsapp.net for users, or group JID)."
	targetIDDescZH := "WhatsApp 接收方 ID（例如：用户号码 1234567890 或群组 JID）。"

	channelIDRequired := true
	targetIDRequired := true

	if t.defaultChannelID > 0 && t.defaultTargetID != "" {
		descEN += fmt.Sprintf(" When this conversation originates from a WhatsApp channel, channel_id and target_id are auto-detected (defaults: channel_id=%d, target_id=%s) and can be omitted.", t.defaultChannelID, t.defaultTargetID)
		descZH += fmt.Sprintf(" 当本会话来源于 WhatsApp 渠道时，channel_id 和 target_id 已自动检测（默认值：channel_id=%d, target_id=%s），可省略不填。", t.defaultChannelID, t.defaultTargetID)
		channelIDDescEN += fmt.Sprintf(" Auto-detected default: %d. Can be omitted.", t.defaultChannelID)
		channelIDDescZH += fmt.Sprintf(" 已自动检测，默认值：%d，可省略。", t.defaultChannelID)
		targetIDDescEN += fmt.Sprintf(" Auto-detected default: %s. Can be omitted.", t.defaultTargetID)
		targetIDDescZH += fmt.Sprintf(" 已自动检测，默认值：%s，可省略。", t.defaultTargetID)
		channelIDRequired = false
		targetIDRequired = false
	}

	return &schema.ToolInfo{
		Name: tools.ToolIDWhatsAppSender,
		Desc: selectDesc(descEN, descZH),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"channel_id": {
				Type:     schema.Integer,
				Desc:     selectDesc(channelIDDescEN, channelIDDescZH),
				Required: channelIDRequired,
			},
			"target_id": {
				Type:     schema.String,
				Desc:     selectDesc(targetIDDescEN, targetIDDescZH),
				Required: targetIDRequired,
			},
			"content": {
				Type: schema.String,
				Desc: selectDesc(
					"Message content for text messages. Not required when file_path is provided.",
					"文本消息内容。当提供 file_path 时可不填。",
				),
			},
			"file_path": {
				Type: schema.String,
				Desc: selectDesc(
					"Local file path to upload and send. Automatically detects file type (image, video, audio, document) from extension.",
					"要上传并发送的本地文件路径。会根据扩展名自动识别文件类型（图片、视频、音频、文档）。",
				),
			},
		}),
	}, nil
}

func (t *whatsappSenderTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var in whatsappSenderInput
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if in.ChannelID <= 0 && t.defaultChannelID > 0 {
		in.ChannelID = t.defaultChannelID
	}
	if strings.TrimSpace(in.TargetID) == "" && t.defaultTargetID != "" {
		in.TargetID = t.defaultTargetID
	}

	if in.ChannelID <= 0 {
		return "Error: channel_id is required and must be positive", nil
	}
	if strings.TrimSpace(in.TargetID) == "" {
		return "Error: target_id is required", nil
	}

	hasContent := strings.TrimSpace(in.Content) != ""
	hasFile := strings.TrimSpace(in.FilePath) != ""
	if !hasContent && !hasFile {
		return "Error: either content or file_path is required", nil
	}

	adapter := t.gateway.GetAdapter(in.ChannelID)
	if adapter == nil {
		return fmt.Sprintf("Error: channel %d is not connected", in.ChannelID), nil
	}
	if adapter.Platform() != channels.PlatformWhatsApp {
		return fmt.Sprintf("Error: channel %d is not a WhatsApp channel (platform: %s)", in.ChannelID, adapter.Platform()), nil
	}

	waAdapter, ok := adapter.(*channels.WhatsAppAdapter)
	if !ok {
		return "Error: failed to cast adapter to WhatsAppAdapter", nil
	}

	if hasFile {
		filePath := strings.TrimSpace(in.FilePath)
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Sprintf("Error: file not accessible: %s", err.Error()), nil
		}

		if err := waAdapter.SendMedia(ctx, in.TargetID, filePath); err != nil {
			return fmt.Sprintf("Error: failed to send media: %s", err.Error()), nil
		}
		return fmt.Sprintf("Media sent successfully to %s via channel %d.", in.TargetID, in.ChannelID), nil
	}

	if err := waAdapter.SendMessage(ctx, in.TargetID, in.Content); err != nil {
		return fmt.Sprintf("Error: failed to send message: %s", err.Error()), nil
	}
	return fmt.Sprintf("Message sent successfully to %s via channel %d.", in.TargetID, in.ChannelID), nil
}
