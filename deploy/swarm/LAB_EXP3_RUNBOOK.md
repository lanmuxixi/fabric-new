# 实验三：v0.6-primarybz-pow 的 Swarm 部署手册

本文档对应分支 `feature/swarm-lab-exp3-primarybz-pow`。本分支以 `v0.6-primarybz-pow` 为业务基线，只迁移 Docker Swarm 部署层，不重写 `TopLevelUpdate` 业务结构，也不替换原有主节点攻击防御逻辑。

## 1. 实验边界

实验三保留原分支逻辑：

- 用户侧仍发送 6 参数 `TopLevelUpdate(domain, ip, type, ttl, owner, signature)` 请求。
- `vp0` 作为恶意主节点时，原代码会在 PBFT 层构造伪造交易。
- PoW 防御路径仍使用原代码中的 `makeTxByPow`，内部伪造请求会带上 `target` 和 `nonce` 字段。
- 本分支不增加新的 DNS 业务结构，不改链码，不改 PBFT 作恶逻辑。

原代码依据在：

```bash
consensus/pbft/batch.go
```

其中关键常量和函数包括：

```go
BYZANTINE_NAME   = "JIM"
BYZANTINE_IP     = "4.4.4.4"
BYZANTINE_FUNC   = "TopLevelUpdate"
BYZANTINE_TARGET = "000002ffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

makeTxByPow(...)
bzLeaderProcReq(...)
```

## 2. 两台服务器部署布局

本实验只使用两台机器部署 10 个 Fabric 节点，Bind9 放在第二台机器：

| 机器 | Swarm hostname | IP | 服务 |
|---|---|---|---|
| 服务器 1 | `roott-I620-G20` | `10.161.34.8` | `vp0` 到 `vp4` |
| 服务器 2 | `qichang-I420-G20` | `10.161.34.51` | `vp5` 到 `vp9`、`bind9` |

如果实际 `docker node ls` 中 hostname 不一致，必须先改 `deploy/swarm/.env.lab.example` 中的 `VP*_NODE` 和 `BIND_NODE`。

## 3. 拉取代码

在 Swarm manager 服务器上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-lab-exp3-primarybz-pow
git pull origin feature/swarm-lab-exp3-primarybz-pow
```

准备环境文件：

```bash
cp deploy/swarm/.env.lab.example deploy/swarm/.env
```

关键配置应保持如下：

```bash
FABRIC_PEER_IMAGE=fabric-dns-peer:swarm-exp3
CORE_PBFT_GENERAL_N=10
CORE_PBFT_GENERAL_F=3
CORE_PBFT_GENERAL_BATCHSIZE=500
CORE_PBFT_GENERAL_TIMEOUT_BATCH=1s
CORE_PBFT_GENERAL_TIMEOUT_REQUEST=24h
CORE_PBFT_GENERAL_TIMEOUT_VIEWCHANGE=24h
CORE_PBFT_GENERAL_VIEWCHANGEPERIOD=0
VP0_BYZANTINE=true
VP0_ENDPOINT=10.161.34.8:7051
CHAINCODE_CTOR={"Function":"init","Args":["com:10.161.34.51:53","cn:10.161.34.51:53"]}
```

## 4. 准备镜像

在可构建 Fabric 镜像的机器上执行：

```bash
bash scripts/swarm/build-peer-image.sh
docker save fabric-dns-peer:swarm-exp3 -o /tmp/fabric-dns-peer-swarm-exp3.tar
```

把 `/tmp/fabric-dns-peer-swarm-exp3.tar` 拷贝到两台服务器，并分别导入：

```bash
docker load -i /tmp/fabric-dns-peer-swarm-exp3.tar
docker images | grep fabric-dns-peer
```

两台机器都必须能看到 `fabric-dns-peer:swarm-exp3`。

## 5. 准备 Bind9

在 manager 上执行：

```bash
bash scripts/swarm/prepare-bind-layout.sh
```

该脚本会把 `deploy/swarm/bind` 下的配置复制到 `.env` 中指定的 Bind9 目录。默认 Bind9 目录是：

```bash
/opt/fabric-dns/bind/config
/opt/fabric-dns/bind/cache
/opt/fabric-dns/bind/records
```

## 6. 启动 Stack

在 manager 上执行：

```bash
bash scripts/swarm/deploy-stack.sh
docker stack services fabricdns
docker stack ps fabricdns
```

检查点：

```bash
docker service ls
docker service ps fabricdns_vp0
docker service ps fabricdns_vp9
docker service ps fabricdns_bind9
```

10 个 `vp` 服务和 1 个 `bind9` 服务都应处于运行状态。

## 7. 部署 DNS 链码并回填链码 ID

首次启动后部署 DNS 链码：

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
```

脚本会把真实链码 ID 写回：

```bash
deploy/swarm/.env
```

确认：

```bash
grep DNS_CHAINCODEID deploy/swarm/.env
```

然后重新部署 stack，让 `vp0` 容器拿到 `CORE_DNS_CHAINCODEID`：

```bash
bash scripts/swarm/deploy-stack.sh
```

这是实验三必须步骤。原始 `v0.6-primarybz-pow` 的恶意主节点逻辑会在 `vp0` 容器内部读取该值，再构造带 PoW 字段的伪造 `TopLevelUpdate` 交易。

## 8. 注册用户并做功能检查

注册测试用户：

```bash
bash scripts/swarm/register-users.sh
```

查询初始化顶级域：

```bash
bash scripts/swarm/query-top-level.sh com
bash scripts/swarm/query-top-level.sh cn
```

发起普通顶级域更新请求：

```bash
bash scripts/swarm/invoke-top-level-update.sh example.org 10.161.34.51:53 A 3600 USER SIG
bash scripts/swarm/query-top-level.sh example.org
```

注意：脚本发送的是普通 6 参数用户请求。恶意主节点内部是否改写请求、是否带 PoW 字段，由 `v0.6-primarybz-pow` 原代码决定，不在脚本中伪造 8 参数请求。

## 9. 成功标准

实验三部署成功至少满足：

- `docker stack services fabricdns` 中 10 个 peer 服务都稳定运行。
- `vp0` 环境变量中 `CORE_PBFT_GENERAL_BYZANTINE=true`。
- `vp0` 环境变量中 `CORE_DNS_CHAINCODEID` 不为空。
- `query-top-level.sh` 能查询链码状态。
- 普通 `TopLevelUpdate` 请求能被提交并出块。
- 日志能看到原始 `v0.6-primarybz-pow` 的主节点作恶/PoW 防御路径，而不是脚本层重写的业务请求。

## 10. 常见问题

如果服务一直 `Pending`，先检查 Swarm hostname：

```bash
docker node ls
```

然后修正 `.env` 中的 `VP*_NODE` 和 `BIND_NODE`。

如果 `vp0` 没有触发原始实验三逻辑，检查：

```bash
docker service inspect fabricdns_vp0 --format '{{json .Spec.TaskTemplate.ContainerSpec.Env}}'
```

重点确认：

```bash
CORE_PBFT_GENERAL_BYZANTINE=true
CORE_DNS_CHAINCODEID=<非空链码ID>
```

如果出现 Swarm 容器 IP 漂移导致 PBFT 节点互联异常，确认 `deploy/swarm/stack.yml` 中保持：

```yaml
CORE_PEER_ADDRESSAUTODETECT: "false"
CORE_PEER_ADDRESS: vpX:7051
CORE_PEER_DISCOVERY_ROOTNODE: vp0:7051
```

这会让节点之间使用 Swarm 服务名通信，而不是缓存容易漂移的容器 overlay IP。
