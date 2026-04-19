# PBFT 报文证据清单与图中箭头映射表

## 一、核对范围

本清单仅基于当前仓库真实代码整理，不采用教材版 PBFT 的默认结论。优先核对文件如下：

- `protos/fabric.proto`
- `consensus/pbft/messages.proto`
- `consensus/pbft/batch.go`
- `consensus/pbft/pbft-core.go`
- `consensus/pbft/viewchange.go`
- `consensus/pbft/external.go`
- `consensus/helper/engine.go`
- `consensus/helper/handler.go`
- `core/peer/peer.go`
- `core/peer/handler.go`

## 二、先给出关键结论

### 1. 当前工程中的 PBFT 消息不是单层结构，而是分层封装

真实封装层次如下：

```text
客户端/节点提交交易
-> protos.Transaction
-> protos.Message(type=CHAIN_TRANSACTION)
-> pbft.Request
-> protos.Message(type=CONSENSUS)
-> pbft.BatchMessage
-> pbft.Message
-> PrePrepare / Prepare / Commit / ...
```

代码依据：

- `protos/fabric.proto:28` `Transaction`
- `protos/fabric.proto:164` `Message`
- `consensus/pbft/messages.proto:51` `request`
- `consensus/pbft/messages.proto:134` `batch_message`
- `consensus/pbft/messages.proto:37` `message`
- `core/peer/peer.go:520` `sendTransactionsToLocalEngine`
- `consensus/pbft/batch.go:349` `txToReq`
- `consensus/pbft/batch.go:140` `submitToLeader`
- `consensus/pbft/batch.go:560` `wrapMessage`

### 2. 图中的 `reply` 在当前工程里不是单独的 PBFT proto 报文

当前代码中没有 `pbft.reply`、`Reply` 或等价的 PBFT 协议消息定义。工程里“返回给调用方”的路径是 `pb.Response`：

- 对交易类请求，正常入队路径下 `EngineImpl.ProcessTransactionMsg` 会返回 `SUCCESS + txid`，随后才异步送入共识；
- 但如果交易预校验失败、共识引擎未初始化或 `RecvMsg` 返回错误，也会直接返回 `FAILURE`；
- 对查询类请求，直接同步执行链码并返回查询结果；
- 未在当前核对路径中定位到“节点在 commit 完成后，再向客户端发送一个 PBFT reply 报文”的实现。

代码依据：

- `protos/fabric.proto:194` `Response`
- `protos/fabric.proto:126` `rpc ProcessTransaction(Transaction) returns (Response)`
- `core/peer/peer.go:302` `ProcessTransaction`
- `core/peer/peer.go:643` `ExecuteTransaction`
- `core/peer/peer.go:520` `sendTransactionsToLocalEngine`
- `consensus/helper/engine.go:47` `ProcessTransactionMsg`

### 3. 图中的 `request` 也不是“客户端直接单播一个 pbft.request 给主节点”

当前工程实现与教材图有差异：

- 客户端或 NVP 先提交的是 `Transaction`
- 验证节点本地把 `Transaction` 封装成 `pbft.Request`
- 然后 `submitToLeader` 调用 `broadcastMsg`，把 `BatchMessage.request` 广播给所有副本，而不是仅单播主节点
- 主节点作为副本之一收到后，再负责聚合成 `RequestBatch`

代码依据：

- `core/peer/peer.go:520` `sendTransactionsToLocalEngine`
- `consensus/helper/engine.go:47` `ProcessTransactionMsg`
- `consensus/pbft/external.go:58` `RecvMsg`
- `consensus/pbft/batch.go:363` `processMessage`
- `consensus/pbft/batch.go:349` `txToReq`
- `consensus/pbft/batch.go:140` `submitToLeader`
- `consensus/pbft/batch.go:156` `broadcastMsg`

### 4. 多个 proto 变体“已定义但当前生产路径未直接使用”

下列结构在 proto 中存在，但在当前生产代码路径中未定位到直接发送逻辑，不能写成“当前正在正常使用”：

- `pbft.message.request_batch` 顶层 oneof 变体
- `pbft.batch_message.request_batch` 变体
- `pbft.batch_message.complaint` 变体

其中：

