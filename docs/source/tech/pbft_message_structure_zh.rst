PBFT 共识消息结构说明
======================

本文档用于说明当前项目中两类关键消息的结构：

1. 节点之间进行 PBFT 共识时交换的报文结构
2. 某个节点收到业务请求后，将请求送入 PBFT 流程时的消息结构

本文基于当前仓库中的 Fabric 0.6 代码实现，重点关注以下几个文件：

- ``protos/fabric.proto``
- ``consensus/pbft/messages.proto``
- ``consensus/pbft/batch.go``
- ``consensus/pbft/pbft-core.go``
- ``core/peer/peer.go``
- ``consensus/helper/engine.go``


总体结论
--------

当前实现中的消息是分层封装的，不能只看单一结构体。

- 最外层统一使用 Fabric 网络消息 ``protos.Message``
- PBFT 模块为了区分“普通请求”和“正式共识消息”，又增加了一层 ``BatchMessage``
- 真正的 PBFT 三阶段消息 ``PrePrepare``、``Prepare``、``Commit`` 位于最内层 ``pbft.Message`` 中

因此，项目中的 PBFT 消息不是单层结构，而是一个三层封装模型。


第一层：Fabric 网络统一信封
---------------------------

所有节点间传输的消息，最外层都统一使用 ``protos.Message``，定义在 ``protos/fabric.proto`` 中。

其核心结构如下：

.. code-block:: proto

   message Message {
       enum Type {
           UNDEFINED = 0;
           CHAIN_TRANSACTION = 6;
           CONSENSUS = 21;
       }
       Type type = 1;
       google.protobuf.Timestamp timestamp = 2;
       bytes payload = 3;
       bytes signature = 4;
   }

字段含义如下：

- ``type``: 指明消息类别
- ``timestamp``: Fabric 层记录的发送时间
- ``payload``: 具体业务负载，里面可能是交易、BatchMessage 或其他同步消息
- ``signature``: 外层签名字段

在本项目里，与 PBFT 最相关的两个枚举值是：

- ``CHAIN_TRANSACTION``: 表示一笔新的链上交易请求
- ``CONSENSUS``: 表示一条共识层消息


第二层：PBFT 批处理封装 BatchMessage
-----------------------------------

PBFT 模块没有直接把 ``PrePrepare``、``Prepare``、``Commit`` 塞进外层 ``protos.Message``，而是先增加了一层 ``BatchMessage``，定义在 ``consensus/pbft/messages.proto`` 中。

其核心结构如下：

.. code-block:: proto

   message batch_message {
       oneof payload {
           request request = 1;
           request_batch request_batch = 2;
           bytes pbft_message = 3;
           request complaint = 4;
       }
   }

这里的设计意图是：

- ``request``: 单条请求刚进入 PBFT 网络时使用
- ``request_batch``: 主节点聚合后形成的请求批
- ``pbft_message``: 正式共识阶段的 PBFT 协议消息，内部再反序列化为 ``pbft.Message``
- ``complaint``: 类似特殊请求，当前不是主流程重点

因此，节点之间看到的 ``CONSENSUS`` 消息，通常需要先反序列化为 ``BatchMessage``，然后再根据 ``payload`` 判断当前所处阶段。


第三层：PBFT 协议内部消息
-------------------------

PBFT 自己的协议消息定义在 ``consensus/pbft/messages.proto`` 中的 ``message`` 结构：

.. code-block:: proto

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

在实验和汇报中，最核心的是三阶段共识相关的三种消息。


1. PrePrepare 消息
~~~~~~~~~~~~~~~~~~

.. code-block:: proto

   message pre_prepare {
       uint64 view = 1;
       uint64 sequence_number = 2;
       string batch_digest = 3;
       request_batch request_batch = 4;
       uint64 replica_id = 5;
   }

字段含义：

- ``view``: 当前视图号
- ``sequence_number``: 当前请求批在该视图中的序列号
- ``batch_digest``: 请求批摘要
- ``request_batch``: 本次共识对应的请求批内容
- ``replica_id``: 发送该消息的副本编号，通常应当是主节点

