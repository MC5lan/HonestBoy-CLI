# 指导论文

《构造性闭环：面向本地编程智能体的信息溯源隔离与完成门禁》

- 文件：[`constructive-closure.pdf`](constructive-closure.pdf)（1.4 MB）
- SHA256：`391A5BAC8A89184C7E2F6C28DCE4BB9706CB0D93D86E42AAF8A92377652E4ACF`

Honest Boy 的核心机制直接来自这篇论文：

- 真值来自验证器而非模型自述
- 完成权外置到模型不可写的 Gate
- 只有一级验证器、真实运行、退出码非负、通过且原文哈希匹配的证据才能接地
- 自我保证、投降表述与长度漂移作为形态观测量
- Receipt 哈希链让落盘后的证据可校验、可定位篡改

实现映射见 [`../architecture.md`](../architecture.md) 与 [`../evidence-format.md`](../evidence-format.md)。
