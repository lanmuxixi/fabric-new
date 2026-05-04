# 三服务器 10 节点实验部署步骤

本文档用于把当前 Fabric DNS 实验从云平台迁移到学校实验室三台全新服务器上运行。三台服务器当前假设尚未加入 Docker Swarm，也没有现成 Docker 镜像、Bind9 目录或 Fabric 运行环境。

## 1. 服务器规划

服务器信息：

```text
node7        10.161.34.8
wdb服务器2   10.161.34.51
node21       10.161.34.22
```

建议角色：

```text
node7:
  Swarm manager
  vp0, vp1, vp2, vp3
  实验操作入口

wdb服务器2:
  Swarm worker
  vp4, vp5, vp6
  实验四恶意副本入口 vp4

node21:
  Swarm worker
  vp7, vp8, vp9
  bind9 正常权威 DNS
```

关键地址：

```bash
CORE_DNS_SUBNET=10.161.34.0/24
VP0_ENDPOINT=10.161.34.8:7051
REPLICA_ENTRY_ENDPOINT=10.161.34.51:7151
NORMAL_DNS_AUTHORITY=10.161.34.22:53
```

## 2. 三台服务器安装基础软件

以下命令三台服务器都执行：

```bash
sudo apt update
sudo apt install -y docker.io git curl python3 dnsutils
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
```

执行完 `usermod` 后，重新登录当前服务器，再确认 Docker 可用：

```bash
docker version
docker ps
```

如果服务器系统不是 Ubuntu/Debian，需要按实际系统安装 Docker、Git、Python3 和 `dig` 工具。

## 3. 初始化 Docker Swarm

在 `node7` 上初始化 Swarm：

```bash
docker swarm init --advertise-addr 10.161.34.8
```

命令会输出一条 worker 加入命令，形如：

```bash
docker swarm join --token <worker-token> 10.161.34.8:2377
```

在 `wdb服务器2` 和 `node21` 上执行这条 `docker swarm join` 命令。

回到 `node7`，确认三台机器已经加入：

```bash
docker node ls
```

必须记录 `HOSTNAME` 列中的真实节点名。后续 `VP*_NODE` 和 `BIND_NODE` 必须使用 `docker node ls` 看到的名称。

如果第二台服务器显示中文 hostname，建议改成 ASCII 名称，例如：

```bash
sudo hostnamectl set-hostname wdb-server-2
```

改名后需要重新登录，并确认：

```bash
hostname
docker node ls
```

## 4. 拉取项目代码

建议在 `node7` 作为主操作机：

```bash
mkdir -p /root/go/src/github.com/hyperledger
cd /root/go/src/github.com/hyperledger
git clone <你的 GitHub 仓库地址> fabric
cd fabric
```

实验一先切到基础 Swarm 分支：

```bash
git checkout feature/swarm-adaptation
git pull origin feature/swarm-adaptation
```

后续实验二、三、四再分别切换对应分支。

## 5. 构建并分发 peer 镜像

在 `node7` 上构建 peer 镜像：

```bash
cd /root/go/src/github.com/hyperledger/fabric
bash scripts/swarm/build-peer-image.sh
docker images | grep fabric-dns-peer
```

如果没有镜像仓库，使用 `docker save / scp / docker load` 分发镜像。

在 `node7` 上导出：

```bash
docker save fabric-dns-peer:swarm -o /tmp/fabric-dns-peer-swarm.tar
scp /tmp/fabric-dns-peer-swarm.tar root@10.161.34.51:/tmp/
scp /tmp/fabric-dns-peer-swarm.tar root@10.161.34.22:/tmp/
```

在 `wdb服务器2` 和 `node21` 上导入：

```bash
docker load -i /tmp/fabric-dns-peer-swarm.tar
docker images | grep fabric-dns-peer
```

## 6. 创建 Swarm overlay 网络

在 `node7` 上执行：

```bash
docker network create --driver overlay --attachable fabric_dns
docker network ls | grep fabric_dns
```

如果网络已经存在，可以跳过创建。

## 7. 准备 Bind9 目录

在 `node21` 上执行：

```bash
sudo mkdir -p /opt/fabric-dns/bind/config
sudo mkdir -p /opt/fabric-dns/bind/cache
sudo mkdir -p /opt/fabric-dns/bind/records
sudo chmod -R 777 /opt/fabric-dns/bind
```

