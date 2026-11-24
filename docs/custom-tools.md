# 自定义工具与 web_fetch

## tools.d

自定义工具默认关闭。开启方式：

```bash
hb config set tools.custom_dir tools.d
```

目录相对工作区；每个 `*.json` 声明一个工具：

```json
{
  "name": "project_echo",
  "description": "回显一段文本",
  "command": "cmd",
  "args": ["/c", "echo", "{text}"],
  "risk": "read",
  "timeout_ms": 10000,
  "schema": {
    "type": "object",
    "properties": {
      "text": {"type": "string"}
    },
    "required": ["text"]
  }
}
```

规则：

- `command` 直接 `exec`，不经过 shell；`args` 里的 `{name}` 用模型参数替换。
- `risk` 必须是 `read` / `write` / `exec` / `net`；默认 `exec`。
- `schema` 必须是合法 JSON Schema 且 `type=object`。
- 名称与内置工具冲突会直接拒绝启动。
- 工具输出上限 1MB，默认超时 30 秒，可通过 `timeout_ms` 调整。
- 自定义工具仍走审批策略与 Gate 证据规则，不会因为“自定义”绕过。

## web_fetch

默认关闭；开启必须同时配置域名白名单：

```bash
hb config set web.enabled true
hb config set web.allowed_domains '["docs.example.com","api.example.com"]'
```

行为：

- 只允许 `http/https`，域名精确匹配或子域名匹配白名单。
- 拒绝私网、回环、链路本地地址。
- 响应超过 2MB 截断；输出 `HTTP 状态 + Content-Type + body`。
- `RiskNet`，按审批策略执行，不会自动批准。
