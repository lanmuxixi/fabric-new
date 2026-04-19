# 当前项目 DNS 业务报文结构说明（老师版）

## 一、send 函数里的业务数据结构

当前代码中**没有单独定义** `struct DNSMessage`。  
真实发送格式就是：

```json
{
  "Function": "<业务动作名>",
  "Args": ["参数1", "参数2", "..."]
}
```

也就是：

- `Function` 本身就是操作类型
- `Args` 是顺序参数数组

老师当前最关心的 6 类核心 DNS 业务消息如下：

| 业务动作 | Function | Args 真实顺序 |
| --- | --- | --- |
| 普通域名注册/修改 | `update` | `["完整域名", "A记录IP"]` |
| 普通域名删除 | `delete` | `["完整域名"]` |
| 普通域名查询 | `resolve` | `["完整域名"]` |
| 顶级域注册/修改 | `TopLevelUpdate` | `["用于提取顶级域的域名", "权威DNS地址"]` |
| 顶级域删除 | `TopLevelDelete` | `["用于提取顶级域的域名"]` |
| 顶级域查询 | `TopLevelQuest` | `["用于提取顶级域的域名"]` |

## 二、普通域名注册/修改消息（`update`）

### 2.1 真实代码表示

```text
Function = "update"
Args[0] = domain
Args[1] = server
```

含义：

- `Args[0]`：完整域名，例如 `www.example.com`
- `Args[1]`：该域名要写入的 A 记录 IP，例如 `10.92.2.138`

### 2.2 等价 C 风格结构体

以下结构体是根据真实参数提炼出的**等价表达**，不是代码原生 struct：

```c
typedef struct {
    char function[32];     // 固定为 "update"
    char domain[256];      // 完整域名，例如 "www.example.com"
    char record_ip[64];    // 目标 A 记录值，例如 "10.92.2.138"
} UpdateMsg;
```

### 2.3 等价 JSON payload

```json
{
  "Function": "update",
  "Args": ["www.example.com", "10.92.2.138"]
}
```

### 2.4 字段注释

- `function`：操作类型，表示“普通域名注册/修改”
- `domain`：要新增或修改的完整域名
- `record_ip`：该域名对应的 A 记录值

### 2.5 一个具体例子

如果要把 `www.example.com` 指向 `10.92.2.138`，send 里的业务数据可写成：

```json
{
  "Function": "update",
  "Args": ["www.example.com", "10.92.2.138"]
}
```

## 三、普通域名删除消息（`delete`）

### 3.1 真实代码表示

```text
Function = "delete"
Args[0] = domain
```

含义：

- `Args[0]`：要删除的完整域名，例如 `www.example.com`

### 3.2 等价 C 风格结构体

以下结构体是根据真实参数提炼出的**等价表达**，不是代码原生 struct：

```c
typedef struct {
    char function[32];     // 固定为 "delete"
    char domain[256];      // 要删除的完整域名，例如 "www.example.com"
} DeleteMsg;
```

### 3.3 等价 JSON payload

```json
{
  "Function": "delete",
  "Args": ["www.example.com"]
}
```

### 3.4 字段注释

- `function`：操作类型，表示“普通域名删除”
- `domain`：要删除的完整域名

### 3.5 一个具体例子

如果要删除 `www.example.com`，send 里的业务数据可写成：

```json
{
  "Function": "delete",
  "Args": ["www.example.com"]
}
```

## 四、普通域名查询消息（`resolve`）

### 4.1 真实代码表示

```text
Function = "resolve"
Args[0] = domain
```

含义：

- `Args[0]`：要查询的完整域名，例如 `www.example.com`

### 4.2 等价 C 风格结构体

以下结构体是根据真实参数提炼出的**等价表达**，不是代码原生 struct：

```c
typedef struct {
    char function[32];     // 固定为 "resolve"
    char domain[256];      // 要查询的完整域名，例如 "www.example.com"
} ResolveMsg;
```

