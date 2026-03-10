package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"chatclaw/internal/eino/tools"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// filterToolsByName extracts tools matching any of the given names.
func filterToolsByName(allTools []tool.BaseTool, names ...string) []tool.BaseTool {
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}

	var result []tool.BaseTool
	for _, t := range allTools {
		info, err := t.Info(context.Background())
		if err != nil || info == nil {
			continue
		}
		if _, ok := nameSet[info.Name]; ok {
			result = append(result, t)
		}
	}
	return result
}

// excludeToolsByName returns tools NOT matching any of the given names.
func excludeToolsByName(allTools []tool.BaseTool, names ...string) []tool.BaseTool {
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}

	var result []tool.BaseTool
	for _, t := range allTools {
		info, err := t.Info(context.Background())
		if err != nil || info == nil {
			continue
		}
		if _, ok := nameSet[info.Name]; !ok {
			result = append(result, t)
		}
	}
	return result
}

// buildSubAgentHandlers creates middleware handlers for a sub-agent.
func buildSubAgentHandlers(
	ctx context.Context,
	backend *tools.Backend,
	config Config,
	chatModel model.BaseChatModel,
	logger *slog.Logger,
	instruction string,
	subAgentName string,
	needReduction, needSummarization, needSkill bool,
) []adk.ChatModelAgentMiddleware {
	var handlers []adk.ChatModelAgentMiddleware

	handlers = append(handlers, NewInstructionHandler(instruction))

	if h := buildPatchToolCallsHandler(ctx, logger); h != nil {
		handlers = append(handlers, h)
	}

	if needReduction || needSummarization {
		subEinoDir := filepath.Join(backend.WorkDir(), einoMetaDir, subAgentName)
		_ = os.MkdirAll(subEinoDir, 0o755)

		if needReduction {
			reductionPath := filepath.Join(subEinoDir, reductionDir)
			_ = os.MkdirAll(reductionPath, 0o755)
			if h := buildReductionHandler(ctx, backend, reductionPath, logger); h != nil {
				handlers = append(handlers, h)
			}
		}

		if needSummarization {
			transcriptPath := filepath.Join(subEinoDir, transcriptFile)
			if h := buildSummarizationHandler(ctx, chatModel, transcriptPath, logger); h != nil {
				handlers = append(handlers, h)
			}
		}
	}

	if needSkill && config.SkillsEnabled {
		if h := buildSkillHandler(ctx, backend, logger); h != nil {
			handlers = append(handlers, h)
		}
	}

	return handlers
}

// --- general-purpose sub-agent (DeerFlow-style: full toolset for any non-trivial task) ---

const generalPurposeMaxIterations = 50

func generalPurposeDescription() string {
	if isZhCN() {
		return "执行代理：拥有完整工具集（web_search、write_file、edit_file、execute、glob、grep 等）。处理调研搜索、写代码、文件操作、分析等任何需要独立上下文的任务。需要搜索或调研时必须用此代理。"
	}
	return "Execution agent with full toolset (web_search, write_file, edit_file, execute, glob, grep, etc.). Handles research/search, coding, file operations, analysis — any task requiring isolated context. MUST use this agent when search or research is needed."
}

