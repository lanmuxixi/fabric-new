# 当前项目 PBFT 报文结构体详细说明（基于代码实现）

## 一、文档目的

本章先限定本文档的边界和阅读方式。本文讨论的是“当前仓库中的真实工程实现”，而不是对 PBFT 论文的抽象性复述；因此在后续分析 `request`、`reply`、`request_batch` 等结构时，均以代码证据为准，而不预设论文图示中的默认语义。

本文档用于说明**当前仓库代码实现中的** PBFT 报文结构体系，目标是回答如下问题：

1. 论文示意图中的 `request / pre-prepare / prepare / commit / reply` 在当前工程里分别落到哪些真实结构体与函数路径；
2. 当前工程中消息到底有几层封装，哪些是网络信封，哪些是批处理封装，哪些才是 PBFT 协议消息；
3. 哪些字段在当前实现中有真实使用，哪些虽然定义在 proto 中，但当前只是预留、透传，或未定位到直接使用逻辑；
4. 当前工程实现与标准 PBFT 论文示意图有哪些一致点和差异点。

本文档**不是论文内容的转述版 PBFT 说明**，所有结论均以当前仓库中的真实代码为准。

代码依据：

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

## 二、总体现实结构

本章用于建立全局视角，解决“消息到底分几层、每层分别是什么”的问题。后续章节会反复引用这里给出的分层框架，因此若先明确 `Transaction`、`protos.Message`、`BatchMessage`、`pbft.Message` 之间的关系，后面的字段说明会更容易把握。

### 2.1 当前代码中的消息不是单层结构

当前工程里，PBFT 相关消息至少分为四类概念层：

1. **业务交易层**：`protos.Transaction`
2. **网络统一信封层**：`protos.Message`
3. **PBFT 批处理封装层**：`pbft.BatchMessage`
4. **PBFT 协议消息层**：`pbft.Message` 与其内部的 `PrePrepare / Prepare / Commit / ...`

其真实关系如下：

```text
业务调用
-> Transaction
-> protos.Message(type=CHAIN_TRANSACTION)
-> Request
-> protos.Message(type=CONSENSUS)
-> BatchMessage
-> pbft.Message
-> PrePrepare / Prepare / Commit / Checkpoint / ViewChange / NewView / ...
```

代码依据：

- `protos/fabric.proto` 中的 `Transaction`、`Message`、`Response`
- `consensus/pbft/messages.proto` 中的 `request`、`batch_message`、`message`
- `core/peer/peer.go:520` `sendTransactionsToLocalEngine`
- `consensus/pbft/batch.go:349` `txToReq`
- `consensus/pbft/batch.go:140` `submitToLeader`
- `consensus/pbft/batch.go:560` `wrapMessage`

### 2.2 当前工程中的“request”与论文示意图并不完全一致

论文示意图通常画成“客户端向主节点发送 request”。  
当前工程中，真实路径并不是“客户端直接发一个 `pbft.request` 给主节点”，而是：

1. 客户端先向某个 Peer 调用 `ProcessTransaction(Transaction)`；
2. Validator 将 `Transaction` 包成 `protos.Message(type=CHAIN_TRANSACTION)`；
3. PBFT 层收到后，调用 `txToReq` 生成 `pbft.Request`；
4. `submitToLeader` 再把 `BatchMessage.request` **广播给全体副本**；
5. 主节点作为副本之一收到该 `Request`，随后聚合为 `RequestBatch`。

因此，论文示意图中的“request 箭头”在工程实现里对应的是一个**业务交易入口 + PBFT Request 广播**的组合路径。

### 2.3 当前工程中的“reply”不是独立 PBFT 报文

在 `consensus/pbft/messages.proto` 中，没有定义 `reply` 消息。  
当前工程的“返回调用方”采用的是 `pb.Response`：

- 对**交易类请求**：正常入队时通常返回 `SUCCESS + txid`，然后异步进入共识与执行；
- 但如果交易预校验失败、共识引擎未初始化，或 `RecvMsg` 返回错误，则会直接返回 `FAILURE`；
- 对**查询类请求**：同步执行链码并直接返回查询结果。

因此，论文示意图中的 `reply` 箭头在当前工程里应解释为**工程级 RPC 返回路径**，而不是一个独立的 PBFT 协议消息。

代码依据：

- `protos/fabric.proto:194` `Response`
- `protos/fabric.proto:126` `rpc ProcessTransaction(Transaction) returns (Response)`
- `core/peer/peer.go:302` `ProcessTransaction`
- `consensus/helper/engine.go:47` `ProcessTransactionMsg`

## 三、图中每个箭头对应的真实报文

本章用于把参考图中的每一条箭头直接映射到代码中的真实消息和函数路径，作用是先建立“图示与代码”的对应关系。若需要快速回答某一条箭头对应什么消息，可以先从本章定位，再回到第四章查看该消息的完整结构和字段细节。

### 3.1 某个节点发给主节点的报文结构体

#### 真实结论

当前工程里，这一箭头不能简单写成“`Request` 直接发给主节点”。  
真实代码路径是：

```text
Transaction
-> protos.Message(type=CHAIN_TRANSACTION)
-> Request
-> protos.Message(type=CONSENSUS)
-> BatchMessage.request
-> 广播给全体副本
-> 主节点收到后聚合
```

#### 代码依据

- `core/peer/peer.go:520` `sendTransactionsToLocalEngine`
- `consensus/helper/engine.go:47` `ProcessTransactionMsg`
- `consensus/pbft/external.go:58` `RecvMsg`
- `consensus/pbft/batch.go:363` `processMessage`
- `consensus/pbft/batch.go:349` `txToReq`
- `consensus/pbft/batch.go:140` `submitToLeader`
- `consensus/pbft/batch.go:156` `broadcastMsg`

