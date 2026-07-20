# 历史：10 节点 Swarm 实验执行手册

> 注意：本文档是早期 `feature/swarm-adaptation` / 4 VM / 10 节点实验记录，保留作历史参考。
> 当前 `feature/swarm-lab-exp1` 的实验一性能上限测试已经改为 2 台物理机、30 个诚实 PBFT peer，请使用 `deploy/swarm/LAB_EXP1_RUNBOOK.md`。

本文档给出当前 `feature/swarm-adaptation` 分支在 4 台 VM 上运行时的推荐执行顺序。

## 1. 拉取最新代码

在所有持有该仓库的 VM 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git checkout feature/swarm-adaptation
git pull origin feature/swarm-adaptation
```

预期结果：

- 本地 HEAD 更新到 `feature/swarm-adaptation` 的最新提交

常见失败点：

- 工作区不干净
- 拉错远端分支
- 仓库路径不在预期的 Fabric GOPATH 路径下

检查方法：

```bash
git log --oneline -n 3
git status --short
```

## 2. 构建 peer 镜像

在一台具备 Docker 和 Fabric 构建环境的机器上执行：

```bash
./scripts/swarm/build-peer-image.sh
```

预期结果：

- `make peer-image` 成功
- 生成的镜像被重新打标签为 `${FABRIC_PEER_IMAGE}`

常见失败点：

- `make` 相关依赖不完整
- Docker daemon 不可用
- Fabric 0.6 的旧构建依赖缺失

检查方法：

```bash
docker images | grep fabric-dns-peer
```

## 3. 把镜像分发到 4 台 VM

可以通过镜像仓库分发，也可以手动导出 / 复制 / 导入。一个可行的手动方案如下：

```bash
docker save "${FABRIC_PEER_IMAGE}" -o /tmp/fabric-dns-peer-swarm.tar
scp /tmp/fabric-dns-peer-swarm.tar dns-fabric-02:/tmp/
scp /tmp/fabric-dns-peer-swarm.tar dns-fabric-03:/tmp/
scp /tmp/fabric-dns-peer-swarm.tar dns-bind-01:/tmp/
```

然后在每个目标节点执行：

```bash
docker load -i /tmp/fabric-dns-peer-swarm.tar
```

预期结果：

- 4 台 VM 都能看到相同 tag 的 `${FABRIC_PEER_IMAGE}`

常见失败点：

- `.env` 中的镜像 tag 与实际加载镜像不一致
- 镜像只存在于 manager，不存在于 worker

## 4. 准备 `.env`

在 Swarm manager（`dns-fabric-01`）上执行：

```bash
cp deploy/swarm/.env.example deploy/swarm/.env
vi deploy/swarm/.env
```

至少确认以下配置：

- `FABRIC_PEER_IMAGE`
- `CORE_DNS_SUBNET=10.92.2.0/24`
- `VP0_NODE` 到 `VP9_NODE`
- `VP0_ENDPOINT=10.92.2.138:7051`
- `BIND_*` 目录配置
- 恶意节点标记，例如 `VP0_BYZANTINE`、`VP4_BYZANTINE`、`VP8_BYZANTINE`

预期结果：

- `.env` 与当前 4 台 VM 的实际布局一致

常见失败点：

- 仍然残留旧实验 IP
- 节点 hostname 配错
- `VP0_ENDPOINT` 指向了错误主机

## 5. 在 dns-bind-01 上准备 Bind9

在 `dns-bind-01` 上执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git checkout feature/swarm-adaptation
git pull origin feature/swarm-adaptation
cp deploy/swarm/.env.example deploy/swarm/.env
./scripts/swarm/prepare-bind-layout.sh
```

预期结果：

- Bind9 配置文件出现在 `${BIND_CONFIG_DIR}`
- zone 文件出现在 `${BIND_RECORDS_DIR}`

常见失败点：

- 目标目录没有写权限
- `dns-bind-01` 上 `.env` 内容与 manager 不一致

检查方法：

```bash
ls -R /opt/fabric-dns/bind
```

## 6. 部署 stack

在 Swarm manager 上执行：

```bash
./scripts/swarm/deploy-stack.sh
```