- `request_batch` 在当前实现中确实被使用，但主要作为：
  - `PrePrepare.request_batch` 的字段
  - `return_request_batch` 的载荷
  - 本地事件循环里的 `*RequestBatch`
- 未定位到生产代码主动发送 `Message_RequestBatch` 或 `BatchMessage_RequestBatch`
- `complaint` 仅在 proto 和生成代码中存在，未定位到生产代码构造或处理逻辑

代码依据：

- `consensus/pbft/messages.proto:37` `message`
- `consensus/pbft/messages.proto:134` `batch_message`
- `consensus/pbft/pbft-core.go:611` `recvRequestBatch`
- `consensus/pbft/pbft-core.go:1250` `recvFetchRequestBatch`
- `consensus/pbft/batch.go:363` `processMessage`
- `rg` 核对未发现生产代码中的 `Message_RequestBatch{` / `BatchMessage_RequestBatch{` / `GetComplaint()` 调用

### 5. `view_change.signature` 有真实签名与验签；`request.signature` 当前基本未使用

- `ViewChange.signature` 会在发送前签名，并在接收 `ViewChange` / `NewView` 时验签
- `Request.signature` 在 `txToReq` 处明确写有 `// XXX sign req`，当前未实际填充
- `PrePrepare`、`Prepare`、`Commit`、`Checkpoint`、`NewView` 本身没有独立签名字段

代码依据：

- `consensus/pbft/messages.proto:55` `request.signature`
- `consensus/pbft/messages.proto:109` `view_change.signature`
- `consensus/pbft/batch.go:359` `// XXX sign req`
- `consensus/pbft/sign.go:29` `sign`
- `consensus/pbft/sign.go:44` `verify`
- `consensus/pbft/viewchange.go:174` `instance.sign(vc)`
- `consensus/pbft/viewchange.go:190` `instance.verify(vc)`
- `consensus/pbft/viewchange.go:304` 在 `recvNewView` 中验证 `Vset` 里的 `ViewChange`

### 6. 当前实现中存在 `null request`，但它不是独立结构体

当前工程实现中存在空请求路径，用于主节点在超时场景下维持协议推进。

- `null request` 不是独立 proto 结构体；
- 它通过一个特殊的 `PrePrepare` 表示；
- 其特征是 `batch_digest == ""`，并且 `request_batch == nil`；
- 执行阶段也会专门按“空请求”分支处理，而不是按普通 `RequestBatch` 执行。

代码依据：

- `consensus/pbft/pbft-core.go:549` `nullRequestHandler`
- `consensus/pbft/pbft-core.go:562` `sendPrePrepare(nil, "")`
- `consensus/pbft/pbft-core.go:955` `digest == ""` 的空请求执行分支

## 三、消息证据清单