### 3.2 主节点发出的消息结构体

#### 真实结论

主节点发出的 `pre-prepare`，真实封装关系为：

```text
protos.Message(type=CONSENSUS)
-> BatchMessage.pbft_message
-> pbft.Message.pre_prepare
-> PrePrepare
```

#### 代码依据

- `consensus/pbft/pbft-core.go:630` `sendPrePrepare`
- `consensus/pbft/pbft-core.go:654` 主节点广播 pre-prepare
- `consensus/pbft/batch.go:560` `wrapMessage`
- `consensus/pbft/pbft-core.go:701` `recvPrePrepare`

### 3.3 每一个节点广播时的消息结构体

#### Prepare 广播

```text
protos.Message(type=CONSENSUS)
-> BatchMessage.pbft_message
-> pbft.Message.prepare
-> Prepare
```

代码依据：

- `consensus/pbft/pbft-core.go:759`
- `consensus/pbft/pbft-core.go:775`

#### Commit 广播

```text
protos.Message(type=CONSENSUS)
-> BatchMessage.pbft_message
-> pbft.Message.commit
-> Commit
```

代码依据：

- `consensus/pbft/pbft-core.go:809`
- `consensus/pbft/pbft-core.go:827`

### 3.4 节点最终返回结果时的消息结构体或返回路径

#### 真实结论

当前工程中，没有独立的 `pbft.reply` 结构体。  
真实路径如下：

- 入口 RPC：`ProcessTransaction(Transaction) returns (Response)`
- 验证节点处理交易时，由 `EngineImpl.ProcessTransactionMsg` 直接构造 `pb.Response`
- 对交易类请求，正常路径通常返回 `SUCCESS + txid`，但存在显式 `FAILURE` 分支
- 共识执行与账本提交在此之后异步进行

#### 代码依据

- `protos/fabric.proto:126` `ProcessTransaction`
- `protos/fabric.proto:194` `Response`
- `core/peer/peer.go:302` `ProcessTransaction`
- `core/peer/peer.go:643` `ExecuteTransaction`
- `core/peer/peer.go:504` `SendTransactionsToPeer`
- `consensus/helper/engine.go:47` `ProcessTransactionMsg`

### 3.5 PBFT 关键补充消息结构体

除图中的五个箭头外，当前工程中还应补充以下关键消息：

- `RequestBatch`
- `Checkpoint`
- `ViewChange`
- `NewView`
- `FetchRequestBatch`
- `ReturnRequestBatch`
- `Metadata`
- 状态同步消息：`SYNC_GET_BLOCKS / SYNC_BLOCKS / SYNC_STATE_*`

这些消息中的一部分属于 PBFT 核心协议，一部分属于工程实现为保证状态恢复、视图切换、补批和状态同步而增加的实际消息。

此外，当前实现还存在一个需要单独说明的协议特例：

- `null request`

它不是独立消息结构体，而是通过特殊的 `PrePrepare` 编码出来的空请求。

## 四、每一种报文的完整结构

本章面向导师直接说明“当前实现里到底有哪些消息、它们分别处在什么层次、字段是否真实参与运行”。为避免上一版按名字平铺时产生“先后顺序和封装顺序混在一起”的理解负担，本章改为“**功能分组 + 组内尽量按进入系统时的封装方向排列**”的组织方式。需要特别说明的是，个别结构如 `RequestBatch` 同时具有“业务批次载荷”和“协议消息内容”两种属性，本文统一按其**主要运行职责**归入对应分组。

### 4.1 本章总览矩阵

为便于快速把握结构，本章先给出一张总览矩阵：

| 分组 | 代表结构 | 主要作用 | 典型所在位置 |
| --- | --- | --- | --- |
| 第一组：外层网络与返回路径 | `protos.Message`、`Response` | 承载网络发送与 RPC 返回 | Peer 网络层、RPC 层 |
| 第二组：业务请求入口 | `Transaction`、`Request` | 把业务调用转成 PBFT 可处理请求 | 业务入口到共识入口 |
| 第三组：PBFT 请求封装与批处理 | `BatchMessage`、`pbft.Message`、`RequestBatch` | 完成 PBFT 内部封装与批次承载 | 共识消息中间层 |
| 第四组：PBFT 正常共识消息 | `PrePrepare`、`Prepare`、`Commit`、`null request` | 完成排序、确认、提交 | PBFT 主流程 |
| 第五组：PBFT 视图切换与恢复消息 | `Checkpoint`、`ViewChange`、`NewView`、`FetchRequestBatch`、`ReturnRequestBatch` | 支持稳定检查点、主节点切换、缺批恢复 | PBFT 恢复路径 |
| 第六组：执行状态、同步与预留补充消息 | `Metadata`、状态同步消息、`Complaint` | 支持执行状态恢复、跨节点追赶、预留扩展 | 账本元数据、Peer 同步层 |

以下各节均按“定义位置 / 发送位置 / 接收位置 / 外层封装 / 结构 / 字段说明”的模板展开。

### 4.2 第一组：外层网络与返回路径

这一组用于回答两个基础问题：第一，PBFT 相关消息在网络上传输时最外层到底包成什么；第二，论文中的 `reply` 在当前工程里具体落在哪里。理解这一组之后，后续章节中出现的所有“广播”“单播”“返回”就都有了明确载体。

#### 4.2.1 最外层网络统一信封：`protos.Message`

**消息名称**：`protos.Message`  
**代码定义位置**：`protos/fabric.proto` `message Message`  
**发送位置**：

- `core/peer/peer.go:530` 发送 `CHAIN_TRANSACTION`
- `consensus/pbft/batch.go:158/168/563` 发送 `CONSENSUS`
- `core/peer/handler.go` 多处发送 `SYNC_*`

**接收/处理位置**：