### 4.3 等价 JSON payload

```json
{
  "Function": "resolve",
  "Args": ["www.example.com"]
}
```

### 4.4 字段注释

- `function`：操作类型，表示“普通域名查询”
- `domain`：要查询的完整域名

### 4.5 一个具体例子

如果要查询 `www.example.com`，send 里的业务数据可写成：

```json
{
  "Function": "resolve",
  "Args": ["www.example.com"]
}
```

## 五、顶级域注册/修改消息（`TopLevelUpdate`）

### 5.1 真实代码表示

```text
Function = "TopLevelUpdate"
Args[0] = domain
Args[1] = authorityServer
```

这里要特别说明：

- `Args[0]` 不是直接传裸顶级域 `com`
- 当前代码传入的是一个“可提取出顶级域的域名字符串”，例如 `example.org`
- 链码内部再从 `example.org` 提取出 `org`
- `Args[1]` 是该顶级域对应的权威 DNS 地址，例如 `2.2.2.2:53`

### 5.2 等价 C 风格结构体

以下结构体是根据真实参数提炼出的**等价表达**，不是代码原生 struct：

```c
typedef struct {
    char function[32];           // 固定为 "TopLevelUpdate"
    char request_domain[256];    // 输入域名，例如 "example.org"，代码会从中提取 "org"
    char authority_server[128];  // 权威 DNS 地址，例如 "2.2.2.2:53"
} TopLevelUpdateMsg;
```

### 5.3 等价 JSON payload

```json
{
  "Function": "TopLevelUpdate",
  "Args": ["example.org", "2.2.2.2:53"]
}
```

### 5.4 字段注释

- `function`：操作类型，表示“顶级域注册/修改”
- `request_domain`：输入域名，链码会从中提取顶级域
- `authority_server`：该顶级域对应的权威 DNS 地址

### 5.5 一个具体例子

如果要把顶级域 `org` 的权威 DNS 改成 `2.2.2.2:53`，当前代码实际 send 的业务数据可写成：

```json
{
  "Function": "TopLevelUpdate",
  "Args": ["example.org", "2.2.2.2:53"]
}
```

## 六、顶级域删除消息（`TopLevelDelete`）

### 6.1 真实代码表示

```text
Function = "TopLevelDelete"
Args[0] = domain
```

这里同样不是直接传裸顶级域，而是传一个可提取顶级域的域名字符串，例如：

```text
Args[0] = "example.org"
```

链码内部再提取出 `org` 并删除其映射。

### 6.2 等价 C 风格结构体

以下结构体是根据真实参数提炼出的**等价表达**，不是代码原生 struct：

```c
typedef struct {
    char function[32];         // 固定为 "TopLevelDelete"
    char request_domain[256];  // 输入域名，例如 "example.org"，代码会从中提取 "org"
} TopLevelDeleteMsg;
```

### 6.3 等价 JSON payload

```json
{
  "Function": "TopLevelDelete",
  "Args": ["example.org"]
}
```

### 6.4 字段注释

- `function`：操作类型，表示“顶级域删除”
- `request_domain`：输入域名，链码会从中提取顶级域

### 6.5 一个具体例子

如果要删除顶级域 `org` 的权威 DNS 映射，当前代码实际 send 的业务数据可写成：

```json
{
  "Function": "TopLevelDelete",
  "Args": ["example.org"]
}
```

## 七、顶级域查询消息（`TopLevelQuest`）

### 7.1 真实代码表示

```text
Function = "TopLevelQuest"
Args[0] = domain
```

这里也不是直接传裸顶级域，而是传一个可提取顶级域的域名字符串，例如：

```text
Args[0] = "google.com"
```

链码内部再提取出 `com` 并查询其权威 DNS 地址。

### 7.2 等价 C 风格结构体

以下结构体是根据真实参数提炼出的**等价表达**，不是代码原生 struct：

