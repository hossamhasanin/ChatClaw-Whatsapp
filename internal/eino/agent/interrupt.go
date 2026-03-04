package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"chatclaw/internal/eino/tools"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// InterruptInfo is the payload attached to an interrupt signal so that
// the caller (processStream) can display a meaningful confirmation prompt.
type InterruptInfo struct {
	Command string `json:"command"`
}

// dangerousPatterns are command substrings that trigger a confirmation prompt
// when running in native (non-sandbox) mode.
var dangerousPatterns = []string{
	"rm -rf", "rm -r", "rmdir",
	"mkfs", "dd if=",
	"format c:", "format d:",
	":(){:|:&};:",
	"sudo", "> /dev/",
	"chmod -R 777",
	"shutdown", "reboot", "halt",
	"kill -9", "killall",
}

type interruptHandler struct {
	adk.BaseChatModelAgentMiddleware
}

// NewInterruptHandler creates a middleware that interrupts execution of
// dangerous shell commands, giving the user a chance to confirm or reject.
func NewInterruptHandler(_ *tools.Backend) adk.ChatModelAgentMiddleware {
	return &interruptHandler{}
}

func (h *interruptHandler) WrapInvokableToolCall(
	ctx context.Context,
	endpoint adk.InvokableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	if tCtx.Name != "execute" {
		return endpoint, nil
	}

	return func(ctx context.Context, args string, opts ...tool.Option) (string, error) {
		isResume, _, _ := compose.GetResumeContext[any](ctx)
		if isResume {
			return endpoint(ctx, args, opts...)
		}

		cmd := extractCommand(args)
		if cmd == "" || !isDangerous(cmd) {
			return endpoint(ctx, args, opts...)
		}

		return "", compose.Interrupt(ctx, &InterruptInfo{Command: cmd})
	}, nil
}

func extractCommand(argsJSON string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return ""
	}
	cmd, _ := m["command"].(string)
	return cmd
}

func isDangerous(cmd string) bool {
	lower := strings.ToLower(cmd)
	for _, p := range dangerousPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// FormatInterruptPrompt creates the assistant message text shown to the user
// when a dangerous command is intercepted.
func FormatInterruptPrompt(info *InterruptInfo) string {
	return fmt.Sprintf(
		"I'm about to execute a potentially dangerous command:\n\n```\n%s\n```\n\nPlease reply **confirm** to proceed or **reject** to cancel.",
		info.Command,
	)
}