预期结果：

- overlay 网络存在
- `vp0` 到 `vp9` 以及 `bind9` 服务都出现在 stack 中

常见失败点：

- manager 上 Swarm 没有激活
- overlay 网络名与配置不一致
- `placement.constraints` 中的 hostname 与实际 Swarm 节点名不一致
- 某些 worker 上没有对应镜像

检查方法：

```bash
docker stack services "${STACK_NAME:-fabricdns}"
docker service ps "${STACK_NAME:-fabricdns}_vp0"
docker service ps "${STACK_NAME:-fabricdns}_bind9"
```

查看日志：

```bash
docker service logs -f "${STACK_NAME:-fabricdns}_vp0"
docker service logs -f "${STACK_NAME:-fabricdns}_bind9"
```

## 7. 定位并进入主 peer 容器

当前部署和查询的主 peer 是 `vp0`。

在 manager 上执行：

```bash
./scripts/swarm/get-service-container.sh vp0
./scripts/swarm/exec-vp0.sh
```

预期结果：

- 第一个命令输出正在运行的容器 ID
- 第二个命令进入 `vp0` 容器 shell

常见失败点：

- `vp0` 服务没有调度成功
- `vp0` 因 PBFT 或网络问题反复重启

## 8. 部署 DNS 链码

在 manager 上执行：

```bash
./scripts/swarm/deploy-dns-chaincode.sh
```

预期结果：

- deploy 成功返回链码名
- deploy 输出写入 `deploy/swarm/last-chaincode-deploy.log`
- 链码名写入 `deploy/swarm/last-chaincode-id.txt`

常见失败点：

- `VP0_ENDPOINT` 不可达
- peer 找不到链码路径
- Fabric 0.6 的链码容器网络没有进入预期网络

检查方法：

```bash
cat deploy/swarm/last-chaincode-deploy.log
cat deploy/swarm/last-chaincode-id.txt
```

## 9. 查询 TopLevelGetAll

在 manager 上执行：

```bash
./scripts/swarm/query-top-levels.sh
```

预期结果：

- 返回类似 JSON 的结果，至少包含 `com` 和 `cn` 的权威 DNS 映射

常见失败点：

- 链码名文件不存在
- deploy 没有真正成功
- `vp0` gRPC 地址配置错误

手动兜底方式：

```bash
docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode query \
    -n "$(cat deploy/swarm/last-chaincode-id.txt)" \
    -c '{"Function":"TopLevelGetAll","Args":[]}'
```

## 10. 更新一条 DNS 记录

在 deploy 成功后，可以基于返回的链码名执行 invoke。下面给出一个例子，把 `www.example.com` 指向 `10.92.2.138`：

```bash
docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode invoke \
    -n "$(cat deploy/swarm/last-chaincode-id.txt)" \
    -c '{"Function":"update","Args":["www.example.com","10.92.2.138"]}'
```

预期结果：

- invoke 不发生 PBFT 超时

常见失败点：

- Bind9 在 `10.92.2.140:53` 还未真正可用
- 当前 zone 没有开启动态更新

## 11. 通过 Bind9 验证

在任意安装了 `dig` 的主机上执行：

```bash
./scripts/swarm/dig-authority.sh www.example.com
```

预期结果：

- 返回刚刚写入的 A 记录

常见失败点：

- Bind9 服务没有监听宿主机 53 端口
- 链码 invoke 在账本层面成功，但更新没有真正写入 Bind9
- 连续实验时被 DNS 缓存干扰

手动兜底方式：

```bash
dig @10.92.2.140 www.example.com +short
```

## 12. 主要风险点

- `GetLocalIP` 与 `CORE_DNS_SUBNET` 必须保持在 `10.92.2.0/24`
- `feature/v0.6-multi-host` 中的旧 YAML 只能参考，不能直接部署到 Swarm
- 旧脚本中写死的 `zzm`、路径和 IP 不能继续复用
- Fabric 0.6 的链码启动依赖宿主机 Docker 网络，对 overlay 网络名很敏感
- 当前仓库没有独立可复用的 `addToZone` 服务，因此目前是由 Bind9 直接作为权威 DNS 端点