- `consensus/helper/handler.go:79` 处理 `CONSENSUS`
- `consensus/pbft/batch.go:363` 进一步解析

**外层封装关系**：所有 PBFT 相关网络消息的最外层统一信封  

**结构体 / proto 原文**：

```proto
message Message {
    enum Type {
        CHAIN_TRANSACTION = 6;
        CONSENSUS = 21;
    }
    Type type = 1;
    google.protobuf.Timestamp timestamp = 2;
    bytes payload = 3;
    bytes signature = 4;
}
```

**字段逐项说明**：

- `type`：消息类型；当前 PBFT 入口主要用到 `CHAIN_TRANSACTION` 与 `CONSENSUS`。
- `timestamp`：外层消息时间戳；`sendTransactionsToLocalEngine` 构造 `CHAIN_TRANSACTION` 时会填充。
- `payload`：真正业务载荷；可能是序列化后的 `Transaction`、`BatchMessage` 或状态同步结构。
- `signature`：外层信封签名字段；在当前 PBFT 路径的构造代码中未见显式赋值，因此不能直接表述为“PBFT 外层消息统一签名”。

**代码依据**：

- `protos/fabric.proto:164`
- `core/peer/peer.go:530`
- `consensus/pbft/batch.go:158/168/563`

#### 4.2.2 `Response` 与论文中 `reply` 的工程对应关系

**消息名称**：`Response`  
**代码定义位置**：`protos/fabric.proto` `message Response`  
**发送位置**：

- `consensus/helper/engine.go:47` `ProcessTransactionMsg`

**接收/处理位置**：

- 调用 `ProcessTransaction` 的上层 RPC 客户端

**外层封装关系**：RPC 返回值，不在 `CONSENSUS` 网络广播内  

**结构体 / proto 原文**：

```proto
message Response {
    enum StatusCode {
        SUCCESS = 200;
        FAILURE = 500;
    }
    StatusCode status = 1;
    bytes msg = 2;
}
```

**字段逐项说明**：

- `status`：返回状态码。
- `msg`：返回内容；交易类请求通常放 `txid`，查询类请求放查询结果。

**特别说明**：

- 当前工程里，论文中 `reply` 的真实对应物是 `Response` 返回路径；
- 它不是 `consensus/pbft/messages.proto` 中的独立 PBFT 协议消息；
- 对交易请求，返回发生在 `eng.consenter.RecvMsg(...)` 前后紧邻位置，本质上是“入队成功/失败”的工程返回，而不是“共识提交成功”回执；
- 对交易请求，正常路径通常返回 `SUCCESS + txid`，但失败分支会返回 `FAILURE`。

**代码依据**：

- `protos/fabric.proto:194`
- `consensus/helper/engine.go:65/84`

### 4.3 第二组：业务请求入口

这一组用于说明业务层请求是如何进入 PBFT 的。它回答的核心问题不是“协议里有哪些消息”，而是“客户端提交的一笔交易是如何先变成工程交易对象，再被转换为 PBFT 请求对象的”。

#### 4.3.1 业务交易：`Transaction`

**消息名称**：`Transaction`  
**代码定义位置**：`protos/fabric.proto` `message Transaction`  
**发送位置**：

- 客户端 RPC 调用 `ProcessTransaction(Transaction)`
- `core/peer/peer.go:520` 把它序列化为 `CHAIN_TRANSACTION`

**接收/处理位置**：

- `core/peer/peer.go:302` `ProcessTransaction`
- `consensus/helper/engine.go:47` `ProcessTransactionMsg`

**外层封装关系**：

```text
Transaction
-> protos.Message(type=CHAIN_TRANSACTION).payload
```

**结构体 / proto 原文**：

```proto
message Transaction {
    Type type = 1;
    bytes chaincodeID = 2;
    bytes payload = 3;
    bytes metadata = 4;
    string txid = 5;
    google.protobuf.Timestamp timestamp = 6;
    bytes nonce = 9;
    bytes toValidators = 10;
    bytes cert = 11;
    bytes signature = 12;
}
```

**字段逐项说明**：

- `type`：交易类型；可为部署、调用、查询等。
- `chaincodeID`：目标链码标识。
- `payload`：链码调用参数等业务数据；后续整体放进 `Request.payload` 中。
- `metadata`：附加元数据；当前 PBFT 主流程中未见专门解析逻辑。
- `txid`：交易标识；在交易类返回路径中会作为 `pb.Response.Msg` 返回。
- `timestamp`：交易时间戳。
- `nonce`：随机数/抗重放字段；属于交易层，不是 PBFT 协议字段。
- `toValidators`：给验证节点的数据；当前 PBFT 主流程中未见专门解析逻辑。
- `cert`：证书数据；由安全模块使用。
- `signature`：交易签名；在 `ProcessTransaction` 中可能先做交易预校验，再进入 PBFT。

**代码依据**：

- `protos/fabric.proto:28`
- `core/peer/peer.go:302`

#### 4.3.2 PBFT 请求：`Request`

**消息名称**：`Request`  
**代码定义位置**：`consensus/pbft/messages.proto` `message request`  
**发送位置**：

- `consensus/pbft/batch.go:349` `txToReq` 生成
- `consensus/pbft/batch.go:140` `submitToLeader`
- `consensus/pbft/batch.go:156` `broadcastMsg`

**接收/处理位置**：

- `consensus/pbft/batch.go:381` `batchMsg.GetRequest()` 分支

**外层封装关系**：

```text
protos.Message(type=CONSENSUS)
-> BatchMessage.request
-> Request
```

**结构体 / proto 原文**：

```proto
message request {
    google.protobuf.Timestamp timestamp = 1;
    bytes payload = 2;
    uint64 replica_id = 3;
    bytes signature = 4;
}
```

**字段逐项说明**：

