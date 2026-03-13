---
name: i18n-check
description: 检查并补充前端和后端的i18n翻译文件。以中文（zh-CN）为基准，检查其他语言翻译文件是否缺少key，并补充缺失的key和对应的翻译值。
---

# i18n 翻译检查与补充

## 快速开始

1. **格式化翻译文件** (确保格式统一):
   ```bash
   # 格式化前端 TS 文件
   python .cursor/skills/i18n-check/scripts/format_frontend.py

   # 格式化后端 JSON 文件
   python .cursor/skills/i18n-check/scripts/format_backend.py
   ```

2. **对比翻译差异**:
   ```bash
   # 对比前端所有语言
   python .cursor/skills/i18n-check/scripts/compare_frontend.py

   # 对比后端所有语言
   python .cursor/skills/i18n-check/scripts/compare_backend.py

   # 对比特定语言
   python .cursor/skills/i18n-check/scripts/compare_frontend.py -t ja-JP
   python .cursor/skills/i18n-check/scripts/compare_backend.py -t ja-JP
   ```

3. **补全缺失的 key** (使用中文作为占位符):
   ```bash
   # 补全前端缺失的 key
   python .cursor/skills/i18n-check/scripts/fill_frontend.py

   # 补全后端缺失的 key
   python .cursor/skills/i18n-check/scripts/fill_backend.py

   # 补全特定语言
   python .cursor/skills/i18n-check/scripts/fill_frontend.py -t en-US
   python .cursor/skills/i18n-check/scripts/fill_backend.py -t en-US
   ```

4. **使用 AI 翻译缺失的内容**:
   - 读取补全后的文件
   - 识别新添加的 key（值为中文）
   - 使用 AI 翻译成目标语言
   - 验证并保存

## 完整工作流程

### Step 1: 格式化

```bash
python .cursor/skills/i18n-check/scripts/format_frontend.py
python .cursor/skills/i18n-check/scripts/format_backend.py
```

### Step 2: 对比

```bash
python .cursor/skills/i18n-check/scripts/compare_frontend.py
python .cursor/skills/i18n-check/scripts/compare_backend.py
```

### Step 3: 补全缺失 key

```bash
# 先预览
python .cursor/skills/i18n-check/scripts/fill_frontend.py --dry-run
python .cursor/skills/i18n-check/scripts/fill_backend.py --dry-run

# 执行补全
python .cursor/skills/i18n-check/scripts/fill_frontend.py
python .cursor/skills/i18n-check/scripts/fill_backend.py
```

### Step 4: AI 翻译

1. 读取补全后的目标语言文件
2. 找出值为中文的 key（这些是刚补全的）
3. 对每个中文值进行机器翻译
4. 保存翻译结果

## 脚本说明

### 脚本位置
所有脚本位于 `.cursor/skills/i18n-check/scripts/` 目录：

| 脚本 | 用途 |
|------|------|
| `format_frontend.py` | 格式化前端 TS 翻译文件 |
| `compare_frontend.py` | 对比前端 TS 翻译文件 |
| `fill_frontend.py` | 补全前端缺失的 key |
| `format_backend.py` | 格式化后端 JSON 翻译文件 |
| `compare_backend.py` | 对比后端 JSON 翻译文件 |
| `fill_backend.py` | 补全后端缺失的 key |

### 使用示例

**对比脚本**
```bash
# 对比所有语言
python compare_frontend.py
python compare_backend.py

# 对比特定语言
python compare_frontend.py -t ja-JP
python compare_backend.py -t ja-JP

# 列出所有可用语言
python compare_frontend.py --list
python compare_backend.py --list
```

**补全脚本**
```bash
# 预览要补全的内容
python fill_frontend.py --dry-run
python fill_backend.py --dry-run

# 执行补全
python fill_frontend.py
python fill_backend.py

# 补全特定语言
python fill_frontend.py -t en-US
python fill_backend.py -t en-US
```

## 文件位置

| 类型 | 目录 | 格式 | 基准文件 |
|------|------|------|----------|
| 前端 | `frontend/src/locales/` | TypeScript `.ts` | `zh-CN.ts` |
| 后端 | `internal/services/i18n/locales/` | JSON `.json` | `zh-CN.json` |

## 注意事项

- **保持 key 结构**: 必须与基准文件完全一致，使用相同的嵌套层级
- **不要删除任何内容**: 只能添加缺失的 key，不能删除现有的 key
- **变量占位符**: 后端 JSON 使用 `{{.xxx}}` 格式，前端使用 `{xxx}` 格式，必须保留
- **格式化后再对比**: 每次对比前先运行格式化脚本，确保格式统一
- **AI 翻译**: 补全 key 后，需要使用 AI 将中文值翻译成目标语言