```c
typedef struct {
    char function[32];         // 固定为 "TopLevelQuest"
    char request_domain[256];  // 输入域名，例如 "google.com"，代码会从中提取 "com"
} TopLevelQuestMsg;
```

### 7.3 等价 JSON payload

```json
{
  "Function": "TopLevelQuest",
  "Args": ["google.com"]
}
```

### 7.4 字段注释

- `function`：操作类型，表示“顶级域查询”
- `request_domain`：输入域名，链码会从中提取顶级域

### 7.5 一个具体例子

如果要查询顶级域 `com` 当前对应的权威 DNS，当前代码实际 send 的业务数据可写成：

```json
{
  "Function": "TopLevelQuest",
  "Args": ["google.com"]
}
```

## 八、附录：代码依据

### 8.1 六类核心业务消息对应的入口代码

动作常量与策略注册位置：

- `examples/chaincode/go/chaincode_dns_reslover/chaincode_dns_reslover.go:17-24`
- `examples/chaincode/go/chaincode_dns_reslover/chaincode_dns_reslover.go:29-36`

具体参数读取位置：

- `update`
  - `functions/simpleDomainUpdate.go:13-18`
- `delete`
  - `functions/simpleDomainDelete.go:14-18`
- `resolve`
  - `functions/simpleDomainQuest.go:13-16`
- `TopLevelUpdate`
  - `functions/topLevelDomainUpdate.go:12-17`
- `TopLevelDelete`
  - `functions/topLevelDomainDelete.go:12-16`
- `TopLevelQuest`
  - `functions/topLevelDomainResolve.go:11-14`

### 8.2 为什么真实格式是 `Function + Args[]`

这一结论的代码依据如下：

- `protos/transaction.go:117-132`
  JSON 中的 `Function` 和 `Args` 会被合并为 `ChaincodeInput.Args`
- `core/util/utils.go:128-131`
  每个字符串参数被转成 `[]byte`
- `core/chaincode/shim/handler.go:911-917`
  链码执行前再拆成：
  - `function = allargs[0]`
  - `params = allargs[1:]`
- `core/chaincode/shim/interfaces.go:27-38`
  链码最终入口就是：
  - `Init(stub, function string, args []string)`
  - `Invoke(stub, function string, args []string)`
  - `Query(stub, function string, args []string)`

### 8.3 与 send 函数直接相关的下游调用点

普通域名 3 个动作最终会继续调用底层 send 函数：

- 查询：`myutils.SendQuest`
  - `functions/simpleDomainQuest.go:36`
- 注册/修改：`myutils.SendUpdate`
  - `functions/simpleDomainUpdate.go:39`
- 删除：`myutils.SendRemoveName`
  - `functions/simpleDomainDelete.go:39`

顶级域 3 个动作则不直接发给 Bind9，而是直接对账本中的：

```text
顶级域 -> 权威DNS地址
```

映射执行读写。

### 8.4 现有测试和脚本样例

已有测试样例：

- `chaincode_dns_reslover_test.go:116`
  - `TopLevelQuest("google.com")`
- `chaincode_dns_reslover_test.go:130`
  - `TopLevelUpdate("example.org", "2.2.2.2:53")`
- `chaincode_dns_reslover_test.go:138`
  - `TopLevelDelete("example.org")`

已有脚本样例：

- `scripts/swarm/deploy-dns-chaincode.sh:20`
  - `init` 初始化 payload
- `scripts/swarm/query-top-levels.sh:32`
  - `TopLevelGetAll`
- `deploy/swarm/EXPERIMENT_RUNBOOK.md:261`
  - `update("www.example.com", "10.92.2.138")`

### 8.5 额外但非本页主线的动作

当前代码中还存在一个附加查询动作：

- `TopLevelGetAll`

其真实格式是：

```json
{
  "Function": "TopLevelGetAll",
  "Args": []
}
```

但它不属于老师当前最关心的 6 类核心业务消息，因此放在附录中说明。