- `timestamp`：请求进入 PBFT 时生成的时间戳；当前实现中用于去重与新旧判断。  
  代码依据：`batch.go:349`，`deduplicator.go:44/58/69`
- `payload`：不透明载荷；当前实现里存放的是 `marshal(Transaction)` 的字节数组。  
  代码依据：`txToReq` 的参数 `tx []byte`
- `replica_id`：当前构造该请求的副本编号；用于日志、去重和来源标识。  
  代码依据：`batch.go:357`，`deduplicator.go`
- `signature`：请求签名字段；proto 中定义了，但 `txToReq` 处明确写有 `// XXX sign req`，当前实现中未完成签名填充。  
  代码依据：`batch.go:359`

**代码依据**：

- `consensus/pbft/messages.proto:51`
- `consensus/pbft/batch.go:349`
- `consensus/pbft/deduplicator.go`

### 4.4 第三组：PBFT 请求封装与批处理

这一组用于说明 PBFT 主流程中的“中间层”结构。它们本身不是最终的 `PrePrepare / Prepare / Commit`，但没有这些封装层，就无法解释为什么代码里一个逻辑消息往往对应多层对象嵌套。

#### 4.4.1 PBFT 批处理封装：`BatchMessage`

**消息名称**：`BatchMessage`  
**代码定义位置**：`consensus/pbft/messages.proto` `message batch_message`  
**发送位置**：

- `consensus/pbft/batch.go:156` `broadcastMsg`
- `consensus/pbft/batch.go:560` `wrapMessage`

**接收/处理位置**：

- `consensus/pbft/batch.go:370` 对 `ocMsg.Payload` 反序列化

**外层封装关系**：

```text
protos.Message(type=CONSENSUS)
-> BatchMessage
```

**结构体 / proto 原文**：

```proto
message batch_message {
    oneof payload {
        request request = 1;
        request_batch request_batch = 2;
        bytes pbft_message = 3;
        request complaint = 4;
    }
}
```

**字段逐项说明**：

- `request`：普通请求入口；当前生产代码真实使用。
- `request_batch`：批量请求 oneof 变体；当前生产代码未定位到直接发送逻辑。
- `pbft_message`：内部再封一层 `pbft.Message`；正式 `PrePrepare / Prepare / Commit / ...` 都走这里。
- `complaint`：注释写为“like request, but processed everywhere”；但当前生产代码未定位到构造或处理逻辑，应视为预留结构。

**代码依据**：

- `consensus/pbft/messages.proto:134`
- `consensus/pbft/batch.go:156/560`
- `consensus/pbft/batch.go:381/403`

#### 4.4.2 PBFT 顶层多态消息：`pbft.Message`

**消息名称**：`pbft.Message`  
**代码定义位置**：`consensus/pbft/messages.proto` `message message`  
**发送位置**：

- `pbft-core.go` 中所有 `innerBroadcast(&Message{Payload: ...})`

**接收/处理位置**：

- `consensus/pbft/pbft-core.go:566` `recvMsg`
- `consensus/pbft/pbft-core.go:322` `ProcessEvent`

**外层封装关系**：

```text
BatchMessage.pbft_message
-> pbft.Message
-> oneof { pre_prepare / prepare / commit / checkpoint / ... }
```

**结构体 / proto 原文**：

```proto
message message {
    oneof payload {
        request_batch request_batch = 1;
        pre_prepare pre_prepare = 2;
        prepare prepare = 3;
        commit commit = 4;
        checkpoint checkpoint = 5;
        view_change view_change = 6;
        new_view new_view = 7;
        fetch_request_batch fetch_request_batch = 8;
        request_batch return_request_batch = 9;
    }
}
```

**字段逐项说明**：

- `request_batch`：顶层 oneof 变体；当前生产代码未定位到直接发送逻辑。
- `pre_prepare`：主节点发起排序阶段时使用。
- `prepare`：备节点确认阶段使用。
- `commit`：提交确认阶段使用。
- `checkpoint`：定期做稳定检查点与状态转移判定使用。
- `view_change`：视图切换请求。
- `new_view`：新主节点发出的新视图建立消息。
- `fetch_request_batch`：视图切换后请求缺失批次。
- `return_request_batch`：返回缺失批次；复用 `RequestBatch` 结构。

**代码依据**：

- `consensus/pbft/messages.proto:37`
- `consensus/pbft/pbft-core.go:566`

#### 4.4.3 请求批：`RequestBatch`

**消息名称**：`RequestBatch`  
**代码定义位置**：`consensus/pbft/messages.proto` `message request_batch`  
**发送位置**：

- `consensus/pbft/batch.go:336` `sendBatch` 本地生成
- `consensus/pbft/pbft-core.go:660` 作为 `PrePrepare.request_batch`
- `consensus/pbft/pbft-core.go:1257` 作为 `return_request_batch`

**接收/处理位置**：

- `consensus/pbft/pbft-core.go:611` `recvRequestBatch`
- `consensus/pbft/pbft-core.go:744` 从 `PrePrepare` 里取出
- `consensus/pbft/pbft-core.go:1269` `recvReturnRequestBatch`

**外层封装关系**：

有三种真实出现方式：

```text
1. 本地事件：RequestBatch -> pbftCore.ProcessEvent
2. 嵌入字段：PrePrepare.request_batch
3. 返回消息：pbft.Message.return_request_batch = RequestBatch
```

**结构体 / proto 原文**：

```proto
message request_batch {
    repeated request batch = 1;
}
```

**字段逐项说明**：

- `batch`：请求列表；当前实现中是一组 `Request`，由主节点从 `batchStore` 聚合得到。

**特别说明**：

- `RequestBatch` 在当前实现中确实被使用；
- 但 `null request` 场景下不会构造普通 `RequestBatch`，此时 `PrePrepare.request_batch` 可为 `nil`；
- `pbft.Message` 顶层的 `request_batch` oneof 变体，在生产代码中未定位到直接发送逻辑；
- `BatchMessage.request_batch` 变体同样未定位到生产发送逻辑。