func generalPurposeInstruction(workDir, toolchainBinDir string, sandboxEnabled, sandboxNetworkEnabled, skillsEnabled bool) string {
	if isZhCN() {
		inst := fmt.Sprintf(`你是执行代理，在独立上下文中自主完成委派的任务。

## 环境
- 工作目录: %s
- 所有文件操作使用绝对路径，基于工作目录`, workDir)

		if sandboxEnabled {
			inst += fmt.Sprintf(`
- 沙箱模式已启用：写入操作仅限工作目录 %s 内`, workDir)
			if sandboxNetworkEnabled {
				inst += "\n- 网络访问已启用"
			} else {
				inst += "\n- 网络访问已禁用"
			}
		}

		if toolchainBinDir != "" {
			inst += "\n- 已预装 uv（Python）和 bun（JavaScript），优先使用"
		}

		inst += `

## 工具使用
- 创建/写入文件用 write_file，编辑现有文件用 edit_file
- 运行命令用 execute，长时间运行的命令用 execute_background
- 读取文件用 read_file，搜索文件用 glob/grep
- 运行 Python 脚本优先用 uv run，运行 JS/TS 优先用 bun run
- 运行 shell 脚本时用 "sh script.sh"（或 "bash script.sh" / "zsh script.sh"），不要用 "./script.sh"（没有执行权限），也不要尝试 chmod
- 需要确认的危险命令先调用 confirm_execution

## 原则
- 理解任务目标后立即开始执行
- 遇到错误时自行诊断和修复，不要放弃
- 完成后清晰总结：做了什么、生成了哪些文件、结果在哪里
- 如果无法完成，说明原因和已尝试的方法
- 调研类任务：产出精炼结论，带来源和证据，不要堆砌过程`

		if skillsEnabled {
			inst += `

## 技能系统
已安装的技能会自动加载到你的能力中。
- 用 skill_list 查看已安装的技能
- 用 skill_search 搜索技能市场，查找与当前任务相关的技能
- 用 skill_install 安装、skill_enable 启用、read_skill 读取技能内容
- 遇到不熟悉的任务时，先搜索是否有相关技能可以指导`
		}
		return inst
	}

	inst := fmt.Sprintf(`You are an execution agent working autonomously in an isolated context.

## Environment
- Working directory: %s
- Use absolute paths based on the working directory for all file operations`, workDir)

	if sandboxEnabled {
		inst += fmt.Sprintf(`
- Sandbox mode enabled: write operations restricted to %s`, workDir)
		if sandboxNetworkEnabled {
			inst += "\n- Network access is enabled"
		} else {
			inst += "\n- Network access is disabled"
		}
	}

	if toolchainBinDir != "" {
		inst += "\n- uv (Python) and bun (JavaScript) are pre-installed — prefer them"
	}

	inst += `

## Tool Usage
- Create/write files with write_file, edit existing files with edit_file
- Run commands with execute, long-running commands with execute_background
- Read files with read_file, search with glob/grep
- Run Python scripts with uv run, JS/TS with bun run
- Run shell scripts with "sh script.sh" (or "bash script.sh" / "zsh script.sh") — never use "./script.sh" (no execute permission) and do not attempt chmod
- Call confirm_execution before dangerous commands

## Principles
- Begin execution immediately after understanding the task goal
- Self-diagnose and fix errors — do not give up
- Summarize clearly when done: what was done, files created, where results are
- If unable to complete, explain why and what was attempted
- For research tasks: produce condensed conclusions with sources and evidence, not process narration`

	if skillsEnabled {
		inst += `

## Skill System
Installed skills are automatically loaded into your capabilities.
- Use skill_list to view installed skills
- Use skill_search to search the marketplace for skills related to the current task
- Use skill_install, skill_enable, and read_skill as needed
- When facing unfamiliar tasks, search for relevant skills first`
	}
	return inst
}

// newGeneralPurposeSubAgent creates the general-purpose sub-agent (DeerFlow-style).
// It has access to all tools except sub-agent tools.
func newGeneralPurposeSubAgent(
	ctx context.Context,
	chatModel model.BaseChatModel,
	allTools []tool.BaseTool,
	backend *tools.Backend,
	config Config,
	skillBackend *filteringSkillBackend,
	logger *slog.Logger,
) (tool.BaseTool, error) {
	// Exclude sub-agent tools to prevent nesting (read_skill is already in baseTools when SkillsEnabled)
	tools := excludeToolsByName(allTools, "general_purpose", "bash")

	handlers := buildSubAgentHandlers(ctx, backend, config, chatModel, logger,
		generalPurposeInstruction(backend.WorkDir(), backend.ToolchainBinDir(),
			backend.SandboxEnabled(), backend.SandboxEnabled() && config.SandboxNetwork,
			config.SkillsEnabled),
		"general_purpose",
		true, true, true,
	)

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "general_purpose",
		Description: generalPurposeDescription(),
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               tools,
				ToolCallMiddlewares: []compose.ToolMiddleware{ErrorCatchingToolMiddleware(tools, logger)},
			},
		},
		Handlers:      handlers,
		MaxIterations: generalPurposeMaxIterations,
	})
	if err != nil {
		return nil, err
	}

	return adk.NewAgentTool(ctx, agent), nil
}

