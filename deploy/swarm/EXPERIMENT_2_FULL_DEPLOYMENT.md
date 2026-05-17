# 实验二完整部署文档：主节点抢注，无 PoW

本文档用于在实验室两台服务器上部署并跑通实验二。实验二对应分支为 `feature/swarm-lab-exp2-primarybz-nopow`，业务基线为 `v0.6-primarybz-nopow`。当前分支只迁移 Docker Swarm 部署层，不重写 DNS 链码，不替换原始主节点抢注逻辑。

## 1. 实验目标和边界

实验二验证的是“恶意主节点抢注”。部署时需要保证 `vp0` 是 PBFT 主节点，并且 `vp0` 开启 byzantine。原始业务逻辑仍然来自 `v0.6-primarybz-nopow`：

- 用户请求仍是 6 参数 `TopLevelUpdate(domain, ip, type, ttl, owner, signature)`。
- 恶意主节点逻辑仍在 `consensus/pbft/batch.go` 中。
- 恶意身份仍是原代码中的 `JIM`。
- 恶意 IP 仍是原代码中的 `4.4.4.4`。
- 不使用两参数版 `TopLevelUpdate(domain, authorityServer)`。

本实验的 Swarm 配置只负责让 10 个 Fabric peer 和 Bind9 在两台服务器上稳定运行。

## 2. 服务器规划

当前只使用两台服务器部署 10 个 Fabric peer：

| 机器 | IP | Swarm hostname | 部署服务 |
|---|---|---|---|
| 服务器 1 | `10.161.34.8` | `roott-I620-G20` | `vp0` 到 `vp4` |
| 服务器 2 | `10.161.34.51` | `qichang-I420-G20` | `vp5` 到 `vp9`、`bind9` |

在 Swarm manager 上确认 hostname：

```bash
docker node ls
```

如果实际 hostname 不一致，必须修改 `deploy/swarm/.env` 中的 `VP*_NODE` 和 `BIND_NODE`。

## 3. 前置条件

三项前置条件必须满足：

- 两台服务器已经加入同一个 Docker Swarm 集群。
- 两台服务器都已经安装 Docker，并能运行 `docker stack`。
- 两台服务器离线环境中已经准备好需要的 Docker 镜像。

至少需要这些镜像：

```bash
fabric-dns-peer:swarm
internetsystemsconsortium/bind9:9.18
```

如果实验室服务器不能访问 Docker Hub，需要在 Windows 或其他可联网机器上提前下载并打包镜像，然后复制到两台服务器。

## 4. 拉取实验二分支

在 Swarm manager 服务器 `10.161.34.8` 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-lab-exp2-primarybz-nopow
git pull origin feature/swarm-lab-exp2-primarybz-nopow
```

准备环境文件：

```bash
cp deploy/swarm/.env.lab.example deploy/swarm/.env
```

## 5. 检查关键配置

打开 `deploy/swarm/.env`，确认以下关键配置：

```bash
STACK_NAME=fabricdns
SWARM_NETWORK=fabric_dns
FABRIC_PEER_IMAGE=fabric-dns-peer:swarm

CORE_PBFT_GENERAL_N=10
CORE_PBFT_GENERAL_F=3
CORE_PBFT_GENERAL_BATCHSIZE=500
CORE_PBFT_GENERAL_TIMEOUT_BATCH=1s
CORE_PBFT_GENERAL_TIMEOUT_REQUEST=24h
CORE_PBFT_GENERAL_TIMEOUT_VIEWCHANGE=24h
CORE_PBFT_GENERAL_TIMEOUT_RESENDVIEWCHANGE=24h
CORE_PBFT_GENERAL_VIEWCHANGEPERIOD=0

