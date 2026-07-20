# 实验一：30 节点两服务器 Swarm 部署步骤

本文档对应分支 `feature/swarm-lab-exp1`，用于在学校实验室两台服务器上运行实验一 30 节点基础 DNS 解析性能实验。实验一的物理机器布局已与实验三对齐，所有 PBFT 节点均为诚实节点。

## 1. 当前 Swarm 节点

用户已经完成 Docker Swarm 加入，`docker node ls` 输出中的节点名为：

```text
roott-I620-G20     manager，建议对应 node7 / 10.161.34.8
qichang-I420-G20   worker，建议对应 wdb服务器2 / 10.161.34.51
```

实验一节点分布：

```text
roott-I620-G20:
  vp0-vp14

qichang-I420-G20:
  vp15-vp29, bind9
```

## 2. 拉取实验一分支

在 manager 节点 `roott-I620-G20` 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-lab-exp1
git pull origin feature/swarm-lab-exp1
```

如果服务器上还没有仓库：

```bash
mkdir -p /root/go/src/github.com/hyperledger
cd /root/go/src/github.com/hyperledger
git clone <你的 GitHub 仓库地址> fabric
cd fabric
git checkout feature/swarm-lab-exp1
```

## 3. 准备环境变量

在 manager 节点执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
cp deploy/swarm/.env.lab.example deploy/swarm/.env
```

确认 `.env` 中关键配置：

```bash
CORE_DNS_SUBNET=10.161.34.0/24
CORE_PBFT_GENERAL_N=30
CORE_PBFT_GENERAL_F=9
VP0_NODE=roott-I620-G20
VP14_NODE=roott-I620-G20
VP15_NODE=qichang-I420-G20
VP29_NODE=qichang-I420-G20
BIND_NODE=qichang-I420-G20
VP0_ENDPOINT=10.161.34.8:7051
CHAINCODE_CTOR={"Function":"init","Args":["com:10.161.34.51:53","cn:10.161.34.51:53"]}
```

`CORE_PBFT_GENERAL_F=9` 表示 30 节点 PBFT 配置最多可容忍 9 个拜占庭节点，但实验一仍然是诚实基线，`.env` 中 `VP0_BYZANTINE` 到 `VP29_BYZANTINE` 都应保持 `false`。

如果 `qichang-I420-G20` 的真实 IP 不是 `10.161.34.51`，需要同步调整 `.env` 中的 `CHAINCODE_CTOR` 以及 Bind9 zone 文件中的 `ns1` 地址。

## 4. 构建并分发 peer 镜像

在 manager 节点构建镜像：

```bash
cd /root/go/src/github.com/hyperledger/fabric
bash scripts/swarm/build-peer-image.sh
docker images | grep fabric-dns-peer
```

把镜像分发到 worker：

```bash
docker save fabric-dns-peer:swarm -o /tmp/fabric-dns-peer-swarm.tar
scp /tmp/fabric-dns-peer-swarm.tar root@10.161.34.51:/tmp/
```

在 `10.161.34.51` 上导入：

```bash
docker load -i /tmp/fabric-dns-peer-swarm.tar
docker images | grep fabric-dns-peer
```

## 5. 创建 overlay 网络

在 manager 节点执行：

```bash
docker network create --driver overlay --attachable fabric_dns || true
docker network ls | grep fabric_dns
```

## 6. 准备 Bind9 目录

在 `qichang-I420-G20 / 10.161.34.51` 上执行：

```bash
sudo mkdir -p /opt/fabric-dns/bind/config
sudo mkdir -p /opt/fabric-dns/bind/cache
sudo mkdir -p /opt/fabric-dns/bind/records
sudo chmod -R 777 /opt/fabric-dns/bind
```

在 manager 节点执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
bash scripts/swarm/prepare-bind-layout.sh
```

如果脚本只在 manager 本地生成 Bind9 配置，需要把 `/opt/fabric-dns/bind` 同步到 `10.161.34.51`。

## 7. 部署 Stack

在 manager 节点执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
bash scripts/swarm/deploy-stack.sh
```

检查服务：

```bash
docker stack services fabricdns
```

预期 `vp0` 到 `vp29` 以及 `bind9` 共 31 个服务都为 `1/1`。

## 8. 检查 30 节点 PBFT 网络

在 manager 节点执行：

```bash
curl -s http://localhost:7050/network/peers | python3 -c \
  "import sys,json; peers=json.load(sys.stdin)['peers']; print(len(peers)); [print(p['ID']['name'], p['address']) for p in peers]"
```

预期输出节点数为 `30`。

如果不足 30 个节点，先强制重启非 vp0 节点：

```bash
for i in $(seq 1 29); do
  vp="vp${i}"
  docker service update --force fabricdns_${vp}
done
```

等待 60 秒后再次查询。

## 9. 部署 DNS 链码

PBFT 网络确认 30 节点后再部署链码：

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
cat deploy/swarm/last-chaincode-id.txt
```

查询顶级域初始化结果：

```bash
bash scripts/swarm/query-top-levels.sh
```

预期包含：

```text
com -> 10.161.34.51:53
cn  -> 10.161.34.51:53
```

## 10. 验证普通域名写入和解析

写入一条普通域名记录：

```bash
CHAINCODE_ID=$(cat deploy/swarm/last-chaincode-id.txt)
docker run --rm \
  -e CORE_PEER_ADDRESS="10.161.34.8:7051" \
  fabric-dns-peer:swarm \
  peer chaincode invoke \
    -n "${CHAINCODE_ID}" \
    -c '{"Function":"update","Args":["www.example.com","10.161.34.8"]}'
```

用 Bind9 验证：

```bash
dig @10.161.34.51 www.example.com +short
```

预期返回：

```text
10.161.34.8
```

## 11. 实验一成功标准

实验一成功必须同时满足：

```text
1. docker stack services fabricdns 全部 1/1
2. /network/peers 返回 30 个节点
3. deploy-dns-chaincode.sh 成功生成链码 ID
4. query-top-levels.sh 返回 com/cn 到 10.161.34.51:53
5. update www.example.com 后，dig @10.161.34.51 返回写入 IP
```

实验一跑通后，再迁移实验二、实验三、实验四。
