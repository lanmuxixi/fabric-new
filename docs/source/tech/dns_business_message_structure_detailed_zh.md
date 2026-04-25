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

它不属于老师当前最关心的 6 类核心业务消息，因此放在附录中说明。

### 8.6 如果继续往下看，真正发给 DNS 服务器的 `send` 数据是什么

如果老师继续追问“链码最终调用的 send 函数里，数据结构长什么样”，那么比起旧的 `AddDomain / DeleteDomain / ResolveDomain`，更应该看当前真实在主线里被调用的这 3 个函数：

- `myutils.SendQuest`
- `myutils.SendUpdate`
- `myutils.SendRemoveName`

它们对应的是链码在完成业务参数解析后，真正向 Bind9 发起请求时使用的下一层数据结构。

#### 8.6.1 `SendQuest`

真实函数签名：

```go
func SendQuest(domain string, dnsServer string, dnsPort string) ([]dns.A, error)
```

代码依据：

- 函数定义：`examples/chaincode/go/chaincode_dns_reslover/myutils/netUtils.go:15`
- 构造查询问题：`examples/chaincode/go/chaincode_dns_reslover/myutils/netUtils.go:18`

等价 C 风格结构体：

```c
typedef struct {
    char domain[256];      // 要查询的完整域名，例如 "www.example.com"
    char dns_server[64];   // 目标权威 DNS 服务器 IP，例如 "10.92.2.140"
    char dns_port[16];     // 目标权威 DNS 服务器端口，例如 "53"
    char qtype[16];        // 查询类型，当前固定为 A
} SendQuestMsg;
```

等价 JSON：

```json
{
  "domain": "www.example.com",
  "dns_server": "10.92.2.140",
  "dns_port": "53",
  "qtype": "A"
}
```

字段说明：

- `domain`：要查询的完整域名
- `dns_server`：实际接收请求的权威 DNS 服务器 IP
- `dns_port`：实际接收请求的权威 DNS 服务器端口
- `qtype`：当前代码固定查询 A 记录

#### 8.6.2 `SendUpdate`

真实函数签名：

```go
func SendUpdate(zone string, domain string, ip string, dnsServer string, dnsPort string) (bool, error)
```

代码依据：

- 函数定义：`examples/chaincode/go/chaincode_dns_reslover/myutils/netUtils.go:41`
- 设置更新 zone：`examples/chaincode/go/chaincode_dns_reslover/myutils/netUtils.go:44`
- 插入 RR：`examples/chaincode/go/chaincode_dns_reslover/myutils/netUtils.go:49`

等价 C 风格结构体：

```c
typedef struct {
    char zone[64];         // 顶级域 zone，例如 "com"
    char domain[256];      // 完整域名，例如 "www.example.com"
    char record_ip[64];    // 要写入的 A 记录 IP，例如 "10.92.2.138"
    char dns_server[64];   // 目标权威 DNS 服务器 IP
    char dns_port[16];     // 目标权威 DNS 服务器端口
} SendUpdateMsg;
```

等价 JSON：

```json
{
  "zone": "com",
  "domain": "www.example.com",
  "record_ip": "10.92.2.138",
  "dns_server": "10.92.2.140",
  "dns_port": "53"
}
```

字段说明：

- `zone`：DNS 更新所在的 zone，来自顶级域提取结果
- `domain`：要新增或修改的完整域名
- `record_ip`：要写入的 A 记录值
- `dns_server`：实际接收更新请求的权威 DNS 服务器 IP
- `dns_port`：实际接收更新请求的权威 DNS 服务器端口

#### 8.6.3 `SendRemoveName`

真实函数签名：

```go
func SendRemoveName(zone string, domain string, dnsServer string, dnsPort string) (bool, error)
```

代码依据：

- 函数定义：`examples/chaincode/go/chaincode_dns_reslover/myutils/netUtils.go:83`
- 设置更新 zone：`examples/chaincode/go/chaincode_dns_reslover/myutils/netUtils.go:86`
- 删除名称：`examples/chaincode/go/chaincode_dns_reslover/myutils/netUtils.go:91`

等价 C 风格结构体：

```c
typedef struct {
    char zone[64];         // 顶级域 zone，例如 "com"
    char domain[256];      // 要删除的完整域名，例如 "www.example.com"
    char dns_server[64];   // 目标权威 DNS 服务器 IP
    char dns_port[16];     // 目标权威 DNS 服务器端口
} SendRemoveNameMsg;
```

等价 JSON：

```json
{
  "zone": "com",
  "domain": "www.example.com",
  "dns_server": "10.92.2.140",
  "dns_port": "53"
}
```

字段说明：

- `zone`：删除操作所在的 zone
- `domain`：要删除的完整域名
- `dns_server`：实际接收删除请求的权威 DNS 服务器 IP
- `dns_port`：实际接收删除请求的权威 DNS 服务器端口

#### 8.6.4 这 3 个 `send` 函数和正文 6 类消息的关系

正文中的 6 类消息描述的是“链码入口收到的业务数据结构”，即：

```json
{
  "Function": "...",
  "Args": [...]
}
```

本节中的 3 个 `send` 函数描述的是“链码把业务参数解析完成后，真正发给 Bind9 的下一层数据结构”。

如果老师问的是：

- “业务请求进链码时长什么样”

就看正文 6 类消息。

如果老师问的是：

- “send 最终发给 DNS 服务器的数据长什么样”

就看本节这 3 个 `Send...` 结构。