VP0_NODE=roott-I620-G20
VP1_NODE=roott-I620-G20
VP2_NODE=roott-I620-G20
VP3_NODE=roott-I620-G20
VP4_NODE=roott-I620-G20
VP5_NODE=qichang-I420-G20
VP6_NODE=qichang-I420-G20
VP7_NODE=qichang-I420-G20
VP8_NODE=qichang-I420-G20
VP9_NODE=qichang-I420-G20
BIND_NODE=qichang-I420-G20

VP0_BYZANTINE=true
VP1_BYZANTINE=false
VP2_BYZANTINE=false
VP3_BYZANTINE=false
VP4_BYZANTINE=false
VP5_BYZANTINE=false
VP6_BYZANTINE=false
VP7_BYZANTINE=false
VP8_BYZANTINE=false
VP9_BYZANTINE=false

VP0_ENDPOINT=10.161.34.8:7051
CHAINCODE_CTOR={"Function":"init","Args":["com:10.161.34.51:53","cn:10.161.34.51:53"]}
```

`DNS_CHAINCODEID` 初始为空，这是正常状态。部署链码后脚本会自动写回。

## 6. 准备离线镜像

如果 manager 可以构建镜像，在 manager 上执行：

```bash
bash scripts/swarm/build-peer-image.sh
```

生成的镜像标签应为：

```bash
fabric-dns-peer:swarm
```

如果需要离线分发，在有镜像的机器上打包：

```bash
docker save fabric-dns-peer:swarm -o /tmp/fabric-dns-peer-swarm.tar
docker save internetsystemsconsortium/bind9:9.18 -o /tmp/bind9-9.18.tar
```

复制到两台服务器后分别导入：

```bash
docker load -i /tmp/fabric-dns-peer-swarm.tar
docker load -i /tmp/bind9-9.18.tar
docker images | grep -E 'fabric-dns-peer|bind9'
```

两台服务器都必须能看到 `fabric-dns-peer:swarm`。`bind9:9.18` 至少需要在 `qichang-I420-G20` 上存在。

## 7. 清理旧实验环境

切换实验或重跑实验前，先删除旧 stack：

```bash
bash scripts/swarm/remove-stack.sh
```

等待服务完全删除：

```bash
docker stack ps fabricdns
docker service ls
```

如果需要彻底清理旧账本和旧链码状态，再删除本实验 stack 的 volume：

```bash
docker volume ls | grep '^local.*fabricdns_'
docker volume ls -q | grep '^fabricdns_' | xargs -r docker volume rm
```

注意：删除 volume 会清空旧账本数据。只有在重新开始实验时执行。

## 8. 准备 Bind9 目录

在 manager 上执行：

```bash
bash scripts/swarm/prepare-bind-layout.sh
```

默认会准备这些目录：

```bash
/opt/fabric-dns/bind/config
/opt/fabric-dns/bind/cache
/opt/fabric-dns/bind/records
```

检查：

```bash
ls -R /opt/fabric-dns/bind
```

如果 `BIND_NODE=qichang-I420-G20`，需要确保这些目录实际存在于 `10.161.34.51`。如果脚本只在 manager 本机执行，需手动把 `deploy/swarm/bind` 下的配置同步到 `10.161.34.51:/opt/fabric-dns/bind`。

## 9. 启动 Swarm Stack

在 manager 上执行：

```bash
bash scripts/swarm/deploy-stack.sh
```

检查服务：

```bash
docker stack services fabricdns
docker stack ps fabricdns
```

重点确认：

- `fabricdns_vp0` 到 `fabricdns_vp9` 都处于运行状态。
- `fabricdns_bind9` 处于运行状态。
- 没有服务长时间停留在 `Pending`。

如果服务 `Pending`，优先检查 `.env` 中的 hostname 是否和 `docker node ls` 完全一致。

## 10. 检查 Swarm 地址配置

本实验必须关闭 Fabric 的地址自动探测，避免 Swarm 容器 IP 漂移导致 PBFT 地址缓存失效。确认 `deploy/swarm/stack.yml` 中有：

```yaml
CORE_PEER_ADDRESSAUTODETECT: "false"
CORE_PEER_ADDRESS: vpX:7051
CORE_PEER_DISCOVERY_ROOTNODE: vp0:7051
```

部署后可检查 `vp0` 环境变量：

```bash
docker service inspect fabricdns_vp0 --format '{{json .Spec.TaskTemplate.ContainerSpec.Env}}'
```

需要看到：

```bash
CORE_PEER_ADDRESSAUTODETECT=false
CORE_PEER_ADDRESS=vp0:7051
CORE_PBFT_GENERAL_BYZANTINE=true
```

## 11. 部署 DNS 链码

首次启动 peer 后，部署 DNS 链码：

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
```

