# 实验二运行手册：主节点抢注，无 PoW

本文件保留为实验二入口文档。当前分支的详细执行步骤见：

```text
deploy/swarm/LAB_EXP2_RUNBOOK.md
```

当前分支基线：

```text
v0.6-primarybz-nopow
```

关键原则：

- 只做 Swarm 部署迁移
- 不重写 DNS 业务代码
- 不重写 `TopLevelUpdate` 参数结构
- 不替换原有主节点抢注逻辑

实验二仍使用原始 6 参数业务请求：

```text
TopLevelUpdate(domain, ip, type, ttl, owner, signature)
```

恶意主节点仍使用原代码中的 `makeTxNoPow` 构造抢注交易：

```text
TopLevelUpdate(domain, 4.4.4.4, type, ttl, JIM, signatureByJIM)
```