**代码依据**：

- `consensus/pbft/messages.proto:130`
- `consensus/pbft/batch.go:336`
- `consensus/pbft/pbft-core.go:611/1257/1269`

### 4.5 第四组：PBFT 正常共识消息

这一组对应 PBFT 主流程中最核心的三阶段消息，以及当前实现中与 `PrePrepare` 绑定的空请求特例。对于导师汇报而言，这一组是解释“正常排序如何推进”的主干。

#### 4.5.1 `PrePrepare`

**消息名称**：`PrePrepare`  
**代码定义位置**：`consensus/pbft/messages.proto` `message pre_prepare`  
**发送位置**：

- `consensus/pbft/pbft-core.go:630` `sendPrePrepare`

**接收/处理位置**：

- `consensus/pbft/pbft-core.go:701` `recvPrePrepare`

**外层封装关系**：

```text
protos.Message(CONSENSUS)
-> BatchMessage.pbft_message
-> pbft.Message.pre_prepare
-> PrePrepare
```

**结构体 / proto 原文**：

```proto
message pre_prepare {
    uint64 view = 1;
    uint64 sequence_number = 2;
    string batch_digest = 3;
    request_batch request_batch = 4;
    uint64 replica_id = 5;
}
```

**字段逐项说明**：

- `view`：视图号；接收端会校验与当前视图关系。
- `sequence_number`：序列号；接收端会校验是否落在水位窗口内。
- `batch_digest`：请求批摘要；普通请求批场景下，接收端会校验与 `request_batch` 内容哈希是否一致；若该字段为空字符串，则表示 `null request`。
- `request_batch`：请求批本体；普通请求批场景下会把它存入 `reqBatchStore`，供后续 `Prepare / Commit` 和执行使用；在 `null request` 场景下该字段可为 `nil`。
- `replica_id`：发送者副本编号；接收端要求它必须等于当前视图主节点编号。

**代码依据**：

- `consensus/pbft/messages.proto:58`
- `consensus/pbft/pbft-core.go:654`
- `consensus/pbft/pbft-core.go:701`

#### 4.5.2 `Prepare`

**消息名称**：`Prepare`  
**代码定义位置**：`consensus/pbft/messages.proto` `message prepare`  
**发送位置**：

- `consensus/pbft/pbft-core.go:759`

**接收/处理位置**：

- `consensus/pbft/pbft-core.go:775`

**外层封装关系**：与 `PrePrepare` 相同  

**结构体 / proto 原文**：

```proto
message prepare {
    uint64 view = 1;
    uint64 sequence_number = 2;
    string batch_digest = 3;
    uint64 replica_id = 4;
}
```

**字段逐项说明**：

- `view`：当前视图号。
- `sequence_number`：当前排序序列号。
- `batch_digest`：确认的请求批摘要。
- `replica_id`：发送 `Prepare` 的备节点编号；主节点发送的 `Prepare` 会被忽略。

**特别说明**：

- 正常路径下，`Prepare` 是备节点在 `recvPrePrepare` 中发送；
- 在 `new-view` 恢复阶段，非主节点也会依据 `Xset` 补发对应的 `Prepare`，以恢复新视图下的 prepared 状态。

**代码依据**：

- `consensus/pbft/messages.proto:66`
- `consensus/pbft/pbft-core.go:759/775`
- `consensus/pbft/viewchange.go:490`

#### 4.5.3 `Commit`

**消息名称**：`Commit`  
**代码定义位置**：`consensus/pbft/messages.proto` `message commit`  
**发送位置**：

- `consensus/pbft/pbft-core.go:809` `maybeSendCommit`

**接收/处理位置**：

- `consensus/pbft/pbft-core.go:827` `recvCommit`

**外层封装关系**：与 `PrePrepare` 相同  

**结构体 / proto 原文**：

```proto
message commit {
    uint64 view = 1;
    uint64 sequence_number = 2;
    string batch_digest = 3;
    uint64 replica_id = 4;
}
```

**字段逐项说明**：

- `view`：当前视图号。
- `sequence_number`：序列号。
- `batch_digest`：请求批摘要。
- `replica_id`：发送 `Commit` 的副本编号。

**代码依据**：

- `consensus/pbft/messages.proto:73`
- `consensus/pbft/pbft-core.go:809/827`

#### 4.5.4 `null request`

**消息名称**：`null request`（工程语义，不是独立 proto 结构）  
**代码定义位置**：无独立定义；通过 `PrePrepare` 特例编码  
**发送位置**：

- `consensus/pbft/pbft-core.go:549` `nullRequestHandler`
- `consensus/pbft/pbft-core.go:562` `sendPrePrepare(nil, "")`

**接收/处理位置**：

- 仍经 `recvPrePrepare`
- 执行阶段在 `digest == ""` 分支中识别

**外层封装关系**：

```text
protos.Message(CONSENSUS)
-> BatchMessage.pbft_message
-> pbft.Message.pre_prepare
-> PrePrepare
   其中 batch_digest == ""，request_batch == nil
```

**结构体 / proto 原文**：

无独立 proto；复用 `PrePrepare`。

**字段逐项说明**：

- `batch_digest == ""`：表示这是空请求，不对应普通请求批摘要。
- `request_batch == nil`：表示没有真实请求批载荷。
- `replica_id`：仍是主节点编号。

**特别说明**：

- 这是当前工程实现中的协议特例；
- 文档中不能把它误写成一个单独的新结构体，也不能忽略这条真实路径。

**代码依据**：

- `consensus/pbft/pbft-core.go:549`
- `consensus/pbft/pbft-core.go:562`
- `consensus/pbft/pbft-core.go:955`

