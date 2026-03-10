# Scheduled Task Custom Cascader Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将计划任务新建弹窗中的自定义时间配置改为级联式单选交互，并清理对应说明文案。

**Architecture:** 保持现有 `ScheduledTaskFormState` 和提交逻辑不变，只在 `CreateTaskDialog.vue` 中调整自定义时间区域的渲染结构与本地交互状态。周/月单选继续复用现有字段，新增的只是前端弹层开关与展示文案。

**Tech Stack:** Vue 3 `script setup`、TypeScript、Tailwind CSS、现有对话框组件。

---

### Task 1: 评估测试与验证入口

**Files:**
- Modify: `frontend/package.json`

**Step 1: 查找现有前端测试入口**

Run: `Get-Content frontend/package.json`
Expected: 确认是否存在单测命令。

**Step 2: 确认验证策略**

Run: `npm run build`
Expected: 作为当前改动的最小可执行验证。

### Task 2: 改造自定义时间交互

**Files:**
- Modify: `frontend/src/pages/scheduled-tasks/components/CreateTaskDialog.vue`

**Step 1: 先写失败场景的检查点**

检查当前组件，确认仍存在以下不符合需求的行为：
- 存在“执行频率”文案
- `daily` 模式右侧存在说明文案
- `weekly/monthly` 右侧是大列表，不是级联式触发器

**Step 2: 写最小实现**

- 删除“执行频率”标题
- 删除 `daily` 右侧说明块
- 新增 `weekly/monthly` 触发器与弹层
- 保持单选并写回原字段

**Step 3: 本地验证**

Run: `npm run build`
Expected: 构建通过。

### Task 3: 复核工作区影响

**Files:**
- Modify: `frontend/src/pages/scheduled-tasks/components/CreateTaskDialog.vue`
- Create: `docs/plans/2026-03-10-scheduled-task-custom-cascader-design.md`
- Create: `docs/plans/2026-03-10-scheduled-task-custom-cascader-implementation-plan.md`

**Step 1: 检查最终差异**

Run: `git diff -- docs/plans/2026-03-10-scheduled-task-custom-cascader-design.md docs/plans/2026-03-10-scheduled-task-custom-cascader-implementation-plan.md frontend/src/pages/scheduled-tasks/components/CreateTaskDialog.vue`
Expected: 仅包含本次需求相关变更。
