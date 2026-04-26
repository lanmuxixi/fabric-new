# 实验三运行手册：主节点防御实验（PoW）

本文档对应分支 `feature/swarm-exp3-primary-defense`。  
该实验仍然建立在“主节点作恶”路径上，不是副本节点分支。

## 1. 实验目标

目标是验证以下机制：

- 用户提交的顶级域注册请求必须携带 PoW
- 恶意主节点如果想把 `TopLevelUpdate(domain, honestAuthority)` 改写成 `TopLevelUpdate(domain, byzantineAuthority)`，就必须重新计算一份新的 PoW
- 由于用户请求已经广播给其他副本，而恶意主节点需要额外花时间重算 PoW，若重算时间超过 PBFT 请求超时，就会触发超时和换主
- 换主后，正常请求由新的主节点继续推进，最终链上保存的是用户原始提交的权威 DNS 地址

## 2. 与实验二的区别

实验二的主节点逻辑是：

- 恶意主节点直接改写请求
- 恶意请求立即先进 batch
- 链码采用“先注册先占有”语义

实验三的主节点逻辑是：

- 恶意主节点先暂扣用户原始请求
- 后台异步重算“恶意 authority + 同一 target”的 PoW
- 只有算出来之后，才会把恶意请求排到原请求前面
- 如果还没算完就触发 PBFT 请求超时，系统会进入 view change，正常请求由新主节点继续处理

因此，实验三不是简单“继续抢注”，而是验证 PoW 是否足以拖慢恶意主节点，给 PBFT 的超时换主留出窗口。

## 3. 代码改动摘要

- `core/util/dns_pow.go`
  - 新增顶级域 `TopLevelUpdate` 的 PoW 公共规则
  - 统一了 payload 形式：`TopLevelUpdate|domain|authority`
- `examples/chaincode/go/chaincode_dns_reslover/functions/topLevelDomainUpdate.go`
  - `TopLevelUpdate` 从 2 个参数改为 4 个参数
  - 新格式为：`domain authority target nonce`
  - 链码在写账本前会校验 PoW
- `consensus/pbft/batch.go`
  - 恶意主节点不再立刻插入伪造请求
  - 改为“暂扣原请求 + 后台算恶意 PoW + 算好后再回注事件”
- `scripts/swarm/invoke-top-level-update.sh`
  - 调用前自动计算 PoW
- `scripts/swarm/calc-top-level-pow.py`
  - 负责本地搜索 nonce

## 4. 前提条件

- 服务器代码已切到 `feature/swarm-exp3-primary-defense`
- 10 个 PBFT 节点可以形成完整网络
- `bind9` 服务可用
- `deploy/swarm/.env` 已准备好

## 5. 关键配置

建议先在 `deploy/swarm/.env` 中确认以下参数：

```bash
VP0_BYZANTINE=true
VP4_BYZANTINE=false
VP8_BYZANTINE=false

DNS_BZAUTHORITY=10.92.2.141:53

# 建议实验三使用比实验二更难的 target
DNS_POW_TARGET=000000ffffffffffffffffffffffffffffffffffffffffffffffffffffffffff

# 如需更容易触发换主，可适当缩短请求超时
CORE_PBFT_GENERAL_TIMEOUT_REQUEST=5s
CORE_PBFT_GENERAL_TIMEOUT_VIEWCHANGE=3s
```

说明：

- 当前实验全部基于“主节点作恶”路径，因此默认只打开 `VP0_BYZANTINE=true`
- `DNS_BZAUTHORITY` 是恶意主节点伪造时想写入链上的权威 DNS 地址
- `DNS_POW_TARGET` 越小越难，恶意主节点重算 PoW 越慢
- 如果恶意主节点仍然能在超时前完成伪造，可以进一步减小 `DNS_POW_TARGET`，或缩短 `CORE_PBFT_GENERAL_TIMEOUT_REQUEST`

## 6. 执行步骤

### 6.1 切换代码并部署 Stack

```bash
cd /root/go/src/github.com/hyperledger/fabric
git fetch origin
git checkout feature/swarm-exp3-primary-defense
git pull origin feature/swarm-exp3-primary-defense
bash scripts/swarm/deploy-stack.sh
```

### 6.2 确认 10 节点 PBFT 网络已经形成

```bash
curl -s http://localhost:7050/network/peers | python3 -c \
  "import sys,json; peers=json.load(sys.stdin)['peers']; print(len(peers)); [print(p['ID']['name'], p['address']) for p in peers]"
```

预期：

- 输出节点数为 `10`

### 6.3 部署 DNS 链码

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
cat deploy/swarm/last-chaincode-id.txt
```

### 6.4 查询目标顶级域的初始状态

```bash
bash scripts/swarm/query-top-level.sh example.org
```

若 `org` 尚未注册，返回应为空或无映射。

### 6.5 发起携带 PoW 的正常注册请求

```bash
bash scripts/swarm/invoke-top-level-update.sh example.org 10.92.2.140:53
```

脚本会先打印本次使用的：

- `target`
- `nonce`

然后再发起链码调用。  
本次调用实际发送的是：

```json
{
  "Function": "TopLevelUpdate",
  "Args": [
    "example.org",
    "10.92.2.140:53",
    "<pow-target>",
    "<nonce>"
  ]
}
```

### 6.6 查询最终链上结果

```bash
bash scripts/swarm/query-top-level.sh example.org
```

## 7. 成功判据

满足以下条件即可认为实验三复现成功：

- 用户提交的是正常权威地址，例如 `10.92.2.140:53`
- 恶意主节点配置的地址是 `DNS_BZAUTHORITY`
- 最终链上查询结果仍然是用户原始提交的正常地址，而不是恶意地址

若同时在日志中看到 `view change`、`new view` 或请求超时相关信息，则更能说明：

- 恶意主节点在重算 PoW 时被 PBFT 的超时机制压制
- 防御成功依赖的是“PoW 增加作恶成本 + PBFT 换主”

## 8. 如果实验没有成功

若最终仍然写入了恶意地址，通常说明以下两点之一：

- `DNS_POW_TARGET` 还不够难，恶意主节点来得及完成伪造
- `CORE_PBFT_GENERAL_TIMEOUT_REQUEST` 过长，系统迟迟没有触发换主

建议按以下顺序调整：

1. 先把 `DNS_POW_TARGET` 再调小一级
2. 如果仍然不够，再把 `CORE_PBFT_GENERAL_TIMEOUT_REQUEST` 适当缩短
3. 重新部署 Stack 后再重复实验

## 9. 备注

- 当前链码对顶级域注册仍采用“先注册先占有”语义
- 实验三的防御点不在于允许覆盖，而在于阻止恶意主节点先一步构造出有效的伪造请求
- `scripts/swarm/invoke-top-level-update.sh` 已经内置 PoW 计算，不需要手工拼 `target` 和 `nonce`
