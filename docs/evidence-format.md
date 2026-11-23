# 证据格式

## Provenance 四值
`unknown` / `assumed` / `inferred` / `observed`，只有 `observed` 可作为完成依据。

## Evidence 六种 Kind
`build` / `test` / `vet` / `command` / `file` / `diff`。
`Summary` 必须是验证器输出原文，禁止模型改写。

## 验证器级别与接地谓词（论文 §3/§4）
证据带 `level` 与 `ran` 字段：
- `level=verifier`（一级，编译/静态检查/测试）可接地；`runtime`（二级）与 `review`（三级）只能支持，不能接地。
- `ran=true` 表示验证器真的运行过；未运行属于**证据缺失**（Gate 报 coverage），不是失败测试，也不是成功。
- `exit_code<0` 为无效。
- 接地谓词为确定性函数：`Provenance.CanGround() && Ran && Level==verifier && ExitCode>=0 && OK && 原文哈希匹配`。

## 原文哈希防改写（论文 §4.1）
每条证据的 `raw_hash = sha256(raw)`，其中 `raw` 是验证器输出原文（可能为空）。
Gate 与 `hb verify` 都会校验；任何下游用摘要替换原文都会导致校验失败。

## Receipt 与哈希链
字段顺序固定，时间用 RFC3339Nano UTC，Evidence 按 ID 排序；
`PrevHash` 串成链，`hb verify` 逐条重算定位篡改。

## jsonl 会话格式
首行 meta，随后 event 行与 receipt 行；O_APPEND 逐行写并 Sync。