| 消息/结构 | 真实定义位置 | 发送位置 | 接收/处理位置 | 外层封装关系 | 备注 |
| --- | --- | --- | --- | --- | --- |
| `Transaction` | `protos/fabric.proto:28` | 客户端提交后，经 `core/peer/peer.go:520` 封入 `CHAIN_TRANSACTION` | `core/peer/peer.go:302` `ProcessTransaction`；`consensus/helper/engine.go:47` | `protos.Message(type=CHAIN_TRANSACTION).payload` | 这是业务交易，不是 PBFT 协议消息 |
| `protos.Message` | `protos/fabric.proto:164` | `core/peer/peer.go:530`、`consensus/pbft/batch.go:158`、`168`、`563`、`core/peer/handler.go` 多处 | `consensus/helper/handler.go:79`、`consensus/pbft/batch.go:363` | 最外层统一网络信封 | `signature` 字段在 PBFT 路径未见显式赋值 |
| `Response` | `protos/fabric.proto:194` | `consensus/helper/engine.go:65` / `84` 返回 | `ProcessTransaction` RPC 调用方 | RPC 返回值，不走 PBFT 广播 | 当前工程里的“reply”更接近它；交易类正常入队返回 `SUCCESS + txid`，失败分支会返回 `FAILURE` |
| `Request` | `consensus/pbft/messages.proto:51` | `consensus/pbft/batch.go:349` `txToReq` 生成；`140` `submitToLeader` 广播 | `consensus/pbft/batch.go:381` `batchMsg.GetRequest()` 分支 | `protos.Message(CONSENSUS)` -> `BatchMessage.request` -> `Request` | `signature` 当前未实际填写 |
| `RequestBatch` | `consensus/pbft/messages.proto:130` | `consensus/pbft/batch.go:336` `sendBatch` 本地生成；`pbft-core.go:660` 放入 `PrePrepare`；`1257` 作为 `return_request_batch` 发送 | `pbft-core.go:611`、`701`、`1269` | 既可嵌在 `PrePrepare` 中，也可作为 `return_request_batch` 载荷 | 顶层 `Message.request_batch` 未定位到生产发送逻辑；`null request` 场景下 `PrePrepare.request_batch` 可为 `nil` |
| `BatchMessage` | `consensus/pbft/messages.proto:134` | `batch.go:156` `broadcastMsg`；`560` `wrapMessage` | `batch.go:370` `proto.Unmarshal(ocMsg.Payload, batchMsg)` | `protos.Message(CONSENSUS).payload` | PBFT 批处理层封装 |
| `pbft.Message` | `consensus/pbft/messages.proto:37` | `pbft-core.go` 各 `innerBroadcast` 调用 | `pbft-core.go:566` `recvMsg` | `BatchMessage.pbft_message` 的内部载荷 | 真正的 PBFT 协议多态消息 |
| `PrePrepare` | `consensus/pbft/messages.proto:58` | `pbft-core.go:630` `sendPrePrepare` | `pbft-core.go:701` `recvPrePrepare` | `protos.Message` -> `BatchMessage.pbft_message` -> `pbft.Message.pre_prepare` | 主节点发出；`null request` 场景下 `batch_digest == ""` 且 `request_batch == nil` |
| `Prepare` | `consensus/pbft/messages.proto:66` | `pbft-core.go:759` 广播；`viewchange.go:490` 在新视图恢复阶段补发 | `pbft-core.go:775` `recvPrepare` | 同上 | 正常路径由备节点广播；新视图恢复阶段也可能补发 |
| `Commit` | `consensus/pbft/messages.proto:73` | `pbft-core.go:809` `maybeSendCommit` | `pbft-core.go:827` `recvCommit` | 同上 | 达到 prepared 后广播 |
| `Checkpoint` | `consensus/pbft/messages.proto:85` | `pbft-core.go:969` `Checkpoint` | `pbft-core.go:584` `recvMsg` 识别；`1143` 后续处理 | 同上 | 到达 checkpoint 周期后发送 |
| `ViewChange` | `consensus/pbft/messages.proto:91` | `viewchange.go:125` `sendViewChange` | `viewchange.go:186` `recvViewChange` | 同上 | 带签名，真实验签 |
| `NewView` | `consensus/pbft/messages.proto:116` | `viewchange.go:257` `sendNewView` | `viewchange.go:293` `recvNewView` | 同上 | 自身无签名字段，依赖 `Vset` 中带签名的 `ViewChange` |
| `FetchRequestBatch` | `consensus/pbft/messages.proto:123` | `pbft-core.go:1237` `fetchRequestBatches` | `pbft-core.go:1250` `recvFetchRequestBatch` | `pbft.Message.fetch_request_batch` | 视图切换时补齐缺失批次 |
| `ReturnRequestBatch` | 复用 `RequestBatch`，定义在 `pbft.Message.return_request_batch` | `pbft-core.go:1257` | `pbft-core.go:1269` `recvReturnRequestBatch` | `pbft.Message.return_request_batch = RequestBatch` | 没有单独的 `return_request_batch` 结构体 |
| `Metadata` | `consensus/pbft/messages.proto:145` | `batch.go:218` 序列化后传入 `stack.Execute` | `pbft.go:151` `getLastSeqNo` 解析 | 不经网络广播，作为区块头共识元数据 | 用于记录 `seqNo` |
| `Complaint` | `consensus/pbft/messages.proto:139` | 未定位到生产发送逻辑 | 未定位到生产接收逻辑 | `BatchMessage.complaint` | 仅 proto 预留，当前不应写成已使用消息 |
| `Null request` | 无独立定义，编码为 `PrePrepare` 特例 | `pbft-core.go:562` | `pbft-core.go:955` 空请求执行分支 | `pbft.Message.pre_prepare`，其中 `batch_digest == ""` 且 `request_batch == nil` | 这是工程实现中的特殊协议路径，不应误写成独立消息结构 |

