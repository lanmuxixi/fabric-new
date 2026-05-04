# 当前项目进度交接文档（给另一个 AI 助手）

更新时间：2026-05-04

本文档用于把当前 Fabric DNS + PBFT 实验项目的状态同步给另一个 AI 助手。当前任务已经从云平台虚拟机迁移方向，转为在学校实验室三台全新服务器上重新部署 10 节点实验环境。

## 1. 当前仓库状态

本地仓库路径：

```text
C:\Users\LEGION\Desktop\fabric-new\fabric0.60_backup
```

当前分支：

```text
feature/swarm-exp4-replica-preemption
```

当前工作区状态：

```text
clean
```

当前最新提交：

```text
742597c docs(swarm): add lab three-node experiment runbook
```

该提交已经推送到远端：

```text
origin/feature/swarm-exp4-replica-preemption
```

注意：最新的三服务器部署总文档目前提交在实验四分支上。如果后续从实验一分支 `feature/swarm-adaptation` 新开迁移分支，需要把该文档复制或 cherry-pick 到新分支中。

## 2. 已完成的四条实验分支

当前项目已有 4 条主要实验线：

```text
实验一：feature/swarm-adaptation
实验二：feature/swarm-exp2-preemption
实验三：feature/swarm-exp3-primary-defense
实验四：feature/swarm-exp4-replica-preemption
```

### 实验一：基础 Swarm DNS 实验

分支：

```text
feature/swarm-adaptation
```

目标：

```text
10 个 PBFT peer + DNS chaincode + Bind9
```

主要能力：

- Docker Swarm 部署 10 个 Fabric peer
- 部署 DNS 链码
- 初始化顶级域映射，例如 `com -> Bind9`
- 执行普通域名 `update / resolve / delete`
- 使用 `dig` 从 Bind9 验证 DNS 解析结果

这是后续实验二、三、四的基础环境。迁移到实验室三台服务器时，必须先跑通实验一。

### 实验二：主节点作恶抢注

分支：

```text
feature/swarm-exp2-preemption
```

对应实验方向：

```text
主节点作恶，nopow
```

核心逻辑：

- 恶意主节点识别 `TopLevelUpdate`
- 将用户提交的正常权威 DNS 地址改写为恶意权威 DNS 地址
- 恶意请求先进入 PBFT batch
- 链码采用“先注册先占有”，后到的正常注册失败

关键配置：

```bash
VP0_BYZANTINE=true
DNS_BZAUTHORITY=<恶意权威 DNS 地址>
```

### 实验三：主节点防御

分支：

```text
feature/swarm-exp3-primary-defense
```

对应历史/PPT 分支：

```text
v0.6-primarybz-pow
```

核心逻辑：

- 仍然是主节点作恶场景
- 请求中加入 PoW 参数
- 正常用户提交合法 nonce
- 恶意主节点如果篡改权威 DNS 地址，需要重新计算 PoW
- 链码侧校验 PoW

注意：实验三是主节点防御实验，不是副本节点实验。

### 实验四：副本节点作恶抢注

分支：

```text
feature/swarm-exp4-replica-preemption
```

对应历史/PPT 分支：

```text
v0.6-replica-nopow
```

当前最新提交：

```text
742597c docs(swarm): add lab three-node experiment runbook
7e4f620 fix(exp4): guard legacy leader request path
a08b6e1 feat(exp4): add swarm replica preemption flow
```

核心逻辑：

- 恶意副本节点收到用户的 `TopLevelUpdate`
- 恶意副本先广播一个伪造请求
- 伪造请求把权威 DNS 地址改成 `DNS_BZAUTHORITY`
- 恶意副本随后再转发用户原始请求
- 链码采用“先注册先占有”，恶意请求先落链，正常请求失败

关键配置：

```bash
VP4_BYZANTINE=true
REPLICA_ENTRY_ENDPOINT=<vp4 对外 gRPC 地址>
DNS_BZAUTHORITY=<恶意权威 DNS 地址>
```

## 3. 当前迁移目标

原云平台虚拟机无法满足实验需求，现在决定使用学校实验室三台全新服务器重新部署。

服务器信息：

```text
node7        10.161.34.8
wdb服务器2   10.161.34.51
node21       10.161.34.22
```

当前重要前提：

- 三台服务器尚未加入 Docker Swarm 集群
- Docker 环境需要重新安装和配置
- Fabric peer 镜像需要重新构建和分发
- Bind9 目录需要重新准备
- 所有实验都需要基于新的三服务器环境重新跑

## 4. 建议的三服务器角色分配

建议先按以下方式部署 10 个 PBFT 节点：

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

关键地址建议：

```bash
CORE_DNS_SUBNET=10.161.34.0/24
VP0_ENDPOINT=10.161.34.8:7051
REPLICA_ENTRY_ENDPOINT=10.161.34.51:7151
NORMAL_DNS_AUTHORITY=10.161.34.22:53
```