在 `node7` 上执行项目脚本准备 Bind9 配置：

```bash
cd /root/go/src/github.com/hyperledger/fabric
bash scripts/swarm/prepare-bind-layout.sh
```

如果该脚本要求本地路径存在，则需要把生成后的 Bind9 配置同步到 `node21` 的 `/opt/fabric-dns/bind`。

## 8. 准备实验室环境变量

在 `node7` 的项目目录中创建实验室环境文件：

```bash
cd /root/go/src/github.com/hyperledger/fabric
cp deploy/swarm/.env.example deploy/swarm/.env
```

编辑 `deploy/swarm/.env`，按 `docker node ls` 的真实 hostname 修改。示例：

```bash
STACK_NAME=fabricdns
SWARM_NETWORK=fabric_dns
FABRIC_PEER_IMAGE=fabric-dns-peer:swarm
BIND_IMAGE=internetsystemsconsortium/bind9:9.18

CORE_PEER_NETWORKID=dev
CORE_LOGGING_LEVEL=info
PEER_START_DELAY=10
CORE_PBFT_GENERAL_N=10
CORE_PBFT_GENERAL_F=3
CORE_DNS_SUBNET=10.161.34.0/24

VP0_NODE=node7
VP1_NODE=node7
VP2_NODE=node7
VP3_NODE=node7

VP4_NODE=wdb-server-2
VP5_NODE=wdb-server-2
VP6_NODE=wdb-server-2

VP7_NODE=node21
VP8_NODE=node21
VP9_NODE=node21
BIND_NODE=node21

VP0_BYZANTINE=false
VP1_BYZANTINE=false
VP2_BYZANTINE=false
VP3_BYZANTINE=false
VP4_BYZANTINE=false
VP5_BYZANTINE=false
VP6_BYZANTINE=false
VP7_BYZANTINE=false
VP8_BYZANTINE=false
VP9_BYZANTINE=false

VP0_REST_PORT=7050
VP0_GRPC_PORT=7051
VP0_EVENTS_PORT=7053
VP0_PROFILE_PORT=6060
VP4_GRPC_PORT=7151

VP0_ENDPOINT=10.161.34.8:7051
REPLICA_ENTRY_ENDPOINT=10.161.34.51:7151

BIND_CONFIG_DIR=/opt/fabric-dns/bind/config
BIND_CACHE_DIR=/opt/fabric-dns/bind/cache
BIND_RECORDS_DIR=/opt/fabric-dns/bind/records
BIND_TCP_PORT=53
BIND_UDP_PORT=53

CHAINCODE_CTOR={"Function":"init","Args":["com:10.161.34.22:53","cn:10.161.34.22:53"]}
```

注意：`VP4_NODE=wdb-server-2` 必须替换成 `docker node ls` 中第二台服务器的真实 hostname。

## 9. 实验一：基础 DNS 解析实验

实验一分支：

```bash
feature/swarm-adaptation
```

在 `node7` 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git checkout feature/swarm-adaptation
git pull origin feature/swarm-adaptation
bash scripts/swarm/deploy-stack.sh
```

确认服务状态：

```bash
docker stack services fabricdns
```

预期所有服务都是 `1/1`。

确认 PBFT 网络连通：

```bash
curl -s http://localhost:7050/network/peers | python3 -c \
  "import sys,json; peers=json.load(sys.stdin)['peers']; print(len(peers)); [print(p['ID']['name'], p['address']) for p in peers]"
```

预期输出节点数为 `10`。

部署 DNS 链码：

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
cat deploy/swarm/last-chaincode-id.txt
```

查询顶级域映射：

```bash
bash scripts/swarm/query-top-levels.sh
```

预期包含：

```text
com -> 10.161.34.22:53
cn  -> 10.161.34.22:53
```