## 四、图中每个箭头到代码的映射

### 1. 客户端 / 某节点 -> 主节点：对应什么结构

严格按当前工程实现，这个箭头不能直接写成“客户端发 `pbft.request` 给主节点”。

真实路径是：

```text
客户端 / NVP
-> RPC ProcessTransaction(Transaction)
-> validator 本地构造 protos.Message(type=CHAIN_TRANSACTION, payload=marshal(Transaction))
-> PBFT 把它转换成 Request
-> 广播 protos.Message(type=CONSENSUS, payload=marshal(BatchMessage{request}))
-> 主节点作为全体副本之一收到 Request
```

代码依据：

- `protos/fabric.proto:126` `ProcessTransaction`
- `core/peer/peer.go:302` `ProcessTransaction`
- `core/peer/peer.go:643` `ExecuteTransaction`
- `core/peer/peer.go:520` `sendTransactionsToLocalEngine`
- `consensus/helper/engine.go:47` `ProcessTransactionMsg`
- `consensus/pbft/external.go:58` `RecvMsg`
- `consensus/pbft/batch.go:363` `processMessage`
- `consensus/pbft/batch.go:349` `txToReq`
- `consensus/pbft/batch.go:140` `submitToLeader`

结论：

- 图里的 `request` 箭头，在当前工程里对应的是“交易 -> Request -> 广播到全体副本”，不是一个只发给主节点的独立 PBFT 单播结构。

### 2. 主节点 -> 各副本（pre-prepare）：对应什么结构

真实结构是：

```text
protos.Message(type=CONSENSUS)
-> BatchMessage.pbft_message
-> pbft.Message.pre_prepare
-> PrePrepare
```

代码依据：

- `consensus/pbft/pbft-core.go:630` `sendPrePrepare`
- `consensus/pbft/pbft-core.go:654` 主节点广播 pre-prepare
- `consensus/pbft/batch.go:560` `wrapMessage`
- `consensus/pbft/pbft-core.go:701` `recvPrePrepare`

### 3. 备节点 -> 全体广播（prepare）：对应什么结构

真实结构是：

```text
protos.Message(type=CONSENSUS)
-> BatchMessage.pbft_message
-> pbft.Message.prepare
-> Prepare
```

代码依据：

- `consensus/pbft/pbft-core.go:759` 广播 `Prepare`
- `consensus/pbft/pbft-core.go:775` `recvPrepare`
- `consensus/pbft/batch.go:560` `wrapMessage`

### 4. 各副本 -> 全体广播（commit）：对应什么结构

真实结构是：

```text
protos.Message(type=CONSENSUS)
-> BatchMessage.pbft_message
-> pbft.Message.commit
-> Commit
```

代码依据：

- `consensus/pbft/pbft-core.go:809` `maybeSendCommit`
- `consensus/pbft/pbft-core.go:812` 广播 `Commit`
- `consensus/pbft/pbft-core.go:827` `recvCommit`
- `consensus/pbft/batch.go:560` `wrapMessage`

### 5. 节点 -> 客户端（reply）：对应什么结构或函数返回路径

当前工程实现中，图中的 `reply` 不能映射为独立 PBFT 协议结构体。

真实情况如下：

- `messages.proto` 中没有 `reply`
- `pbft-core.go` / `batch.go` 中未定位到“commit 完成后构造 reply 报文并回发客户端”的逻辑
- 工程返回路径是 `pb.Response`
- 对交易请求，`ProcessTransactionMsg` 在把消息送入 PBFT 前就先返回 `SUCCESS + txid`
- 对查询请求，直接同步执行并返回查询结果

代码依据：

- `protos/fabric.proto:194` `Response`
- `core/peer/peer.go:302` `ProcessTransaction`
- `core/peer/peer.go:504` `SendTransactionsToPeer`
- `core/peer/peer.go:520` `sendTransactionsToLocalEngine`
- `consensus/helper/engine.go:47` `ProcessTransactionMsg`
- `consensus/helper/helper.go:338` `Commit`
- `consensus/helper/helper.go:364` `Committed`

结论：

- 教材图中的 `reply`，在当前工程里应写成“工程返回路径差异”，不能伪造成一个不存在的 `pbft.reply`。

