package qq

import (
	"context"
	"encoding/json"
	"fmt"
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

// QQSenderConfig configures the qq_sender tool.
type QQSenderConfig struct {
	Gateway          *channels.Gateway
	DefaultChannelID int64
	DefaultTargetID  string
}

// NewQQSenderTool creates a tool that sends messages via a connected QQ channel.
func NewQQSenderTool(config *QQSenderConfig) (tool.BaseTool, error) {
	if config == nil || config.Gateway == nil {
		return nil, fmt.Errorf("Gateway is required for qq_sender tool")
	}
	return &qqSenderTool{
		gateway:          config.Gateway,
		defaultChannelID: config.DefaultChannelID,
		defaultTargetID:  config.DefaultTargetID,
	}, nil
}

type qqSenderTool struct {
	gateway          *channels.Gateway
	defaultChannelID int64
	defaultTargetID  string
}

type qqSenderInput struct {
	ChannelID int64  `json:"channel_id"`
	TargetID  string `json:"target_id"`
	Content   string `json:"content"`
}

func (t *qqSenderTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	descEN := "Send a message to QQ via a connected channel. " +
		"Supports sending to groups (prefix target_id with 'group:') or individual users via C2C (prefix with 'user:'). " +
		"Content: plain text; or JSON: markdown {\"msg_type\":\"markdown\",\"content\":\"...\"}; image/file with url (direct send) or {\"msg_type\":\"image\",\"url\":\"...\",\"srv_send_msg\":false} to upload-then-send; or use {\"msg_type\":\"image\",\"file_info\":\"...\"} with pre-obtained file_info."
	descZH := "通过已连接的 QQ 渠道发送消息。" +
		"支持发送到群聊（target_id 添加 'group:' 前缀）或通过 C2C 发送给个人用户（添加 'user:' 前缀）。" +
		"内容：纯文本；或 JSON：markdown {\"msg_type\":\"markdown\",\"content\":\"...\"}；图片/文件可传 url 直接发送，或 {\"msg_type\":\"image\",\"url\":\"...\",\"srv_send_msg\":false} 先上传再发；也可用 {\"msg_type\":\"image\",\"file_info\":\"...\"} 传入已有 file_info。"

	channelIDDescEN := "The channel ID of the connected QQ channel to use for sending."
	channelIDDescZH := "用于发送的已连接 QQ 渠道 ID。"
	targetIDDescEN := "QQ receive ID. Use 'group:{groupOpenID}' for group messages or 'user:{userOpenID}' for C2C messages. If no prefix, defaults to group message."
	targetIDDescZH := "QQ 接收方 ID。群消息使用 'group:{groupOpenID}'，C2C 私聊使用 'user:{userOpenID}'。无前缀时默认为群消息。"

	channelIDRequired := true
	targetIDRequired := true

	if t.defaultChannelID > 0 && t.defaultTargetID != "" {
		descEN += fmt.Sprintf(" When this conversation originates from a QQ channel, channel_id and target_id are auto-detected (defaults: channel_id=%d, target_id=%s) and can be omitted.", t.defaultChannelID, t.defaultTargetID)
		descZH += fmt.Sprintf(" 当本会话来源于 QQ 渠道时，channel_id 和 target_id 已自动检测（默认值：channel_id=%d, target_id=%s），可省略不填。", t.defaultChannelID, t.defaultTargetID)
		channelIDDescEN += fmt.Sprintf(" Auto-detected default: %d. Can be omitted.", t.defaultChannelID)
		channelIDDescZH += fmt.Sprintf(" 已自动检测，默认值：%d，可省略。", t.defaultChannelID)
		targetIDDescEN += fmt.Sprintf(" Auto-detected default: %s. Can be omitted.", t.defaultTargetID)
		targetIDDescZH += fmt.Sprintf(" 已自动检测，默认值：%s，可省略。", t.defaultTargetID)
		channelIDRequired = false
		targetIDRequired = false
	}

	return &schema.ToolInfo{
		Name: tools.ToolIDQQSender,
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
					"Message content: plain text; or JSON with msg_type (markdown/content, image/file with url or file_info). Use srv_send_msg:false for upload-then-send.",
					"消息内容：纯文本；或 JSON，msg_type 为 markdown/content、image/file 可带 url 或 file_info；srv_send_msg:false 表示先上传再发。",
				),
				Required: true,
			},
		}),
	}, nil
}

func (t *qqSenderTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var in qqSenderInput
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
	if strings.TrimSpace(in.Content) == "" {
		return "Error: content is required", nil
	}

	adapter := t.gateway.GetAdapter(in.ChannelID)
	if adapter == nil {
		return fmt.Sprintf("Error: channel %d is not connected", in.ChannelID), nil
	}
	if adapter.Platform() != channels.PlatformQQ {
		return fmt.Sprintf("Error: channel %d is not a QQ channel (platform: %s)", in.ChannelID, adapter.Platform()), nil
	}

	if err := adapter.SendMessage(ctx, in.TargetID, in.Content); err != nil {
		return fmt.Sprintf("Error: failed to send message: %s", err.Error()), nil
	}
	return fmt.Sprintf("Message sent successfully to %s via channel %d.", in.TargetID, in.ChannelID), nil
}