该消息由主节点发出，代码位置在 ``consensus/pbft/pbft-core.go`` 的 ``recvRequestBatch`` 路径中。


2. Prepare 消息
~~~~~~~~~~~~~~~

.. code-block:: proto

   message prepare {
       uint64 view = 1;
       uint64 sequence_number = 2;
       string batch_digest = 3;
       uint64 replica_id = 4;
   }

字段含义：

- ``view``: 当前视图号
- ``sequence_number``: 当前序列号
- ``batch_digest``: 对应请求批的摘要
- ``replica_id``: 发送该 Prepare 的副本编号

该消息由备节点在收到合法 ``PrePrepare`` 后发送。


3. Commit 消息
~~~~~~~~~~~~~~

.. code-block:: proto

   message commit {
       uint64 view = 1;
       uint64 sequence_number = 2;
       string batch_digest = 3;
       uint64 replica_id = 4;
   }

字段含义与 ``Prepare`` 基本一致，但语义是“该副本已经确认该请求批可提交”。

当某副本达到 prepared 条件后，会广播 ``Commit``。


节点间共识报文的完整封装关系
----------------------------

节点间正式共识时，消息整体封装关系如下：

.. code-block:: text

   protos.Message
     type = CONSENSUS
     payload = marshal(BatchMessage)
       BatchMessage.payload = pbft_message
         pbft_message = marshal(pbft.Message)
           pbft.Message.payload = PrePrepare / Prepare / Commit / ...

也就是说：

- 网络层看到的是 ``protos.Message``
- PBFT 批处理层先拆成 ``BatchMessage``
- 协议层再拆成 ``pbft.Message``
- 最后才得到具体的 ``PrePrepare``、``Prepare``、``Commit`` 等消息


业务请求进入 PBFT 前的原始结构
------------------------------

某个节点收到客户端请求后，不会立刻生成 ``PrePrepare``。首先生成的是一条交易消息 ``Transaction``，定义在 ``protos/fabric.proto`` 中：

.. code-block:: proto

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

其中常见重要字段包括：

- ``type``: 交易类型，例如部署、调用、查询
- ``chaincodeID``: 目标链码标识
- ``payload``: 链码调用参数等业务数据
- ``txid``: 交易 ID
- ``timestamp``: 交易时间戳
- ``signature``: 交易签名

随后该交易会被包装成一条外层 Fabric 消息：

.. code-block:: text

   protos.Message
     type = CHAIN_TRANSACTION
     payload = marshal(Transaction)

对应代码位于 ``core/peer/peer.go`` 的 ``sendTransactionsToLocalEngine``。


节点发出请求后的 PBFT Request 结构
---------------------------------

PBFT 模块收到 ``CHAIN_TRANSACTION`` 之后，会在 ``consensus/pbft/batch.go`` 中把交易转换成 PBFT 自己的 ``Request``：

.. code-block:: proto

   message request {
       google.protobuf.Timestamp timestamp = 1;
       bytes payload = 2;
       uint64 replica_id = 3;
       bytes signature = 4;
   }

字段含义如下：

- ``timestamp``: 请求进入 PBFT 时生成的时间戳
- ``payload``: 原始交易字节，也就是 ``marshal(Transaction)`` 的结果
- ``replica_id``: 当前发起该请求的副本编号
- ``signature``: 请求签名字段，当前实现中预留但未完整使用

换句话说，PBFT 的 ``Request`` 并不直接存业务字段，而是把整个 ``Transaction`` 当成一个字节数组放进 ``payload``。


节点发出请求后的完整封装路径
----------------------------

从某个节点收到请求，到 PBFT 正式开始共识，路径如下。


阶段一：交易进入本地共识引擎
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

节点把交易先封成：

.. code-block:: text

   protos.Message
     type = CHAIN_TRANSACTION
     payload = marshal(Transaction)


阶段二：PBFT 将交易转成 Request
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

