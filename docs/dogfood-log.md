# 自用记录

TD06 要求累计 30 条真实使用中的“今天哪不舒服”。当前是开发期记录，只写实际发生过的问题，
不补写、不编造；正式发布后按使用频率继续累计。

## 2026-11-23

1. `hb export` 的 meta 行手工拼 JSON，Windows 路径 `C:\...` 被当成非法转义 → 改用 `json.Marshal`，并加 jsonl 合法性测试。
2. 自更新时旧二进制正在运行，`Copy-Item` 覆盖失败 → 先改名为 `.old` 再落新文件；启动时清理 `.old`。
3. `govulncheck` 报两个可达漏洞（x/text、goldmark）→ 升级到 v0.39.0 / v1.7.17，复扫为 0。
4. 高危授权测试最初用 `shutdown` 做样本，批准后会真的关机 → 换成无副作用的 `git reset --hard` / `git push --force`（临时空仓库内必然失败）。
5. 自我保证措辞测试发现 `done` 被 Gate 拒绝 → 确认是预期的加严行为，修改测试预期而不是放宽 Gate。
6. 回放样本里 `verify_test` 在空模块上退出码 0，无法制造失败 → 测试前写入一个必失败的 `fail_test.go`。
7. 用 goreleaser snapshot 产物（真实 zip + checksums）通过本地 HTTP 跑了一次 `hb upgrade`，升级后的 `hb --version` 显示 zip 内版本，确认资产命名与解压逻辑一致。
8. 用 DeepSeek Flash 实跑单文件 HTML 太阳系建模。产物 21.8KB；Edge headless 截图正常，8 个行星按钮、暂停/继续、点击显示简介均通过 CDP 实测。运行过程中暴露并修复：
   - 无关 shell 失败证据会让 Gate 直接拒绝完成 → 改为只判定与改动相关的证据。
   - 自我保证正则把“未通过 / 无法通过”误判成自我保证 → 收紧正则。
   - `max_tokens` 按 `MaxTurns*512` 分配，推理模型耗尽预算后返回空响应 → 改为独立 32k 输出预算。
   - 缺少 `[SELF]` 直接终止整个回合 → 改为拒执行当前步并给两次补救。
   - shell 空输出被模型误判为失败 → 工具结果附退出码，system prompt 明确 `request_done` / `declare_untested` 参数。
9. 第 5 次实跑时 DeepSeek 返回 `Insufficient Balance`，修复后的端到端 `done` 复跑未能完成；产物与本地浏览器验证保留在 `_live/html-solar5`。
