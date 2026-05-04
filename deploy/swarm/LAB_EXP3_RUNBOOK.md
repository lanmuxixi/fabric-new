# 实验三：三服务器主节点 PoW 防御实验

本文档对应分支 `feature/swarm-lab-exp3`，用于在三台实验室服务器上运行实验三。

## 1. 实验定位

实验三是主节点防御实验，对应历史方向 `v0.6-primarybz-pow`。

实验逻辑：

```text
正常用户提交带 PoW nonce 的 TopLevelUpdate
恶意主节点如果篡改 authority，需要为篡改后的内容重新计算 PoW
链码侧校验 PoW
```

默认配置：

```bash
VP0_BYZANTINE=true
DNS_BZAUTHORITY=10.161.34.22:5353
DNS_POW_TARGET=000000ffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
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

## 3. 拉取分支并准备配置

在 manager 节点 `roott-I620-G20` 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-lab-exp3
git pull origin feature/swarm-lab-exp3
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

## 6. 执行实验三

实验三的 `invoke-top-level-update.sh` 会自动计算 nonce：

```bash
bash scripts/swarm/invoke-top-level-update.sh example.org 10.161.34.22:53
bash scripts/swarm/query-top-level.sh example.org
```

预期根据实验三 runbook 判断：

```text
正常请求具有合法 PoW
恶意主节点篡改后需要重新计算 PoW
链码会拒绝不满足 PoW 的 TopLevelUpdate
```

## 7. 成功标准

实验三成功需要满足：

```text
1. PBFT 网络连通 10 个节点
2. VP0_BYZANTINE=true 生效
3. invoke 脚本成功计算 nonce 并发起请求
4. 查询结果符合 PoW 防御预期
```

更详细的实验三说明见：

```text
deploy/swarm/EXPERIMENT_3_PRIMARY_DEFENSE_RUNBOOK.md
```