注意：`stack.yml` 的 placement constraint 使用的是 `docker node ls` 中的真实 hostname。必须先在 Swarm manager 上执行：

```bash
docker node ls
```

确认三台机器真实节点名后，才能最终写死 `VP*_NODE` 和 `BIND_NODE`。

如果 `wdb服务器2` 在 Docker 中显示为中文 hostname，建议改成 ASCII，例如：

```bash
sudo hostnamectl set-hostname wdb-server-2
```

## 5. 已新增的三服务器部署文档

当前已经新增完整部署文档：

```text
deploy/swarm/LAB_3NODE_EXPERIMENT_RUNBOOK.md
```

该文档内容包括：

- 三服务器角色规划
- 从零安装 Docker
- 初始化 Docker Swarm
- 加入 worker 节点
- 构建并分发 `fabric-dns-peer:swarm` 镜像
- 创建 overlay 网络 `fabric_dns`
- 准备 Bind9 目录
- 配置实验室 `.env`
- 实验一基础验证步骤
- 实验二、三、四迁移后的运行入口
- 常见问题排查

## 6. 下一步建议

下一步不要直接改实验二、三、四。应先迁移实验一。

推荐操作：

1. 切回实验一基础分支：

```bash
git checkout feature/swarm-adaptation
git pull origin feature/swarm-adaptation
```

2. 从实验一分支新开实验室迁移分支：

```bash
git checkout -b feature/swarm-lab-exp1
```

3. 把 `LAB_3NODE_EXPERIMENT_RUNBOOK.md` 复制或 cherry-pick 到该分支。

4. 新增或调整实验室专用配置，建议新增：

```text
deploy/swarm/.env.lab.example
```

不要直接覆盖 `.env.example`，避免破坏原云平台配置参考。

5. 等用户提供 `docker node ls` 输出后，再写入真实节点名：

```bash
VP0_NODE=<docker node ls 中 node7 的 hostname>
VP1_NODE=<同上>
VP2_NODE=<同上>
VP3_NODE=<同上>

VP4_NODE=<第二台服务器 hostname>
VP5_NODE=<第二台服务器 hostname>
VP6_NODE=<第二台服务器 hostname>

VP7_NODE=<node21 hostname>
VP8_NODE=<node21 hostname>
VP9_NODE=<node21 hostname>
BIND_NODE=<node21 hostname>
```

6. 实验一跑通后，再把同样的三服务器部署参数同步到：

```text
feature/swarm-exp2-preemption
feature/swarm-exp3-primary-defense
feature/swarm-exp4-replica-preemption
```

## 7. 实验一跑通标准

实验一在三服务器环境中必须满足以下条件：

```bash
docker stack services fabricdns
```

所有服务应为 `1/1`。

```bash
curl -s http://localhost:7050/network/peers
```

应看到 10 个 peer。

```bash
bash scripts/swarm/deploy-dns-chaincode.sh
bash scripts/swarm/query-top-levels.sh
```

应看到类似：

```text
com -> 10.161.34.22:53
cn  -> 10.161.34.22:53
```

普通域名写入后：

```bash
dig @10.161.34.22 www.example.com +short
```

应返回写入的 IP，例如：

```text
10.161.34.8
```

## 8. 需要另一个 AI 助手特别注意的点

1. 当前迁移是从零搭环境，不是单纯改 `.env`。

2. 必须先跑通实验一，实验二、三、四都依赖同一套 10 节点 PBFT 网络。

3. 不要把实验三 PoW 逻辑混进实验二或实验四。

4. 实验四是副本节点作恶，对应 `v0.6-replica-nopow`，不是主节点作恶。

5. 实验二和实验四都依赖链码“先注册先占有”语义。

6. `VP0_ENDPOINT` 是主入口，实验四的 `REPLICA_ENTRY_ENDPOINT` 必须打到恶意副本 `vp4`。

7. `docker node ls` 的 hostname 是部署配置的依据，不要凭服务器备注名直接写。

8. 当前本机没有 Go 工具链，之前代码 review 多为源码级和静态检查，编译/运行验证需要在服务器上完成。

## 9. 当前最短行动清单

给另一个 AI 助手的下一步：

```text
1. 等用户完成三台服务器 Docker 安装。
2. 让用户在 node7 上执行 docker swarm init。
3. 让另外两台服务器 join Swarm。
4. 获取 docker node ls 输出。
5. 切到 feature/swarm-adaptation。
6. 新开 feature/swarm-lab-exp1。
7. 新增 deploy/swarm/.env.lab.example。
8. 按真实 hostname 写三服务器节点布局。
9. 推送分支。
10. 让用户在 node7 拉取后部署实验一。
```