// --- bash sub-agent (DeerFlow-style: command execution specialist) ---

const bashMaxIterations = 30

func bashDescription() string {
	if isZhCN() {
		return "终端代理：仅用于运行 bash 命令序列（git、npm、docker、构建/测试/部署）。只有 execute、ls、read_file、write_file、edit_file，没有搜索能力。不要用于调研或搜索任务。"
	}
	return "Terminal agent for bash command sequences ONLY (git, npm, docker, build/test/deploy). Has only execute, ls, read_file, write_file, edit_file — NO search capability. Do NOT use for research or search tasks."
}

func bashInstruction(workDir string, sandboxEnabled, sandboxNetworkEnabled bool) string {
	if isZhCN() {
		inst := fmt.Sprintf(`你是命令执行助手，在独立上下文中运行 bash 命令。

## 环境
- 工作目录: %s
- 使用绝对路径进行文件操作`, workDir)

		if sandboxEnabled {
			inst += fmt.Sprintf(`
- 沙箱模式已启用：写入仅限 %s`, workDir)
			if sandboxNetworkEnabled {
				inst += "\n- 网络访问已启用"
			} else {
				inst += "\n- 网络访问已禁用"
			}
		}

		inst += `

## 原则
- 相关命令逐个执行，相互独立的命令可并行
- 报告 stdout 和 stderr（相关时）
- 出错时解释原因
- 对破坏性操作（rm、覆盖等）保持谨慎

## 输出格式
对每条或每组命令：1) 执行了什么 2) 结果（成功/失败）3) 相关输出（冗长则摘要）4) 错误或警告`
		return inst
	}

	inst := fmt.Sprintf(`You are a bash command execution specialist. Execute the requested commands carefully and report results clearly.

## Environment
- Working directory: %s
- Use absolute paths for file operations`, workDir)

	if sandboxEnabled {
		inst += fmt.Sprintf(`
- Sandbox mode enabled: writes restricted to %s`, workDir)
		if sandboxNetworkEnabled {
			inst += "\n- Network access is enabled"
		} else {
			inst += "\n- Network access is disabled"
		}
	}

	inst += `

## Principles
- Execute commands one at a time when they depend on each other
- Use parallel execution when commands are independent
- Report both stdout and stderr when relevant
- Handle errors gracefully and explain what went wrong
- Be cautious with destructive operations (rm, overwrite, etc.)

## Output Format
For each command or group: 1) What was executed 2) Result (success/failure) 3) Relevant output (summarized if verbose) 4) Any errors or warnings`
	return inst
}

// newBashSubAgent creates the bash sub-agent (DeerFlow-style).
// Tools: execute, ls, read_file, write_file, edit_file (sandbox tools only).
func newBashSubAgent(
	ctx context.Context,
	chatModel model.BaseChatModel,
	registeredTools []tool.BaseTool,
	backend *tools.Backend,
	config Config,
	logger *slog.Logger,
) (tool.BaseTool, error) {
	bashTools := filterToolsByName(registeredTools,
		"execute", "ls", "read_file", "write_file", "edit_file",
	)

	handlers := buildSubAgentHandlers(ctx, backend, config, chatModel, logger,
		bashInstruction(backend.WorkDir(),
			backend.SandboxEnabled(), backend.SandboxEnabled() && config.SandboxNetwork),
		"bash",
		true, true, false, // bash does not need skill middleware
	)

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "bash",
		Description: bashDescription(),
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               bashTools,
				ToolCallMiddlewares: []compose.ToolMiddleware{ErrorCatchingToolMiddleware(bashTools, logger)},
			},
		},
		Handlers:      handlers,
		MaxIterations: bashMaxIterations,
	})
	if err != nil {
		return nil, err
	}

	return adk.NewAgentTool(ctx, agent), nil
}
