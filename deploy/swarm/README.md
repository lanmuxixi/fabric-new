# Fabric DNS 实验三 Swarm 部署目录

当前目录对应实验三 10 节点 Docker Swarm 部署版本：

```bash
feature/swarm-lab-exp3-primarybz-pow
```

业务基线是原始分支：

```bash
v0.6-primarybz-pow
```

本目录只负责部署层迁移，不改原有 DNS 链码、PBFT 主节点作恶逻辑或 PoW 防御逻辑。

## 目录结构

```bash
deploy/swarm/
  stack.yml
  .env.example
  .env.lab.example
  LAB_EXP3_RUNBOOK.md
  EXPERIMENT_3_PRIMARY_DEFENSE_RUNBOOK.md
  EXPERIMENT_RUNBOOK.md
  bind/

scripts/swarm/
  build-peer-image.sh
  deploy-stack.sh
  deploy-dns-chaincode.sh
  prepare-bind-layout.sh
  register-users.sh
  invoke-top-level-update.sh
  query-top-level.sh
  query-top-levels.sh
  exec-vp0.sh
  remove-stack.sh
```

## 部署边界

本分支做的事情：

- 新增 Docker Swarm 版 `stack.yml`。
- 固定 10 个 peer 的 Swarm placement。
- 关闭 `CORE_PEER_ADDRESSAUTODETECT`，避免容器 overlay IP 漂移造成 PBFT 地址缓存失效。
- 将 PBFT 参数放入 `.env`，默认 10 节点、`f=3`、batch size 500、batch timeout 1s。
- 提供 Bind9 配置和部署脚本。

本分支不做的事情：

- 不重写 `TopLevelUpdate` 参数结构。
- 不替换 `v0.6-primarybz-pow` 的主节点作恶逻辑。
- 不新增脚本层伪造 8 参数业务请求。
- 不修改链码业务逻辑。

## 两台机器布局

默认实验室布局：

| 机器 | Swarm hostname | IP | 服务 |
|---|---|---|---|
| 服务器 1 | `roott-I620-G20` | `10.161.34.8` | `vp0` 到 `vp4` |
| 服务器 2 | `qichang-I420-G20` | `10.161.34.51` | `vp5` 到 `vp9`、`bind9` |

实际 hostname 以 `docker node ls` 为准。如果不同，改 `deploy/swarm/.env` 中的 `VP*_NODE` 和 `BIND_NODE`。

## 快速运行顺序

```bash
cp deploy/swarm/.env.lab.example deploy/swarm/.env
bash scripts/swarm/build-peer-image.sh
bash scripts/swarm/prepare-bind-layout.sh
bash scripts/swarm/deploy-stack.sh
bash scripts/swarm/deploy-dns-chaincode.sh
bash scripts/swarm/deploy-stack.sh
bash scripts/swarm/register-users.sh
```

链码部署后必须第二次执行 `deploy-stack.sh`，目的是把 `DNS_CHAINCODEID` 注入 `vp0`。原始实验三代码需要该值来执行内部伪造交易。

## 镜像标签

实验三默认 peer 镜像标签：

```bash
fabric-dns-peer:swarm-exp3
```

如果离线搬运镜像：

```bash
docker save fabric-dns-peer:swarm-exp3 -o /tmp/fabric-dns-peer-swarm-exp3.tar
docker load -i /tmp/fabric-dns-peer-swarm-exp3.tar
```

两台机器都需要导入同一个镜像。

## 关键配置

```bash
CORE_PBFT_GENERAL_N=10
CORE_PBFT_GENERAL_F=3
CORE_PBFT_GENERAL_BATCHSIZE=500
CORE_PBFT_GENERAL_TIMEOUT_BATCH=1s
CORE_PBFT_GENERAL_TIMEOUT_REQUEST=24h
CORE_PBFT_GENERAL_TIMEOUT_VIEWCHANGE=24h
CORE_PBFT_GENERAL_VIEWCHANGEPERIOD=0
VP0_BYZANTINE=true
DNS_CHAINCODEID=
```

`DNS_CHAINCODEID` 初始为空，由 `deploy-dns-chaincode.sh` 部署链码后写回 `.env`。

## 详细文档

完整部署、检查和排错步骤见：

```bash
deploy/swarm/LAB_EXP3_RUNBOOK.md
```
