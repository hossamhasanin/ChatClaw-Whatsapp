package openaiutil

import (
	"context"
	"strings"

	openaicomponent "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func chatWikiExtraHeader(token string) map[string]string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil
	}
	return map[string]string{"Token": trimmed}
}

func appendChatWikiHeader(token string, opts []model.Option) []model.Option {
	header := chatWikiExtraHeader(token)
	if len(header) == 0 {
		return opts
	}
	return append(opts, openaicomponent.WithExtraHeader(header))
}

type chatWikiToolCallingModel struct {
	base  model.ToolCallingChatModel
	token string
}

func WrapToolCallingChatModelWithToken(base model.ToolCallingChatModel, token string) model.ToolCallingChatModel {
	if base == nil || strings.TrimSpace(token) == "" {
		return base
	}
	return &chatWikiToolCallingModel{
		base:  base,
		token: token,
	}
}

func (m *chatWikiToolCallingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.base.Generate(ctx, input, appendChatWikiHeader(m.token, opts)...)
}

func (m *chatWikiToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.base.Stream(ctx, input, appendChatWikiHeader(m.token, opts)...)
}

func (m *chatWikiToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	next, err := m.base.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return WrapToolCallingChatModelWithToken(next, m.token), nil
}

type chatWikiChatModel struct {
	base  model.ChatModel
	token string
}

func WrapChatModelWithToken(base model.ChatModel, token string) model.ChatModel {
	if base == nil || strings.TrimSpace(token) == "" {
		return base
	}
	return &chatWikiChatModel{
		base:  base,
		token: token,
	}
}

func (m *chatWikiChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.base.Generate(ctx, input, appendChatWikiHeader(m.token, opts)...)
}

func (m *chatWikiChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.base.Stream(ctx, input, appendChatWikiHeader(m.token, opts)...)
}

func (m *chatWikiChatModel) BindTools(tools []*schema.ToolInfo) error {
	return m.base.BindTools(tools)
}