### 4.6 第五组：PBFT 视图切换与恢复消息

这一组对应主流程之外的恢复路径。它们的作用不是正常推进每一笔交易，而是在主节点故障、消息缺失或检查点推进时，维持系统继续达成一致。

#### 4.6.1 `Checkpoint`

**消息名称**：`Checkpoint`  
**代码定义位置**：`consensus/pbft/messages.proto` `message checkpoint`  
**发送位置**：

- `consensus/pbft/pbft-core.go:969` `Checkpoint`

**接收/处理位置**：

- `consensus/pbft/pbft-core.go:1144` 后续 checkpoint 处理逻辑

**外层封装关系**：`pbft.Message.checkpoint`  

**结构体 / proto 原文**：

```proto
message checkpoint {
    uint64 sequence_number = 1;
    uint64 replica_id = 2;
    string id = 3;
}
```

**字段逐项说明**：

- `sequence_number`：检查点对应的序列号；必须是 checkpoint 周期 `K` 的倍数。
- `replica_id`：发送 checkpoint 的副本编号。
- `id`：检查点状态摘要；当前实现中为区块链信息快照的 Base64 字符串。

**代码依据**：

- `consensus/pbft/messages.proto:85`
- `consensus/pbft/pbft-core.go:969/1144`

#### 4.6.2 `ViewChange`

**消息名称**：`ViewChange`  
**代码定义位置**：`consensus/pbft/messages.proto` `message view_change`  
**发送位置**：

- `consensus/pbft/viewchange.go:125` `sendViewChange`

**接收/处理位置**：

- `consensus/pbft/viewchange.go:186` `recvViewChange`

**外层封装关系**：`pbft.Message.view_change`  

**结构体 / proto 原文**：

```proto
message view_change {
    message C {
        uint64 sequence_number = 1;
        string id = 3;
    }
    message PQ {
        uint64 sequence_number = 1;
        string batch_digest = 2;
        uint64 view = 3;
    }

    uint64 view = 1;
    uint64 h = 2;
    repeated C cset = 3;
    repeated PQ pset = 4;
    repeated PQ qset = 5;
    uint64 replica_id = 6;
    bytes signature = 7;
}
```

**字段逐项说明**：

- `view`：目标新视图号。
- `h`：当前低水位。
- `cset`：检查点集合；每个元素记录一个稳定或可用检查点。
- `pset`：prepared 证据集合。
- `qset`：pre-prepared 证据集合。
- `replica_id`：发起视图切换的副本编号。
- `signature`：视图切换消息签名；当前实现中存在真实签名与验签逻辑。

**内部字段说明**：

- `C.sequence_number`：该检查点的序列号。
- `C.id`：该检查点摘要。
- `PQ.sequence_number`：证据对应的序列号。
- `PQ.batch_digest`：请求批摘要。
- `PQ.view`：该证据对应的旧视图。

**代码依据**：

- `consensus/pbft/messages.proto:91`
- `consensus/pbft/viewchange.go:125/186`
- `consensus/pbft/sign.go`

#### 4.6.3 `NewView`

**消息名称**：`NewView`  
**代码定义位置**：`consensus/pbft/messages.proto` `message new_view`  
**发送位置**：

- `consensus/pbft/viewchange.go:257` `sendNewView`

**接收/处理位置**：

- `consensus/pbft/viewchange.go:293` `recvNewView`
- `consensus/pbft/viewchange.go:314` `processNewView`

**外层封装关系**：`pbft.Message.new_view`  

**结构体 / proto 原文**：

```proto
message new_view {
    uint64 view = 1;
    repeated view_change vset = 2;
    map<uint64, string> xset = 3;
    uint64 replica_id = 4;
}
```

**字段逐项说明**：

- `view`：新视图号。
- `vset`：组成新视图证明的 `ViewChange` 集合；接收端会验证其中每个 `ViewChange` 的签名。
- `xset`：新视图下序列号到请求批摘要的映射。
- `replica_id`：发送 `NewView` 的新主节点编号。

**特别说明**：

- `NewView` 自身没有 `signature` 字段；
- 当前实现通过验证 `vset` 中的 `ViewChange.signature` 来间接验证新视图基础证据。

**代码依据**：

- `consensus/pbft/messages.proto:116`
- `consensus/pbft/viewchange.go:257/293/314`

#### 4.6.4 `FetchRequestBatch`

**消息名称**：`FetchRequestBatch`  
**代码定义位置**：`consensus/pbft/messages.proto` `message fetch_request_batch`  
**发送位置**：

- `consensus/pbft/pbft-core.go:1237` `fetchRequestBatches`

**接收/处理位置**：

- `consensus/pbft/pbft-core.go:1250` `recvFetchRequestBatch`

**外层封装关系**：`pbft.Message.fetch_request_batch`  

**结构体 / proto 原文**：

```proto
message fetch_request_batch {
    string batch_digest = 1;
    uint64 replica_id = 2;
}
```

**字段逐项说明**：

- `batch_digest`：缺失请求批的摘要。
- `replica_id`：请求补批的副本编号，响应方据此单播返回。

**代码依据**：

- `consensus/pbft/messages.proto:123`
- `consensus/pbft/pbft-core.go:1237/1250`

#### 4.6.5 `ReturnRequestBatch`

**消息名称**：`return_request_batch`（不是独立结构体，而是 `pbft.Message` 的 oneof 变体）  
**代码定义位置**：

- `consensus/pbft/messages.proto` 顶层 `message` 的 `return_request_batch = 9`
- 复用的实体结构是 `RequestBatch`

**发送位置**：

- `consensus/pbft/pbft-core.go:1257` `recvFetchRequestBatch`

**接收/处理位置**：

- `consensus/pbft/pbft-core.go:1269` `recvReturnRequestBatch`

