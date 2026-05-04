# 实验四：三服务器副本节点作恶抢注

本文档对应分支 `feature/swarm-lab-exp4`，用于在三台实验室服务器上运行实验四。

## 1. 实验定位

实验四是副本节点作恶实验，对应历史方向 `v0.6-replica-nopow`。

实验逻辑：

```text
用户从恶意副本 vp4 入口提交正常 TopLevelUpdate
vp4 先广播一条伪造请求
vp4 再转发用户原始请求
链码采用先注册先占有
恶意权威地址先落链
```

默认配置：

```bash
VP4_BYZANTINE=true
REPLICA_ENTRY_ENDPOINT=10.161.34.51:7151
DNS_BZAUTHORITY=10.161.34.22:5353
```

## 2. 节点分布

```text
roott-I620-G20:
  vp0, vp1, vp2, vp3

qichang-I420-G20:
  vp4, vp5, vp6

node21:
  vp7, vp8, vp9, bind9
```

`vp4` 会暴露 gRPC 端口 `7151`，实验四必须从该入口发起请求。

## 3. 拉取分支并准备配置

在 manager 节点 `roott-I620-G20` 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-lab-exp4
git pull origin feature/swarm-lab-exp4
cp deploy/swarm/.env.lab.example deploy/swarm/.env
```

## 4. 构建并分发镜像

```bash
bash scripts/swarm/build-peer-image.sh
docker save fabric-dns-peer:swarm -o /tmp/fabric-dns-peer-swarm.tar
scp /tmp/fabric-dns-peer-swarm.tar root@10.161.34.51:/tmp/
scp /tmp/fabric-dns-peer-swarm.tar root@10.161.34.22:/tmp/
```

在 `10.161.34.51` 和 `10.161.34.22` 上执行：

```bash
docker load -i /tmp/fabric-dns-peer-swarm.tar
```

## 5. 部署 Stack 和链码

```bash
docker network create --driver overlay --attachable fabric_dns || true
bash scripts/swarm/deploy-stack.sh
docker stack services fabricdns
```

确认 10 节点连通：

```bash
curl -s http://localhost:7050/network/peers | python3 -c \
  "import sys,json; peers=json.load(sys.stdin)['peers']; print(len(peers)); [print(p['ID']['name'], p['address']) for p in peers]"
```

部署链码：

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
cat deploy/swarm/last-chaincode-id.txt
```

## 6. 执行副本节点抢注

先查询目标顶级域：

```bash
bash scripts/swarm/query-top-level.sh example.org
```

从恶意副本入口提交正常注册：

```bash
bash scripts/swarm/invoke-top-level-update-replica.sh example.org 10.161.34.22:53
```

查询链上结果：

```bash
bash scripts/swarm/query-top-level.sh example.org
```

预期结果：

```text
authorityServer = 10.161.34.22:5353
```

这说明抢注发生在副本节点接收用户请求之后、转发给主节点之前。

## 7. 成功标准

实验四成功需要满足：

```text
1. PBFT 网络连通 10 个节点
2. VP4_BYZANTINE=true 生效
3. REPLICA_ENTRY_ENDPOINT 指向 10.161.34.51:7151
4. 用户提交地址为 10.161.34.22:53
5. 链上最终地址为 10.161.34.22:5353
```

更详细的实验四说明见：

```text
deploy/swarm/EXPERIMENT_4_REPLICA_PREEMPTION_RUNBOOK.md
```
