# 实验二：三服务器主节点作恶抢注

本文档对应分支 `feature/swarm-lab-exp2`，用于在三台实验室服务器上运行实验二。

## 1. 节点分布

```text
roott-I620-G20:
  vp0, vp1, vp2, vp3

qichang-I420-G20:
  vp4, vp5, vp6

node21:
  vp7, vp8, vp9, bind9
```

实验二默认让 `vp0` 作为恶意主节点：

```bash
VP0_BYZANTINE=true
DNS_BZAUTHORITY=10.161.34.22:5353
```

## 2. 拉取分支并准备配置

在 manager 节点 `roott-I620-G20` 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-lab-exp2
git pull origin feature/swarm-lab-exp2
cp deploy/swarm/.env.lab.example deploy/swarm/.env
```

## 3. 构建并分发镜像

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

## 4. 部署 Stack 和链码

```bash
docker network create --driver overlay --attachable fabric_dns || true
bash scripts/swarm/deploy-stack.sh
docker stack services fabricdns
```

确认 10 个节点连通：

```bash
curl -s http://localhost:7050/network/peers | python3 -c \
  "import sys,json; peers=json.load(sys.stdin)['peers']; print(len(peers)); [print(p['ID']['name'], p['address']) for p in peers]"
```

部署链码：

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
cat deploy/swarm/last-chaincode-id.txt
```

## 5. 执行抢注实验

先查询目标顶级域：

```bash
bash scripts/swarm/query-top-level.sh example.org
```

发起用户看起来正常的顶级域注册：

```bash
bash scripts/swarm/invoke-top-level-update.sh example.org 10.161.34.22:53
```

查询链上结果：

```bash
bash scripts/swarm/query-top-level.sh example.org
```

预期结果：

```text
authorityServer = 10.161.34.22:5353
```

这说明请求被恶意主节点改写，恶意权威地址先落链。

## 6. 成功标准

实验二成功需要满足：

```text
1. PBFT 网络连通 10 个节点
2. VP0_BYZANTINE=true 生效
3. 用户提交地址为 10.161.34.22:53
4. 链上最终地址为 10.161.34.22:5353
```
