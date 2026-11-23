# 错误码全表

格式 `HB-<域>-<编号>`。可用 `hb explain <code>` 查询。

| 码 | 含义 | 处置 |
|---|---|---|
| HB-NET-001 | 模型端点无法连接 | 检查 base_url / HTTPS_PROXY / 防火墙；`hb doctor` |
| HB-NET-002 | 请求或流式超时 | 检查网络；`--timeout` 增大超时 |
| HB-SBX-003 | 路径越出沙箱 | 硬边界：把外部文件复制进工作区后使用 |
| HB-TOOL-001 | 工具执行失败 | 查看 tool_end 原文输出，按提示修正参数 |
| HB-GATE-002 | 完成申请被拒 | 按 gate_deny 的 Missing/Reason 补证据；无证据不能完成 |
| HB-CFG-001 | 配置无效 | `hb config list`；密钥只放环境变量 |
| HB-IO-001 | 磁盘读写失败 | 检查路径权限与剩余空间 |