写入普通域名记录：

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
dig @10.161.34.22 www.example.com +short
```

预期返回：

```text
10.161.34.8
```

## 10. 实验二：主节点作恶抢注

实验二分支：

```bash
feature/swarm-exp2-preemption
```

切换分支并更新代码：

```bash
git checkout feature/swarm-exp2-preemption
git pull origin feature/swarm-exp2-preemption
bash scripts/swarm/build-peer-image.sh
```

将新镜像分发到三台服务器后，修改 `deploy/swarm/.env`：

```bash
VP0_BYZANTINE=true
VP4_BYZANTINE=false
VP8_BYZANTINE=false
DNS_BZAUTHORITY=10.161.34.22:5353
```

重新部署 stack：

```bash
bash scripts/swarm/deploy-stack.sh
```

部署链码后执行实验二脚本：

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
bash scripts/swarm/invoke-top-level-update.sh example.org 10.161.34.22:53
bash scripts/swarm/query-top-level.sh example.org
```

预期链上 `org` 的权威地址变为 `DNS_BZAUTHORITY`。

## 11. 实验三：主节点防御实验

实验三分支：

```bash
feature/swarm-exp3-primary-defense
```

切换分支：

```bash
git checkout feature/swarm-exp3-primary-defense
git pull origin feature/swarm-exp3-primary-defense
bash scripts/swarm/build-peer-image.sh
```

分发镜像后，保留 `VP0_BYZANTINE=true`，并按实验三 runbook 设置 PoW 参数。

执行：

```bash
bash scripts/swarm/deploy-stack.sh
bash scripts/swarm/deploy-dns-chaincode.sh
bash scripts/swarm/invoke-top-level-update.sh example.org 10.161.34.22:53
bash scripts/swarm/query-top-level.sh example.org
```

预期：正常请求带合法 PoW，恶意主节点篡改后需要重新计算 PoW，实验现象以实验三 runbook 为准。

## 12. 实验四：副本节点作恶抢注

实验四分支：

```bash
feature/swarm-exp4-replica-preemption
```

切换分支：

```bash
git checkout feature/swarm-exp4-replica-preemption
git pull origin feature/swarm-exp4-replica-preemption
bash scripts/swarm/build-peer-image.sh
```

分发镜像后，修改 `deploy/swarm/.env`：

```bash
VP0_BYZANTINE=false
VP4_BYZANTINE=true
VP8_BYZANTINE=false
DNS_BZAUTHORITY=10.161.34.22:5353
REPLICA_ENTRY_ENDPOINT=10.161.34.51:7151
```

重新部署 stack：

```bash
bash scripts/swarm/deploy-stack.sh
bash scripts/swarm/deploy-dns-chaincode.sh
```

从恶意副本入口发起正常注册：

```bash
bash scripts/swarm/invoke-top-level-update-replica.sh example.org 10.161.34.22:53
bash scripts/swarm/query-top-level.sh example.org
```

预期链上 `org` 的权威地址变为 `DNS_BZAUTHORITY`。

## 13. 常见问题排查

服务没有调度到指定服务器：

```bash
docker node ls
docker service ps fabricdns_vp4
```

检查 `.env` 中的 `VP*_NODE` 是否和 `docker node ls` 的 hostname 完全一致。

节点没有形成 10 节点网络：

```bash
docker stack services fabricdns
docker service logs fabricdns_vp0 --tail 100
curl -s http://localhost:7050/network/peers
```

如果某些节点启动过早，可以强制重启：

```bash
for vp in vp1 vp2 vp3 vp4 vp5 vp6 vp7 vp8 vp9; do
  docker service update --force fabricdns_${vp}
done
```

链码部署后查询不到数据：

```bash
cat deploy/swarm/last-chaincode-id.txt
bash scripts/swarm/query-top-levels.sh
```

如果返回 `ledger: resource not found`，优先检查 PBFT 是否已经形成 10 节点网络。

Bind9 没有响应：

```bash
docker service ps fabricdns_bind9
docker service logs fabricdns_bind9 --tail 100
dig @10.161.34.22 www.example.com +short
```

检查 `node21` 的 53/tcp 和 53/udp 是否被防火墙放行。

## 14. 实验推进顺序

必须先跑通实验一，再迁移后续实验：

```text
1. feature/swarm-adaptation
2. feature/swarm-exp2-preemption
3. feature/swarm-exp3-primary-defense
4. feature/swarm-exp4-replica-preemption
```

实验一跑通后，后续实验主要复用同一套三服务器 Swarm、Bind9 目录、overlay 网络和 `.env` 节点布局。