**外层封装关系**：

```text
pbft.Message.return_request_batch = RequestBatch
```

**结构体 / proto 原文**：

```proto
message message {
    oneof payload {
        request_batch return_request_batch = 9;
    }
}
```

**字段逐项说明**：

- 本消息没有单独字段定义；
- 实际承载内容就是 `RequestBatch.batch`；
- 因此不能把它写成一个并不存在的独立 `ReturnRequestBatch` 结构体。

**代码依据**：

- `consensus/pbft/messages.proto:47`
- `consensus/pbft/pbft-core.go:1257/1269`

### 4.7 第六组：执行状态、同步与预留补充消息

这一组不属于 PBFT 主流程本身，但如果忽略它们，就无法完整解释“系统如何从账本中恢复位置”“节点落后时如何追赶”“proto 中哪些分支只是预留”。因此本组应作为汇报中的补充说明，而不是主干消息链路。

#### 4.7.1 `Metadata`

**消息名称**：`Metadata`  
**代码定义位置**：`consensus/pbft/messages.proto` `message metadata`  
**发送位置**：

- `consensus/pbft/batch.go:218` 在执行前序列化

**接收/处理位置**：

- `consensus/pbft/pbft.go:151` `getLastSeqNo` 读取区块头共识元数据时反序列化

**外层封装关系**：不经 PBFT 网络广播，作为共识元数据写入账本头部  

**结构体 / proto 原文**：

```proto
message metadata {
    uint64 seqNo = 1;
}
```

**字段逐项说明**：

- `seqNo`：当前执行/提交的 PBFT 序列号；用于恢复最近执行位置。

**代码依据**：

- `consensus/pbft/messages.proto:145`
- `consensus/pbft/batch.go:218`
- `consensus/pbft/pbft.go:151`

#### 4.7.2 状态同步消息（补充）

这些消息不是 `consensus/pbft/messages.proto` 中的 PBFT 协议报文，但与 PBFT 的状态恢复实际相关。

**相关消息定义位置**：`protos/fabric.proto`

- `SyncBlockRange`
- `SyncBlocks`
- `SyncStateSnapshotRequest`
- `SyncStateSnapshot`
- `SyncStateDeltasRequest`
- `SyncStateDeltas`

**发送位置**：`core/peer/handler.go`

- `321` `RequestBlocks`
- `334` 发送 `SYNC_GET_BLOCKS`
- `429` 发送 `SYNC_BLOCKS`
- `458` 发送 `SYNC_STATE_GET_SNAPSHOT`
- `561/574` 发送 `SYNC_STATE_SNAPSHOT`
- `604` 发送 `SYNC_STATE_GET_DELTAS`
- `667` 发送 `SYNC_STATE_DELTAS`

**与 PBFT 的关系**：

- `consensus/pbft/pbft.go:136` `op.stack.UpdateState(...)`
- `consensus/pbft/pbft-core.go:907` `instance.consumer.skipTo(...)`

**结论**：

- 状态同步消息应列入“补充消息”章节；
- 但需要明确它们属于 Peer 层状态同步协议，而不是 `pbft.Message` 中的核心协议结构。

#### 4.7.3 `Complaint`

**消息名称**：`complaint`  
**代码定义位置**：`consensus/pbft/messages.proto` `batch_message` 的 `complaint = 4`  
**发送位置**：当前生产代码未定位到发送逻辑  
**接收/处理位置**：当前生产代码未定位到接收逻辑  
**外层封装关系**：理论上属于 `BatchMessage.complaint`  

**结构体 / proto 原文**：

```proto
message batch_message {
    oneof payload {
        request complaint = 4;
    }
}
```

**字段逐项说明**：

- `complaint`：复用 `Request` 结构；proto 注释写明“like request, but processed everywhere”；
- 但当前生产代码未定位到构造、发送、反序列化分支或处理逻辑；
- 因此在正式说明中应标注为“定义存在，但当前实现中未定位到直接证据支持其被实际使用”。

**代码依据**：

- `consensus/pbft/messages.proto:139`
- 生产代码未定位到 `GetComplaint()`、`BatchMessage_Complaint{}` 的实际调用

## 五、消息流转路径

本章用于把前面拆散的结构体重新串回一条完整流程，回答“从业务请求进入系统，到最终执行/返回，中间究竟发生了什么”。若用于汇报，本章可直接作为讲解顺序：先按函数调用链说明流程，再结合第四章回看每一步对应的结构体。

### 5.1 收到业务请求到进入 PBFT

1. 客户端调用 `ProcessTransaction(Transaction)`  
   代码依据：`protos/fabric.proto:126`，`core/peer/peer.go:302`
2. Validator 执行 `ExecuteTransaction`  
   代码依据：`core/peer/peer.go:643`
3. 本地 Validator 将交易封装为 `protos.Message(type=CHAIN_TRANSACTION)`  
   代码依据：`core/peer/peer.go:520`
4. `EngineImpl.ProcessTransactionMsg` 将其送入共识模块  
   代码依据：`consensus/helper/engine.go:47`
5. `externalEventReceiver.RecvMsg` 将外部消息投递给 PBFT 事件队列  
   代码依据：`consensus/pbft/external.go:58`
6. `obcBatch.processMessage` 识别到 `CHAIN_TRANSACTION`，调用 `txToReq` 生成 `Request`  
   代码依据：`consensus/pbft/batch.go:363/349`
7. `submitToLeader` 广播 `BatchMessage.request`  
   代码依据：`consensus/pbft/batch.go:140/156`

### 5.2 从 Request 到 RequestBatch

1. 各副本收到 `BatchMessage.request`
2. 非主节点仅存入 `reqStore` 并启动超时相关逻辑
3. 主节点调用 `leaderProcReq` 把多个 `Request` 放入 `batchStore`
4. 满足批大小或超时后，`sendBatch` 生成 `RequestBatch`