脚本会把链码 ID 写入：

```bash
deploy/swarm/last-chaincode-id.txt
deploy/swarm/.env
```

检查：

```bash
cat deploy/swarm/last-chaincode-id.txt
grep '^DNS_CHAINCODEID=' deploy/swarm/.env
```

如果 `DNS_CHAINCODEID` 为空，说明链码部署失败或脚本没有解析到链码 ID，需要查看：

```bash
cat deploy/swarm/last-chaincode-deploy.log
```

## 12. 重新下发 Stack 配置

部署链码后必须重新部署 stack：

```bash
bash scripts/swarm/deploy-stack.sh
```

原因：原始实验二代码中，恶意主节点 `vp0` 会在容器内部读取 `CORE_DNS_CHAINCODEID`，再发起抢注交易。如果不重新部署，`vp0` 拿不到链码 ID，抢注逻辑可能不触发。

重新部署后检查：

```bash
docker service inspect fabricdns_vp0 --format '{{json .Spec.TaskTemplate.ContainerSpec.Env}}' | grep CORE_DNS_CHAINCODEID
```

## 13. 注册测试用户

执行：

```bash
bash scripts/swarm/register-users.sh
```

原链码 `init` 已经内置部分用户，但性能脚本可能依赖额外用户。重跑注册脚本通常是安全的。

## 14. 基础功能验证

查询初始化顶级域：

```bash
bash scripts/swarm/query-top-level.sh com
bash scripts/swarm/query-top-level.sh cn
```

发起一次普通顶级域更新请求：

```bash
bash scripts/swarm/invoke-top-level-update.sh example.org 10.161.34.51:53 A 3600 USER SIG
```

再查询：

```bash
bash scripts/swarm/query-top-level.sh example.org
```

说明：上面的 `USER` 和 `SIG` 只是命令格式示例。严格验证业务时，应使用原实验脚本中已有的用户和签名数据，避免因为签名不匹配导致链码拒绝。

## 15. 验证主节点抢注逻辑

实验二的关键不是只看普通更新是否成功，而是看恶意主节点是否按原逻辑发起抢注。检查点：

```bash
docker service logs fabricdns_vp0 --tail 200
```

重点看是否出现与 `JIM`、`4.4.4.4`、`TopLevelUpdate`、`dns.chaincodeid` 相关的日志。

也可以查询目标域最终链上状态：

```bash
bash scripts/swarm/query-top-level.sh <domain>
```

如果用户提交的是正常地址，但最终链上出现恶意地址，说明抢注路径触发。

## 16. 运行性能实验

运行性能脚本前，先导出链码 ID：

```bash
export zzm="$(cat deploy/swarm/last-chaincode-id.txt)"
```

然后运行原实验脚本或压测工具。需要保证压测请求仍是原始 6 参数 `TopLevelUpdate`，不要使用两参数版请求。

如果使用外部 `perf-test.py`，确认：

- 目标 peer REST 地址为 `http://10.161.34.8:7050`。
- chaincode ID 使用 `deploy/swarm/last-chaincode-id.txt` 中的值。
- 请求 payload 保持原实验格式。
- 不要在压测过程中执行 `docker service update --force`，否则会重启容器并影响实验。

## 17. 成功标准

实验二部署成功至少满足：

