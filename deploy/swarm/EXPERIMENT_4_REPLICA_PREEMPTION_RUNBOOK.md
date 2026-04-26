# 实验四运行手册：副本节点作恶抢注

本文档对应分支 `feature/swarm-exp4-replica-preemption`，用于在当前 10 节点 Swarm 环境中复现实验四“恶意副本节点抢先伪造顶级域注册”。

## 1. 实验目标

目标是验证以下路径：

- 正常用户通过一个恶意副本节点提交 `TopLevelUpdate("example.org", "10.92.2.140:53")`
- 恶意副本节点在把原始请求转发给主节点前，先广播一个伪造请求
- 伪造请求会被改写成 `TopLevelUpdate("example.org", "<恶意权威地址>")`
- 链码采用“先注册先占有”语义，后到的正常注册失败
- 最终链上 `org` 的权威地址变为恶意地址

## 2. 代码改动摘要

- `consensus/pbft/batch.go`
  - 恶意副本节点识别 `TopLevelUpdate`
  - 先广播一个改写后的恶意请求，再转发原始请求
  - 视图切换和状态同步时清空作恶标记
- `examples/chaincode/go/chaincode_dns_reslover/functions/topLevelDomainUpdate.go`
  - 同一个顶级域一旦已存在，再次注册直接失败
- `scripts/swarm/invoke-top-level-update-replica.sh`
  - 从恶意副本节点入口发送顶级域注册请求
- `scripts/swarm/query-top-level.sh`
  - 查询某个顶级域当前映射

## 3. 实验前提

- 服务器代码已经切到 `feature/swarm-exp4-replica-preemption`
- 10 个 PBFT 节点已经形成完整网络
- `bind9` 服务可用
- `.env` 已存在于 `deploy/swarm/.env`

## 4. 关键配置

在 `deploy/swarm/.env` 中至少确认以下参数：

```bash
VP0_BYZANTINE=false
VP4_BYZANTINE=true
VP8_BYZANTINE=false
DNS_BZAUTHORITY=10.92.2.141:53
VP0_ENDPOINT=10.92.2.138:7051
REPLICA_ENTRY_ENDPOINT=10.92.2.139:7151
```

说明：

- 当前实验默认让 `vp4` 作为恶意副本节点
- `DNS_BZAUTHORITY` 是恶意副本节点伪造后要写入链上的权威 DNS 地址
- `REPLICA_ENTRY_ENDPOINT` 必须指向恶意副本节点对外暴露的 gRPC 入口

## 5. 执行步骤

### 5.1 部署或重启 Stack

```bash
cd /root/go/src/github.com/hyperledger/fabric
git checkout feature/swarm-exp4-replica-preemption
git pull origin feature/swarm-exp4-replica-preemption
bash scripts/swarm/deploy-stack.sh
```

### 5.2 确认 10 节点已经连通

```bash
curl -s http://localhost:7050/network/peers | python3 -c \
  "import sys,json; peers=json.load(sys.stdin)['peers']; print(len(peers)); [print(p['ID']['name'], p['address']) for p in peers]"
```

预期结果：

- 输出节点数为 `10`

### 5.3 部署 DNS 链码

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
cat deploy/swarm/last-chaincode-id.txt
```

### 5.4 先查看当前顶级域状态

```bash
bash scripts/swarm/query-top-level.sh example.org
```

预期结果：

- 初始情况下 `org` 尚未注册，或返回空结果

### 5.5 通过恶意副本发起正常注册请求

```bash
bash scripts/swarm/invoke-top-level-update-replica.sh example.org 10.92.2.140:53
```

这条命令表面上发送的是正常请求，但若 `vp4` 为恶意副本，会先额外广播：

```text
TopLevelUpdate("example.org", "10.92.2.141:53")
```

然后再把用户原始请求转发给主节点。

### 5.6 查询链上最终结果

```bash
bash scripts/swarm/query-top-level.sh example.org
```

预期结果：

- 查询返回的 `authorityServer` 为 `10.92.2.141:53`
- 而不是用户原始提交的 `10.92.2.140:53`

## 6. 判定标准

满足以下两条即可认为实验四复现成功：

- 用户提交的是正常权威地址
- 链上最终保存的是恶意权威地址

这说明抢注不是发生在主节点组批阶段，而是发生在恶意副本节点接收用户请求之后、转发给主节点之前。

## 7. 失败排查

- 若 `query-top-level.sh` 提示找不到链码：
  - 检查 `deploy/swarm/last-chaincode-id.txt`
- 若查询结果仍是正常地址：
  - 检查 `VP4_BYZANTINE=true` 是否已生效
  - 检查 `REPLICA_ENTRY_ENDPOINT` 是否确实打到了 `vp4`
  - 检查 `DNS_BZAUTHORITY` 是否已注入容器环境
- 若 invoke 直接失败：
  - 检查 `vp4` 的 7151 端口是否已暴露
  - 检查 10 节点是否都已连通

## 8. 注意事项

- 新增脚本在部分环境下可能没有 git 可执行位，必要时请使用 `bash scripts/swarm/...` 调用
- 本实验修改了 `TopLevelUpdate` 语义：当前分支中它不再是覆盖更新，而是“首次注册成功，重复注册失败”
- 如果要重复做实验，请先删除目标顶级域，或更换为新的顶级域名
