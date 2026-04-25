# 当前服务现状与下一步执行步骤（2026-04-20）

## 一、当前结论

当前阶段**不建议先修改代码**。

原因是：

- 当前阻塞点不是新的源码缺陷，而是 **PBFT 10 节点网络尚未完整形成**
- 交接文档中提到的关键配置项已经修正：
  - `CORE_DNS_SUBNET=10.0.1.0/24`
  - `CORE_PEER_ADDRESSAUTODETECT=true`
  - 不再手动设置 `CORE_PEER_ADDRESS`
- 在 PBFT 网络未恢复前，继续改代码的收益很低，且容易引入新的不确定性

因此，当前最合理的策略是：

1. 先确认服务器代码是最新版本
2. 先恢复 10 节点 PBFT 网络
3. 再执行链码 deploy / query / invoke / dig 验证
4. 只有在运行验证仍失败时，才进入下一轮针对性修改代码

## 二、当前服务现状

根据桌面交接文档《实验交接文档_当前进度_20260420.md》，当前服务状态如下：

- `fabricdns` Stack 已部署完成
- 11 个服务都处于 `1/1` 运行
- 但 PBFT 共识网络尚未形成
- 当前只有 `vp0` 和 `vp3` 已互联
- 其余 8 个节点在 `vp0` 尚未启动时尝试连接失败，并且没有自动恢复

因此，当前系统属于：

“**表面服务都正常，但底层共识网络不完整**”

## 三、现在应该按什么顺序做

以下步骤建议严格按顺序执行，不要跳步。

### Step 1：确认服务器上的代码版本

在 `dns-fabric-01` 上进入工作目录：

```bash
cd /root/go/src/github.com/hyperledger/fabric
```

检查当前分支和最新提交：

```bash
git branch
git log -1 --oneline
```

确认两件事：

- 当前分支为 `feature/swarm-adaptation`
- 提交已经是本地推送后的最新版本

如果不是最新版本，先执行：

```bash
git pull
```

### Step 2：确认 Swarm 与服务状态

先确认 Docker Swarm 正常：

```bash
docker node ls
```

再确认 `fabricdns` Stack 的服务状态：

```bash
docker stack services fabricdns
```

预期结果：

- 4 个 swarm 节点都在线
- `fabricdns` 下 11 个服务都是 `1/1`

### Step 3：强制重启 8 个未正常接入 PBFT 的节点

在 `dns-fabric-01` 上执行：

```bash
for vp in vp1 vp2 vp4 vp5 vp6 vp7 vp8 vp9; do
  docker service update --force fabricdns_${vp}
  echo "已触发 ${vp} 重启"
done
```

执行后等待约 60 秒。

### Step 4：检查 PBFT 已连接节点数

执行：

```bash
curl -s http://localhost:7050/network/peers | python3 -c \
  "import sys,json; peers=json.load(sys.stdin)['peers']; \
   print(f'已连接: {len(peers)} 个节点'); \
   [print(f'  {p[\"ID\"][\"name\"]} @ {p[\"address\"]}') for p in peers]"
```

判断标准：

- **必须看到 10 个节点**
- 少于 10 个节点时，先不要做链码部署

### Step 5：如果已经达到 10 节点，部署 DNS 链码

执行：

```bash
./scripts/swarm/deploy-dns-chaincode.sh
```

预期结果：

- 脚本正常返回链码名
- 链码名自动写入：

```text
deploy/swarm/last-chaincode-id.txt
```

### Step 6：查询顶级域映射，确认账本写入成功

执行：

```bash
./scripts/swarm/query-top-levels.sh
```

预期结果：

```json
{"com":"10.92.2.140:53","cn":"10.92.2.140:53"}
```

如果这里出现：

- `ledger: resource not found`
- 或没有查到 `com/cn`

说明链码没有真正提交到账本，需要回头继续检查 PBFT 连接状态。

### Step 7：写入一条 DNS 记录

执行：

```bash
CHAINCODE_ID=$(cat deploy/swarm/last-chaincode-id.txt)
docker run --rm \
  -e CORE_PEER_ADDRESS="10.92.2.138:7051" \
  fabric-dns-peer:swarm \
  peer chaincode invoke \
    -n "${CHAINCODE_ID}" \
    -c '{"Function":"update","Args":["www.example.com","10.92.2.138"]}'
```

这一步的业务含义是：

- 向链码提交 `update`
- 把 `www.example.com` 指向 `10.92.2.138`

### Step 8：用 `dig` 验证最终解析结果

执行：

```bash
./scripts/swarm/dig-authority.sh www.example.com
```

或者直接执行：

```bash
dig @10.92.2.140 www.example.com +short
```

如果返回 `10.92.2.138`，说明实验主链路已经打通。

## 四、执行过程中的判断逻辑

可以按下面这套逻辑快速判断下一步动作：

### 情况 1：服务不是 `1/1`

先不要继续实验，先排查 stack 和容器状态。

优先检查：

```bash
docker stack services fabricdns
docker service ps fabricdns_vp0
docker service logs --tail 50 fabricdns_vp0
```

### 情况 2：服务都 `1/1`，但 `network/peers` 少于 10 个

说明当前问题仍然是 PBFT 网络未形成。  
此时**不要先 deploy 链码**，应继续排查节点连接。

### 情况 3：节点达到 10 个，但 `query-top-levels.sh` 查不到结果

说明链码可能没有真正提交到账本。  
此时优先检查：

- deploy 输出是否正常
- PBFT 节点是否在 deploy 时仍然保持 10 节点连通

### 情况 4：链码 deploy 和 query 都正常，但 `update` 后 `dig` 查不到

说明问题已经从 PBFT/账本层转移到 DNS 权威服务侧。  
此时重点检查：

- Bind9 是否就绪
- zone 是否允许动态更新
- `10.92.2.140:53` 是否真的可达

## 五、当前最重要的一句话

当前不应继续盲目修改代码。  
**现在最重要的工作，是先把 PBFT 网络恢复到 10 节点，再继续链码部署与 DNS 验证。**

## 六、最短执行清单

如果只保留最短操作步骤，可按下面顺序执行：

```bash
cd /root/go/src/github.com/hyperledger/fabric
git branch
git log -1 --oneline
docker node ls
docker stack services fabricdns
for vp in vp1 vp2 vp4 vp5 vp6 vp7 vp8 vp9; do docker service update --force fabricdns_${vp}; done
sleep 60
curl -s http://localhost:7050/network/peers | python3 -c "import sys,json; peers=json.load(sys.stdin)['peers']; print(f'已连接: {len(peers)} 个节点')"
./scripts/swarm/deploy-dns-chaincode.sh
./scripts/swarm/query-top-levels.sh
CHAINCODE_ID=$(cat deploy/swarm/last-chaincode-id.txt)
docker run --rm -e CORE_PEER_ADDRESS=\"10.92.2.138:7051\" fabric-dns-peer:swarm peer chaincode invoke -n \"${CHAINCODE_ID}\" -c '{\"Function\":\"update\",\"Args\":[\"www.example.com\",\"10.92.2.138\"]}'
./scripts/swarm/dig-authority.sh www.example.com
```
