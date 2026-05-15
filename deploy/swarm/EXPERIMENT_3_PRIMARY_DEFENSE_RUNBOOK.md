# 实验三入口：主节点攻击防御，PoW

当前分支对应实验三 Swarm 迁移版本：

```bash
feature/swarm-lab-exp3-primarybz-pow
```

业务基线是原始分支：

```bash
v0.6-primarybz-pow
```

本分支只迁移 Docker Swarm 部署层和必要运行配置，不重写 `TopLevelUpdate`，不替换原始主节点作恶/防御逻辑。

详细执行步骤见：

```bash
deploy/swarm/LAB_EXP3_RUNBOOK.md
```

实验三仍使用原始 6 参数用户请求：

```json
{"Function":"TopLevelUpdate","Args":["example.org","10.161.34.51:53","A","3600","USER","SIG"]}
```

恶意主节点内部仍按 `v0.6-primarybz-pow` 中的 `makeTxByPow` 构造带 PoW 字段的伪造交易，不在脚本层重写业务结构。
