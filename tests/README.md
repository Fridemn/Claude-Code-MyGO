# Tests

这个目录放仓库级自动化测试和 smoke checks。

运行全部测试：

```bash
go run . test
```

仅运行 `tests` 目录：

```bash
go test ./tests/...
```

当前 smoke 覆盖：

- MCP service lifecycle
- dynamic MCP tools merge
- hooks runtime execution