PBFT 收到 ``CHAIN_TRANSACTION`` 后，调用 ``txToReq`` 生成：

.. code-block:: text

   Request
     timestamp = 当前时间
     payload = marshal(Transaction)
     replica_id = 当前副本 ID


阶段三：Request 通过 BatchMessage 广播
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

随后该请求被广播为：

.. code-block:: text

   protos.Message
     type = CONSENSUS
     payload = marshal(BatchMessage)
       BatchMessage.payload = request
         request.payload = marshal(Transaction)

这是“请求进入 PBFT 网络”的结构，还不是正式三阶段共识消息。


阶段四：主节点形成 RequestBatch
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

主节点收集若干 ``Request`` 之后，打包为：

.. code-block:: proto

   message request_batch {
       repeated request batch = 1;
   }

随后主节点基于该批次生成 ``PrePrepare``。


阶段五：PBFT 三阶段共识
~~~~~~~~~~~~~~~~~~~~~~~

之后进入标准流程：

1. 主节点广播 ``PrePrepare``
2. 备节点验证后广播 ``Prepare``
3. 副本达到 prepared 条件后广播 ``Commit``
4. 满足提交条件后执行请求批并写账本

在该阶段，消息结构为：

.. code-block:: text

   protos.Message
     type = CONSENSUS
     payload = marshal(BatchMessage)
       BatchMessage.payload = pbft_message
         pbft_message = marshal(pbft.Message)
           pbft.Message.payload = PrePrepare / Prepare / Commit


代码路径总结
------------

下面是最关键的代码入口，便于阅读源码或向导师说明。

- ``core/peer/peer.go``
  - ``sendTransactionsToLocalEngine``: 将 ``Transaction`` 封装为 ``CHAIN_TRANSACTION``
- ``consensus/helper/engine.go``
  - ``ProcessTransactionMsg``: 把交易消息送入共识模块
- ``consensus/pbft/external.go``
  - ``RecvMsg``: 将外部消息写入 PBFT 事件队列
- ``consensus/pbft/batch.go``
  - ``processMessage``: 区分 ``CHAIN_TRANSACTION`` 和 ``CONSENSUS``
  - ``txToReq``: 将交易转换为 ``Request``
  - ``submitToLeader``: 广播 ``BatchMessage.request``
  - ``wrapMessage``: 将 ``pbft.Message`` 包装为 ``BatchMessage.pbft_message``
- ``consensus/pbft/pbft-core.go``
  - ``recvRequestBatch``: 主节点为请求批生成 ``PrePrepare``
  - ``recvPrePrepare``: 备节点处理 ``PrePrepare`` 并发送 ``Prepare``
  - ``recvPrepare`` / ``maybeSendCommit``: 达到条件后发送 ``Commit``


汇报时的简化讲法
----------------

如果需要用尽量短的方式向导师汇报，可以直接这样讲：

“当前代码中的 PBFT 消息分三层。最外层是 Fabric 的统一网络消息 ``protos.Message``。中间层是 PBFT 自己的 ``BatchMessage``，用于区分普通请求和正式共识消息。最内层才是 ``PrePrepare``、``Prepare``、``Commit`` 等 PBFT 协议消息。某个节点发出业务请求时，最开始发送的是 ``CHAIN_TRANSACTION``，里面装的是 ``Transaction``。进入 PBFT 后，这笔交易会被转换成 ``Request``，广播到各副本；主节点再把多个 Request 打包成 ``RequestBatch``，之后才进入 ``PrePrepare``、``Prepare``、``Commit`` 三阶段共识。” 


一句话总结
----------

本项目中的“节点请求消息”和“节点间共识消息”不是同一个结构：

- 请求入口是 ``Message(type=CHAIN_TRANSACTION) -> Transaction``
- 进入 PBFT 后变成 ``Message(type=CONSENSUS) -> BatchMessage.request -> Request``
- 正式共识阶段变成 ``Message(type=CONSENSUS) -> BatchMessage.pbft_message -> pbft.Message -> PrePrepare/Prepare/Commit``