- `docker stack services fabricdns` 中 10 个 peer 和 1 个 Bind9 服务正常运行。
- `vp0` 环境变量中 `CORE_PBFT_GENERAL_BYZANTINE=true`。
- `vp0` 环境变量中 `CORE_DNS_CHAINCODEID` 不为空。
- `/network/peers` 能看到 10 个 peer。
- 链码查询脚本可以查询 `com`、`cn` 或测试域名。
- 普通 6 参数 `TopLevelUpdate` 能提交并出块。
- 恶意主节点抢注路径能触发。
- 性能实验中区块数随 batch 产生，而不是所有交易被憋成 1 个超大块。

## 18. 常见故障和处理

### 18.1 服务 Pending

原因通常是 placement hostname 不匹配。执行：

```bash
docker node ls
```

然后修改 `.env` 中的：

```bash
VP0_NODE=...
BIND_NODE=...
```

### 18.2 服务器不能拉镜像

这是实验室离线环境的正常问题。需要在可联网机器上执行：

```bash
docker pull internetsystemsconsortium/bind9:9.18
docker save internetsystemsconsortium/bind9:9.18 -o bind9-9.18.tar
```

再复制到实验室服务器 `docker load`。

### 18.3 链码 ID 没有写入 `.env`

查看部署日志：

```bash
cat deploy/swarm/last-chaincode-deploy.log
```

如果日志里有 `Deploy chaincode:`，可以手动写入：

```bash
echo 'DNS_CHAINCODEID=<链码ID>' >> deploy/swarm/.env
bash scripts/swarm/deploy-stack.sh
```

### 18.4 `vp0` 没有触发抢注

优先检查：

```bash
docker service inspect fabricdns_vp0 --format '{{json .Spec.TaskTemplate.ContainerSpec.Env}}'
```

需要同时满足：

```bash
CORE_PBFT_GENERAL_BYZANTINE=true
CORE_DNS_CHAINCODEID=<非空链码ID>
```

还要确认当前镜像确实来自 `feature/swarm-lab-exp2-primarybz-nopow`，不是实验三或实验四镜像。

### 18.5 只产生 1 个大块

优先检查：

- 是否误用旧镜像。
- 是否误用两参数 `TopLevelUpdate`。
- `CORE_PBFT_GENERAL_BATCHSIZE` 是否为 `500`。
- `CORE_PBFT_GENERAL_TIMEOUT_BATCH` 是否为 `1s`。
- 压测期间是否重启过 Swarm 服务。
- `vp0` 是否拿到了正确的 `CORE_DNS_CHAINCODEID`。

### 18.6 PBFT 节点互联异常

确认没有开启地址自动探测：

```bash
docker service inspect fabricdns_vp1 --format '{{json .Spec.TaskTemplate.ContainerSpec.Env}}' | grep CORE_PEER_ADDRESSAUTODETECT
```

必须是：

```bash
CORE_PEER_ADDRESSAUTODETECT=false
```

## 19. 推荐执行顺序汇总

完整命令顺序如下：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-lab-exp2-primarybz-nopow
git pull origin feature/swarm-lab-exp2-primarybz-nopow

cp deploy/swarm/.env.lab.example deploy/swarm/.env

docker load -i /tmp/fabric-dns-peer-swarm.tar
docker load -i /tmp/bind9-9.18.tar

bash scripts/swarm/remove-stack.sh || true
bash scripts/swarm/prepare-bind-layout.sh
bash scripts/swarm/deploy-stack.sh

docker stack services fabricdns
docker stack ps fabricdns

bash scripts/swarm/deploy-dns-chaincode.sh
grep '^DNS_CHAINCODEID=' deploy/swarm/.env

bash scripts/swarm/deploy-stack.sh
bash scripts/swarm/register-users.sh

bash scripts/swarm/query-top-level.sh com
bash scripts/swarm/query-top-level.sh cn
```

确认以上步骤通过后，再开始正式性能压测。
