# Swarm 实验中的 Bind9 目录说明

本目录中的文件是 `deploy/swarm/stack.yml` 中 `bind9` 服务的样板配置起点。

当前设计默认假设：

- 权威 DNS 节点是 `.env` 中的 `BIND_NODE`
- zone 文件挂载到 `/var/lib/bind`
- Fabric DNS 链码通过动态更新 / 查询直接与 Bind9 交互

## 在 BIND_NODE 上安装

在 `BIND_NODE` 对应宿主机上拉取当前分支代码后，执行：

```bash
./scripts/swarm/prepare-bind-layout.sh
```

该脚本会把当前目录中的样板配置和 zone 文件复制到宿主机路径，具体目标路径由以下变量控制：

- `BIND_CONFIG_DIR`
- `BIND_CACHE_DIR`
- `BIND_RECORDS_DIR`

## 动态更新策略

当前样板 zone 配置中使用的是：

```txt
allow-update { any; };
```

这是为了内网实验快速跑通而故意设置得比较宽松。

如果以后要在实验室以外或更严格环境中使用，应该改为更受限的更新策略。

## 文件说明

- `config/named.conf`：Bind9 主 include 文件
- `config/named.conf.options`：Bind9 运行参数
- `config/named.conf.local`：定义 `com` 和 `cn` 权威 zone
- `records/db.com`：`com` 的初始 zone 文件
- `records/db.cn`：`cn` 的初始 zone 文件