代码依据：

- `consensus/pbft/batch.go:381`
- `consensus/pbft/batch.go:288` `leaderProcReq`
- `consensus/pbft/batch.go:336` `sendBatch`

### 5.3 从 RequestBatch 到 PrePrepare / Prepare / Commit

1. `pbftCore.recvRequestBatch` 接收请求批
2. 若当前副本是主节点，则 `sendPrePrepare`
3. 备节点在 `recvPrePrepare` 中校验并发送 `Prepare`
4. 达到 prepared 条件后在 `maybeSendCommit` 中发送 `Commit`
5. `recvCommit` 达到 committed 条件后进入执行与提交流程

代码依据：

- `consensus/pbft/pbft-core.go:611`
- `consensus/pbft/pbft-core.go:630`
- `consensus/pbft/pbft-core.go:701`
- `consensus/pbft/pbft-core.go:775`
- `consensus/pbft/pbft-core.go:809`
- `consensus/pbft/pbft-core.go:827`

### 5.4 执行、提交与返回路径

1. `pbftCore` 判定可执行后调用 `consumer.execute`
2. `obcBatch.execute` 将 `RequestBatch` 中的 `Request.payload` 反序列化为 `Transaction`
3. 构造 `Metadata{seqNo}` 并调用 `stack.Execute(meta, txs)`
4. 执行完成后通过 `Executed / Commit / Committed` 回调推进后续状态
5. 该路径未见重新向客户端构造 `PBFT reply` 报文

代码依据：

- `consensus/pbft/pbft-core.go:961`
- `consensus/pbft/batch.go:203`
- `consensus/pbft/batch.go:218`
- `consensus/helper/helper.go:338/357/364`

### 5.5 视图切换与补批

1. 副本发送 `ViewChange`
2. 新主节点收齐后发送 `NewView`
3. 若缺失非检查点请求批，则发送 `FetchRequestBatch`
4. 其他副本单播 `ReturnRequestBatch`

代码依据：

- `consensus/pbft/viewchange.go:125`
- `consensus/pbft/viewchange.go:257`
- `consensus/pbft/pbft-core.go:1237`
- `consensus/pbft/pbft-core.go:1250`
- `consensus/pbft/pbft-core.go:1269`

## 六、代码与论文示意图的异同

本章集中说明“论文示意图如何表达”与“工程实现如何落地”之间的差异。这一部分是汇报中的重点，因为它直接解释了为什么当前项目里没有独立 `reply`、为什么 `request` 不是简单单播主节点、以及为什么工程实现会出现多层封装。

### 6.1 一致之处

- 仍然保留了 `PrePrepare -> Prepare -> Commit` 三阶段；
- 仍然有 `Checkpoint`、`ViewChange`、`NewView`；
- 仍然以视图号、序列号、批摘要作为核心协议字段。

### 6.2 差异之处

#### 差异一：`request` 不是“客户端直接单播主节点”

论文示意图常画成客户端把 request 发给主节点。  
当前工程中，真实实现是：

- 客户端先提交 `Transaction`
- Validator 本地转成 `Request`
- 再广播 `BatchMessage.request` 给全体副本

因此工程实现多了一层**业务交易入口**和一层**广播到全体副本**的处理。

#### 差异二：多了一层工程封装

论文示意图通常只讨论协议消息。  
当前工程多出了：

- `protos.Message` 外层网络信封
- `BatchMessage` 中间批处理封装

所以图上的一个箭头，在代码里往往不是一个结构体，而是多层封装后的消息对象。

#### 差异三：`reply` 不是独立 PBFT 协议消息

论文示意图里通常画有 `reply`。  
当前工程里没有 `pbft.reply`，真实返回路径是 `pb.Response`，而且：

- 交易请求的返回时机早于共识完成；
- 查询请求不进入 PBFT 共识主流程。

因此，文档中必须明确写为“工程实现差异”。

#### 差异四：部分 proto 变体并未走到生产路径

当前 proto 中定义了：

- `pbft.Message.request_batch`
- `BatchMessage.request_batch`
- `BatchMessage.complaint`

但当前生产代码未定位到这些变体的直接发送路径。  
因此它们不能被写成“当前系统中正在参与正常消息流”的消息。

## 七、结论

本章用于把全文压缩成一个可直接复述的收束版本，适合在汇报前最后快速通读。如需在较短时间内概括全文，直接围绕本章的几条总结展开即可。

当前工程中的 PBFT 报文体系，可以归纳为以下几点：

1. **最外层**是 Fabric 工程统一网络消息 `protos.Message`。
2. **交易入口**首先提交的是 `Transaction`，不是 PBFT 协议消息。
3. **PBFT 请求入口**是 `Request`，但它通过 `BatchMessage.request` 被广播给全体副本，而不是简单单播主节点。
4. **正式共识阶段**真实使用的是：
   - `PrePrepare`
   - `Prepare`
   - `Commit`
   - `Checkpoint`
   - `ViewChange`
   - `NewView`
   - `FetchRequestBatch`
   - `ReturnRequestBatch`
5. **reply 在当前工程中不是独立 PBFT 报文**，而是 `pb.Response` 形式的工程返回路径。
6. **字段使用必须区分“真实使用”和“proto 预留”**：
   - `ViewChange.signature` 有真实签名逻辑；
   - `Request.signature` 当前基本未用；
   - `Complaint` 与若干 `request_batch` oneof 变体当前未见生产发送路径。

综上，若要准确说明当前项目中的 PBFT 报文结构，必须采用“**工程实现视角**”而非“**论文抽象视角**”：  
真正需要讲清楚的不是几个名字相似的消息，而是**交易、网络信封、批处理封装、协议消息、返回路径**这五层之间的真实对应关系。