## 五、字段使用状态核对

### 1. 明确定义且当前实现中有直接使用证据的字段

- `Request.timestamp`
  - 由 `txToReq` 填充
  - 被 `deduplicator.IsNew/Execute` 用于判重与新旧判断
  - 代码依据：`consensus/pbft/batch.go:349`，`consensus/pbft/deduplicator.go:44/58/69`
- `Request.replica_id`
  - 由 `txToReq` 填充
  - 被日志与判重逻辑使用
  - 代码依据：`batch.go:357`，`deduplicator.go:46/47/60/71`
- `PrePrepare.view/sequence_number/batch_digest/request_batch/replica_id`
  - 在 `recvPrePrepare` 中被完整检查和使用
- `Prepare.view/sequence_number/batch_digest/replica_id`
  - 在 `recvPrepare` / `maybeSendCommit` 中使用
- `Commit.view/sequence_number/batch_digest/replica_id`
  - 在 `recvCommit` / `committed` 判断中使用
- `Checkpoint.sequence_number/replica_id/id`
  - 在 checkpoint 证书与状态转移判断中使用
- `ViewChange.signature`
  - 真实签名与验签
- `NewView.vset/xset`
  - 在 `processNewView` 中用于重建新视图状态
- `FetchRequestBatch.batch_digest/replica_id`
  - 用于请求缺失批次
- `Metadata.seqNo`
  - 作为共识元数据存取

### 2. 已定义但当前实现中未见完整使用逻辑的字段或结构

- `Request.signature`
  - proto 中定义，但 `txToReq` 明确未签名
- `BatchMessage.complaint`
  - 仅见 proto 与生成代码，未见生产构造/处理逻辑
- `BatchMessage.request_batch`
  - proto 中存在，但未定位到生产发送路径
- `pbft.Message.request_batch` 顶层 oneof
  - 生产代码未定位到发送路径，当前主要通过本地事件或嵌套字段使用 `RequestBatch`
- `protos.Message.signature`
  - 在 PBFT 路径的构造代码中未见显式赋值

## 六、补充：状态同步消息是否应纳入文档

应纳入“补充消息”章节，但要明确它们不属于 `consensus/pbft/messages.proto` 中的核心 PBFT 协议消息，而是 PBFT 触发状态转移后，底层 Peer 层使用的同步消息。

相关消息位于 `protos/fabric.proto`：

- `SYNC_GET_BLOCKS`
- `SYNC_BLOCKS`
- `SYNC_STATE_GET_SNAPSHOT`
- `SYNC_STATE_SNAPSHOT`
- `SYNC_STATE_GET_DELTAS`
- `SYNC_STATE_DELTAS`

发送代码位于 `core/peer/handler.go`：

- `321` `RequestBlocks`
- `334` 发送 `SYNC_GET_BLOCKS`
- `429` 发送 `SYNC_BLOCKS`
- `458` 发送 `SYNC_STATE_GET_SNAPSHOT`
- `561/574` 发送 `SYNC_STATE_SNAPSHOT`
- `604` 发送 `SYNC_STATE_GET_DELTAS`
- `667` 发送 `SYNC_STATE_DELTAS`

PBFT 触发入口：

- `consensus/pbft/pbft.go:136` `op.stack.UpdateState(...)`
- `consensus/pbft/pbft-core.go:907` `instance.consumer.skipTo(...)`

## 七、可直接带入正式文档的总括结论

1. 当前工程中的 PBFT 报文体系是分层封装的，至少要区分 `Transaction`、`protos.Message`、`BatchMessage`、`pbft.Message`、具体协议结构体。
2. 教材图中的 `request` 箭头，在当前工程中实现为“交易进入验证节点后转为 `Request`，再广播到全体副本”，不是简单的“客户端单播主节点”。
3. 教材图中的 `reply` 箭头，在当前工程中没有对应的独立 PBFT proto 报文，真实落点是 `pb.Response` 返回路径，而且交易请求是“先回 txid，再异步共识”。
4. `ViewChange.signature` 有真实签名逻辑；`Request.signature` 当前基本属于预留字段。
5. `complaint`、`BatchMessage.request_batch`、`pbft.Message.request_batch` 顶层 oneof 不能写成当前生产路径中正在使用的正式消息。
