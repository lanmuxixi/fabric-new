# 实验二：v0.6-primarybz-nopow 的 Swarm 部署手册

本文档对应分支 `feature/swarm-lab-exp2-primarybz-nopow`。本分支以 `v0.6-primarybz-nopow` 为业务基线，只迁移 Docker Swarm 部署层，不重写 `TopLevelUpdate` 业务结构，也不替换原有主节点抢注逻辑。

## 1. 实验边界

保留原实验二逻辑：

- `TopLevelUpdate` 仍然使用 6 参数结构：`domain, ip, type, ttl, owner, signature`
- 恶意主节点仍然使用 `consensus/pbft/batch.go` 中已有的 `bzLeaderProcNVPReq` / `makeTxNoPow`
- 恶意身份仍然是原代码中的 `JIM`
- 恶意 IP 仍然是原代码中的 `4.4.4.4`
- 不使用后来简化版的 `TopLevelUpdate(domain, authorityServer)`

本分支只新增或修改：

- `deploy/swarm/*`
- `scripts/swarm/*`

## 2. 服务器节点规划

当前只使用两台服务器部署 10 个 peer 节点：

```text
10.161.34.8   roott-I620-G20      Swarm manager
10.161.34.51  qichang-I420-G20    Swarm worker
```

默认节点放置：

```text
roott-I620-G20:    vp0, vp1, vp2, vp3, vp4
qichang-I420-G20:  vp5, vp6, vp7, vp8, vp9, bind9
```

## 3. 拉取分支

在 manager 节点执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-lab-exp2-primarybz-nopow
git pull origin feature/swarm-lab-exp2-primarybz-nopow
cp deploy/swarm/.env.lab.example deploy/swarm/.env
```

## 4. 关键配置

确认 `deploy/swarm/.env`：

```bash
FABRIC_PEER_IMAGE=fabric-dns-peer:swarm
CORE_PBFT_GENERAL_N=10
CORE_PBFT_GENERAL_F=3
CORE_PBFT_GENERAL_BATCHSIZE=500
CORE_PBFT_GENERAL_TIMEOUT_BATCH=1s
CORE_PBFT_GENERAL_TIMEOUT_REQUEST=876000h
CORE_PBFT_GENERAL_TIMEOUT_VIEWCHANGE=876000h
CORE_PBFT_GENERAL_TIMEOUT_RESENDVIEWCHANGE=876000h
CORE_PBFT_GENERAL_VIEWCHANGEPERIOD=0
CORE_DNS_SUBNET=10.161.34.0/24
VP0_BYZANTINE=true
VP0_ENDPOINT=10.161.34.8:7051
BIND_NODE=qichang-I420-G20
CHAINCODE_CTOR={"Function":"init","Args":["com:10.161.34.51:53","cn:10.161.34.51:53"]}
```

`DNS_CHAINCODEID` 初始可以为空。部署链码后，`scripts/swarm/deploy-dns-chaincode.sh` 会自动写入该值。

## 5. 构建并分发镜像

在有构建环境的机器上执行：

```bash
bash scripts/swarm/build-peer-image.sh
```

然后把镜像分发到两台 Swarm 节点。离线环境可以使用 `docker save` / `docker load`：

```bash
docker save fabric-dns-peer:swarm -o /tmp/fabric-dns-peer-swarm.tar
```

每台服务器加载：

```bash
docker load -i /tmp/fabric-dns-peer-swarm.tar
docker images | grep fabric-dns-peer
```

## 6. 准备 Bind9

在 `qichang-I420-G20` 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git checkout feature/swarm-lab-exp2-primarybz-nopow
cp deploy/swarm/.env.lab.example deploy/swarm/.env
bash scripts/swarm/prepare-bind-layout.sh
```

检查：

```bash
ls -R /opt/fabric-dns/bind
```

## 7. 部署 Stack

在 manager 节点执行：

```bash
bash scripts/swarm/deploy-stack.sh
docker stack services fabricdns
```

确认 10 个 peer 正常：

```bash
curl -s http://localhost:7050/network/peers
```

## 8. 部署链码并回写链码 ID

部署链码：

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
cat deploy/swarm/last-chaincode-id.txt
grep '^DNS_CHAINCODEID=' deploy/swarm/.env
```

关键点：原 `v0.6-primarybz-nopow` 的恶意主节点会在 vp0 容器内部调用 `peer chaincode invoke` 发起抢注交易，因此 vp0 必须通过 `CORE_DNS_CHAINCODEID` 拿到真实链码 ID。

部署链码后需要重新下发 stack 配置：

```bash
bash scripts/swarm/deploy-stack.sh
```

这个步骤会让 `DNS_CHAINCODEID` 进入 vp0 容器环境。

## 9. 注册测试用户

原性能脚本会使用 `TOM`、`JACK`、`TAN` 等用户签名，因此需要注册用户：

```bash
bash scripts/swarm/register-users.sh
```

如果只测试 `ADMIN` 和 `JIM`，链码 `Init` 已经内置注册；但跑原始性能脚本时仍建议执行该脚本。

## 10. 功能验证

查询：

```bash
bash scripts/swarm/query-top-level.sh xxx.google.com
```

发起 6 参数顶级域注册：

```bash
bash scripts/swarm/invoke-top-level-update.sh \
  xxx.google.com \
  2.2.2.2 \
  A \
  86400 \
  admin \
  '<signature>'
```

注意：`signature` 必须与原业务逻辑一致，不能随便填。性能测试建议继续使用仓库中已有的 `txscripts` 数据。

## 11. 跑性能脚本

进入仓库后设置链码变量：

```bash
export zzm="$(cat deploy/swarm/last-chaincode-id.txt)"
```

然后执行原有交易脚本，例如：

```bash
bash txscripts/2_5w_normal_0.sh
```

具体压测工具如果使用外部 `perf-test.py`，需要保证它提交的是原始 6 参数 `TopLevelUpdate`，不要使用两参数版请求。

## 12. 成功标准

实验二成功需要同时满足：

- Swarm 中 10 个 peer 正常运行
- `VP0_BYZANTINE=true`
- `DNS_CHAINCODEID` 已经注入 vp0
- 原始 6 参数 `TopLevelUpdate` 可以提交
- 恶意主节点按原逻辑发起 `JIM / 4.4.4.4` 抢注交易
- batchsize 和 timeout 按 `500 / 1s` 正常出块

如果只产生 1 个大块，优先检查：

- 是否部署的是本分支镜像
- vp0 是否拿到了 `CORE_DNS_CHAINCODEID`
- 压测请求是否仍是原始 6 参数格式
- 是否误用了两参数版 `TopLevelUpdate`
