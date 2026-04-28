# 测试组织规范

本项目的测试文件遵循以下组织原则：

## 1. 测试目录结构

所有测试文件应放置在以下位置之一：

- **`tests/`** - 公开 API 测试（使用 `package tests`）
- **`internal/xxx/`** - 内部实现测试（使用 `package xxx`，与源文件同目录）

## 2. 测试分类原则

### 可移动到 `tests/` 的测试

以下类型的测试可以放在 `tests/` 目录：

1. **公开 API 测试** - 只使用包的公开类型和函数（首字母大写）
2. **集成测试** - 测试多个包之间的交互
3. **端到端测试** - 模拟用户场景的完整流程测试

### 必须保留在原位置的测试

以下类型的测试必须保留在 `internal/xxx/` 目录（使用 `package xxx`）：

1. **内部实现测试** - 测试私有函数或私有类型
2. **白盒测试** - 需要访问包内部状态
3. **边界条件测试** - 测试内部实现的边界情况

## 3. 已移动到 `tests/` 的测试

以下测试已成功移动到 `tests/` 目录：

| 测试文件 | 原位置 | 说明 |
|----------|--------|------|
| `engine_error_handling_test.go` | `internal/engine/` | 测试 Engine 公开 API |
| `engine_repl_progress_test.go` | `internal/engine/` | 测试 BuildREPLToolProgressData 公开函数 |
| `command_history_test.go` | `internal/utils/` | 测试 CommandHistoryManager 公开 API |
| `search_tools_glob_test.go` | `internal/tool/search/` | 测试 GlobTool 公开 API |
| `repl_script_parse_test.go` | `internal/tool/repl/` | 测试 Extract* 和 Classify* 公开函数 |
| `repl_tool_mode_test.go` | `internal/tool/repl/` | 测试 REPLTool 公开 API |

## 4. 保留在原位置的测试

以下测试因使用私有类型/函数，保留在原位置：

| 测试文件 | 位置 | 原因 |
|----------|------|------|
| `screen_test.go` | `internal/ui/` | 使用私有函数 `wrapInputForDisplay`、`RenderInputPanel` |
| `render_test.go` | `internal/ui/` | 使用私有函数 `visibleWidth`、`wrapText`、`truncateVisible` |
| `runner_transcript_payload_test.go` | `internal/cli/` | 使用私有函数 `buildTranscriptEntries` |
| `runner_localjsx_test.go` | `internal/cli/` | 使用私有类型 `chatModel`、`localJSXCustomMsg` |
| `runner_scrollback_test.go` | `internal/cli/` | 使用私有类型 `streamUpdate` |
| `structured_io_test.go` | `internal/cli/` | 使用私有函数 `appendUTF8Byte`、`dropLastRune` |
| `cli_interactive_env_test.go` | `cmd/` | 命令行入口测试 |

## 5. 测试助手函数

`tests/testutil.go` 提供通用测试助手函数：

```go
// 使用示例
func TestSomething(t *testing.T) {
    AssertNoError(t, err, "operation should succeed")
    AssertEqual(t, expected, actual, "values should match")
    AssertContains(t, output, "expected text", "output should contain text")
}
```

## 6. 运行测试

```bash
# 运行所有测试
go test ./...

# 只运行 tests/ 目录的测试
go test ./tests/...

# 运行特定包的内部测试
go test ./internal/cli/...

# 运行单个测试文件
go test ./tests/engine_error_handling_test.go
```

## 7. 添加新测试

添加新测试时，请遵循以下步骤：

1. **确定测试类型**：
   - 如果只使用公开 API → 放在 `tests/`
   - 如果需要访问私有实现 → 放在 `internal/xxx/`

2. **公开 API 测试**：
   ```go
   package tests
   
   import (
       "testing"
       "claude-go/internal/somepackage"
   )
   
   func TestPublicAPI(t *testing.T) {
       result := somepackage.PublicFunction()
       // assertions...
   }
   ```

3. **内部实现测试**：
   ```go
   package somepackage
   
   import "testing"
   
   func TestPrivateFunction(t *testing.T) {
       result := privateFunction()
       // assertions...
   }
   ```

## 8. 测试统计

| 类别 | 文件数 | 位置 |
|------|--------|------|
| 公开 API 测试 | 6 | `tests/` |
| 内部实现测试 | 7 | `internal/*/` |
| tests/ 目录原有测试 | 71 | `tests/` |
| **总计** | 84 | - |

## 9. 最佳实践

1. **测试命名**：`<功能>_<场景>_test.go`
2. **测试函数命名**：`Test<功能><场景>`
3. **使用 t.Parallel()**：对于独立测试，启用并行执行
4. **使用 t.TempDir()**：对于需要临时目录的测试
5. **使用 t.Setenv()**：对于需要设置环境变量的测试
