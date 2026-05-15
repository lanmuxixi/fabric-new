# Swarm 实验说明

当前目录对应的是实验二 10 节点 Docker Swarm 部署版本：

- `roott-I620-G20` 上部署 `vp0` `vp1` `vp2` `vp3`
- `qichang-I420-G20` 上部署 `vp4` `vp5` `vp6`
- `node21` 上部署 `vp7` `vp8` `vp9` 和 Bind9

这套方案以 `v0.6-primarybz-nopow` 为业务基线，只把部署方式迁移到 Docker Swarm。DNS 链码和主节点抢注逻辑仍使用原实验二实现，对应目录是：

- `examples/chaincode/go/chaincode_dns_reslover`

## 为什么旧 compose 文件不能直接用

历史上的 `peer.yml`、`4-peers.yml`、`10-peers.yml`、`10vp_1nvp.yml` 可以作为参考，但不能直接用于 `docker stack deploy`，原因包括：

- 依赖 `extends`
- 依赖 `links`
- 默认是单机 Compose 语义
- 存在旧镜像名、旧启动方式和历史硬编码

因此，当前分支不是“照搬旧 yml”，而是保留旧拓扑思路后，改写成明确的 Swarm service 定义。

## 当前网络设计的关键点

Fabric 0.6 的链码是通过宿主机 Docker daemon 启动的。在 Swarm 环境中，下面三项必须保持一致：

- `stack.yml` 中的 overlay 网络名
- `CORE_VM_DOCKER_HOSTCONFIG_NETWORKMODE`
- `GetLocalIP()` 使用的 `dns.subnet` / `CORE_DNS_SUBNET`

如果这三项不一致，常见结果是：

- Peer 在多网卡环境下选错对外地址
- 链码容器进入错误网络
- Peer 与链码容器之间无法按预期通信

## 当前 DNS 链码接口

本分支中的 DNS 链码当前支持以下接口：

- `init`
- `resolve`
- `update`
- `delete`
- `TopLevelQuest`
- `TopLevelUpdate`
- `TopLevelDelete`
- `TopLevelGetAll`

其中：

- `TopLevelGetAll` 是当前最重要的部署后验活查询接口
- `TopLevelUpdate` 仍是原始 6 参数结构：`domain, ip, type, ttl, owner, signature`

## 目录内容

- `stack.yml`：10 个 validating peer 加 1 个 Bind9 服务的 Swarm 编排文件
- `.env.example`：部署变量模板
- `LAB_EXP2_RUNBOOK.md`：实验二完整执行顺序、检查点和常见失败点
- `bind/`：Bind9 样板配置与 `com` / `cn` 初始 zone 文件
- `../../scripts/swarm/build-peer-image.sh`：构建并重打标签 peer 镜像
- `../../scripts/swarm/deploy-stack.sh`：创建 overlay 网络并部署 stack
- `../../scripts/swarm/remove-stack.sh`：删除 stack
- `../../scripts/swarm/deploy-dns-chaincode.sh`：通过 `vp0` 部署 DNS 链码
- `../../scripts/swarm/register-users.sh`：注册原性能脚本需要的测试用户
- `../../scripts/swarm/query-top-levels.sh`：执行 `TopLevelGetAll` 验证查询
- `../../scripts/swarm/get-service-container.sh`：解析某个 Swarm service 对应的运行中容器
- `../../scripts/swarm/exec-vp0.sh`：进入 `vp0` 容器
- `../../scripts/swarm/prepare-bind-layout.sh`：在 `dns-bind-01` 上安装 Bind9 样板配置
- `../../scripts/swarm/dig-authority.sh`：通过 Bind9 做 `dig` 验证

## 构建镜像

`feature/v0.6-multi-host` 里的旧 `build_image.sh` 不建议继续直接使用，因为它存在这些问题：

- 写死旧路径
- 写死旧 `zzm`
- 需要把代码复制到单独的镜像工作目录

当前分支推荐的替代方式是：

```bash
./scripts/swarm/build-peer-image.sh
```

该脚本会在当前仓库内执行 `make peer-image`，然后把生成的镜像重命名为适合 Swarm 使用的 tag。

当前实验默认未启用 Fabric 安全模式，所以通常不需要再构建 `membersrvc-image`。

## 部署 stack

1. 在 Swarm manager 上把 `.env.example` 复制为 `.env`，并按实际环境调整镜像、节点放置和 Bind9 路径：

```bash
cp deploy/swarm/.env.example deploy/swarm/.env
```

2. 确保 peer 镜像已经存在于所有 Swarm 节点上。

3. 确保 `dns-bind-01` 上的 Bind9 目录已经准备好：

- `${BIND_CONFIG_DIR}`
- `${BIND_CACHE_DIR}`
- `${BIND_RECORDS_DIR}`

也可以直接运行：

```bash
./scripts/swarm/prepare-bind-layout.sh
```

4. 部署 stack：

```bash
./scripts/swarm/deploy-stack.sh
```

5. 检查部署状态：

```bash
docker stack services "${STACK_NAME:-fabricdns}"
docker service ps "${STACK_NAME:-fabricdns}_vp0"
docker service logs -f "${STACK_NAME:-fabricdns}_vp0"
```

## 部署 DNS 链码

确认 `vp0` 正常后，在 manager 上执行：

```bash
./scripts/swarm/deploy-dns-chaincode.sh
```

当前默认初始化内容为：

- `com -> 10.92.2.140:53`
- `cn -> 10.92.2.140:53`

deploy 成功后，脚本会做两件事：

- 把原始 deploy 输出写入 `deploy/swarm/last-chaincode-deploy.log`
- 把识别到的链码名写入 `deploy/swarm/last-chaincode-id.txt`
- 把识别到的链码名写入 `.env` 中的 `DNS_CHAINCODEID`

原主节点抢注逻辑会在 `vp0` 容器内部读取 `dns.chaincodeid` 并调用 `peer chaincode invoke` 发起伪造交易。因此链码部署成功后，需要再次执行：

```bash
./scripts/swarm/deploy-stack.sh
```

这样 `DNS_CHAINCODEID` 才会以 `CORE_DNS_CHAINCODEID` 的形式进入 `vp0` 容器环境。

## 验证查询

最短路径是直接运行：

```bash
./scripts/swarm/query-top-levels.sh
```

如果要手动执行，把 `<CHAINCODE_NAME>` 替换为 deploy 返回的链码名：

```bash
docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode query \
    -n <CHAINCODE_NAME> \
    -c '{"Function":"TopLevelGetAll","Args":[]}'
```

## 定位并进入 vp0

当前主部署 / 主查询节点是 `vp0`。

在 manager 上获取其容器 ID：

```bash
./scripts/swarm/get-service-container.sh vp0
```

直接进入该容器：

```bash
./scripts/swarm/exec-vp0.sh
```

## Bind9 / addToZone 说明

当前仓库中并没有一个独立、可直接复用的 `addToZone` 守护进程。

因此当前设计是：

- 把 Bind9 作为权威 DNS 服务
- 链码直接通过动态更新 / 查询与 Bind9 交互

如果将来补一个单独的同步程序，它应当：

- 运行在 `dns-bind-01`
- 使用当前 deploy 返回的链码名进行联动

最基本的 DNS 验证方式是：

```bash
./scripts/swarm/dig-authority.sh www.example.com
```

## 清理

```bash
./scripts/swarm/remove-stack.sh
```
