# DNS + Fabric 0.6 Open Usage Guide

## 1. Document Scope

This document is a code-driven usage manual for the `fabric0.60_backup` project.
It is intended for external users, maintainers, and integrators who need to:

- understand what this project does;
- choose the correct branch;
- deploy the network;
- call the built-in Fabric interfaces;
- call the DNS business chaincode interfaces.

This guide is based on three sources:

- the repository code itself;
- the local PPT design document in the repository root;
- the local PDF usage manual in the workspace root.

Where the code and the PPT differ, this document treats the code as the source of truth and explicitly marks the discrepancy.

## 2. What This Project Is

This repository is not a plain upstream Hyperledger Fabric repository. It is a customized Fabric 0.6 codebase with three layers:

1. Fabric 0.6 base runtime
   - peer startup, gRPC services, PBFT consensus, REST gateway, chaincode runtime
2. DNS business chaincode layer
   - on-chain top-level-domain registration and lookup
   - user registration and certificate storage
3. Experimental attack/defense layer
   - malicious primary / malicious replica behavior
   - request broadcast and proof-of-work defense

From the code and PPT together, the system goal is:

- use a consortium blockchain to manage top-level domain ownership and authority records;
- let institutions register, query, update, and delete TLD records on-chain;
- in some branches, synchronize or proxy those records to an external BIND9 DNS service;
- in some branches, simulate malicious registration races and evaluate PBFT + POW defenses.

## 3. Repository Layout

The repository can be understood in two halves.

### 3.1 Fabric base

- `peer/`
  - CLI entrypoints such as `peer node start`
- `core/`
  - peer runtime, devops service, REST service, ledger, chaincode support
- `consensus/`
  - PBFT and NOOPS consensus implementations
- `protos/`
  - gRPC protocol definitions
- `membersrvc/`
  - Fabric 0.6 membership service

### 3.2 DNS customization

- `examples/chaincode/go/chaincode_dns_reslover/`
  - main DNS business chaincode
- `examples/chaincode/go/chaincode_domain_dns/`
  - wrapper/proxy chaincode, used in some POW branches
- `txscripts/`
  - batch invoke scripts and transaction generators
- `4-peers.yml`, `10vp_*.yml`, `22vp_*.yml`, `31vp_*.yml`
  - deployment topologies and experiment setups
- `build_image.sh`
  - packaging script for the custom peer image

## 4. Branch Matrix

The project contains multiple branches with different purposes. They should not be treated as interchangeable.

| Branch | Role | Main characteristics | Recommended use |
| --- | --- | --- | --- |
| `yuanshi` | earliest import baseline | initial import branch; minimal custom history | archive / history only |
| `v0.6` | upstream baseline | closest to Fabric 0.6 baseline in this repo | reference baseline |
| `feature/domain-reslover` | DNS business branch | adds DNS resolver chaincode, domain management logic, scripts, partial PBFT-related integration | business function baseline |
| `feature/bind9-support` | DNS + BIND integration branch | builds on DNS business branch and adds BIND-facing simple domain operations and DNS network utilities | external DNS integration |
| `feature/v0.6-multi-host` | multi-host experiment branch | adds multi-host deployment, PBFT tuning, experiment scripts | lab / cluster experiments |
| `v0.6-nopow` | non-POW baseline | PBFT request-path modifications without POW | control group |
| `v0.6-pow` | POW defense baseline | broadcast-based invoke path plus POW validation in chaincode | defense baseline |
| `v0.6-primarybz-nopow` | malicious primary experiment | simulates a Byzantine primary front-running path without POW | attack experiment |
| `v0.6-replica-nopow` | malicious replica experiment | simulates a Byzantine replica front-running path without POW | attack experiment |
| `v0.6-primarybz-pow` | malicious primary + defense | combines malicious primary behavior with request broadcast and POW defense | full experiment branch |

### 4.1 How the branches relate to the PPT

The PPT maps cleanly to the code:

- `feature/domain-reslover`
  - corresponds to the on-chain DNS business implementation
- `feature/bind9-support`
  - corresponds to the external DNS/BIND support path
- `v0.6-primarybz-nopow`
  - corresponds to the malicious primary attack branch
- `v0.6-replica-nopow`
  - corresponds to the malicious replica attack branch
- `v0.6-pow`
  - corresponds to the general POW defense baseline
- `v0.6-primarybz-pow`
  - corresponds to the malicious primary + POW defense branch

### 4.2 Recommended public-facing baseline

If the goal is to expose stable business functions externally, the most suitable branches are:

- `feature/domain-reslover`
  - if you only need on-chain TLD management
- `feature/bind9-support`
  - if you also need DNS server integration

The `v0.6-*-pow` and `v0.6-*-nopow` branches should be described as experiment branches, not as general production branches.

## 5. Runtime Architecture

## 5.1 Peer startup

Each node starts with:

```bash
peer node start
```

The startup entry is `peer/node/start.go`, function `serve()`.

At startup, the node creates and registers:

- `PeerServer`
  - peer-to-peer message processing
- `AdminServer`
  - administrative operations
- `DevopsServer`
  - deploy / invoke / query entrypoint
- `OpenchainServer`
  - blockchain and peer information queries
- REST server
  - only if `rest.enabled=true`
- Event hub server
  - event push channel

If the node is a validating peer, startup also installs the consensus engine through:

- `consensus/helper/engine.go`
- `consensus/controller/controller.go`

## 5.2 Invoke path

The core transaction path is:

1. client calls `Devops.Invoke` or `Devops.Query`
2. `core/devops.go` builds a Fabric transaction
3. `peer.ExecuteTransaction` decides how to route the request
4. validating peer sends the request into the local consensus engine
5. PBFT orders the request
6. ordered transaction invokes chaincode
7. chaincode updates ledger state

The PPT and code both highlight these main points:

- `core/devops.go`
  - `invokeOrQuery`
  - `createExecTx`
- `core/peer/peer.go`
  - `ExecuteTransaction`
  - `sendTransactionsToLocalEngine`
- `consensus/helper/engine.go`
  - `ProcessTransactionMsg`

## 5.3 PBFT customization

This repository modifies PBFT for DNS registration experiments.

The hot files are:

- `consensus/pbft/batch.go`
- `consensus/pbft/requeststore.go`
- `core/peer/peer.go`

Experiment branches add some or all of the following behaviors:

- broadcast invoke transactions to all peers instead of single-casting to one peer;
- track outstanding DNS registration requests by domain name;
- let a Byzantine primary or replica generate competing transactions;
- add POW generation and verification delays;
- trigger view-change earlier when the normal request is delayed.

## 6. Deployment Model

## 6.1 Prerequisites

This project is built around Fabric 0.6-era assumptions:

- Docker / Docker Compose
- Linux-style container environment
- old Fabric membership service model
- PBFT consensus plugin

You should treat it as a legacy system. It is not compatible with modern Fabric 2.x operational patterns.

## 6.2 Main compose files

| File | Purpose |
| --- | --- |
| `peer.yml` | base peer service definition |
| `4-peers.yml` | 4 validating peers + 1 non-validating peer |
| `4-peers-with-membersrvc.yml` | topology with membersrvc enabled |
| `10vp_1nvp.yml` | larger PBFT experiment topology |
| `10vp_2nvp.yml` | larger PBFT experiment topology |
| `22vp_*`, `31vp_3nvp.yml` | attack/defense experiment topologies |

## 6.3 Example startup

Start the default 4-peer topology:

```bash
docker-compose -f 4-peers.yml up -d
```

Important defaults from `peer.yml`:

- consensus plugin: `pbft`
- PBFT mode: `batch`
- default node count: `N=4`
- fault tolerance: `F=1`

## 6.4 Custom DNS-related peer configuration

`peer/core.yaml` contains a custom `dns:` section:

- `chaincodeid`
  - chaincode ID used by the DNS wrapper logic
- `bzname`
  - Byzantine test username
- `bzip`
  - Byzantine test IP
- `bzcert`
  - Byzantine test certificate
- `bzprivatekey`
  - Byzantine test private key
- `updatefunction`
  - update function name, normally `TopLevelUpdate`
- `delayenable`
  - whether delay logic is enabled
- `subnet`
  - subnet filter used in some branches

## 6.5 Image build

The custom image packaging flow is described in `build_image.sh`.

Its behavior is:

- copy customized `consensus/`, `core/`, `peer/`, `protos/`, `txscripts/` and DNS chaincode into a peer-image build context;
- set a default chaincode ID shell variable `zzm`;
- build `peer-image`.

Because the script assumes a specific local filesystem layout such as `/home/qichang/...`, it is better treated as an internal packaging script than a portable build script.

## 7. DNS Business Chaincode

The main business chaincode is:

- `examples/chaincode/go/chaincode_dns_reslover/chaincode_dns_reslover.go`

Its routing model is a simple function table:

- `Init`
- `Invoke`
- `Query`

Mapped function names include:

- `TopLevelUpdate`
- `TopLevelQuest`
- `TopLevelDelete`
- `TopLevelGetAll`
- `UserRegister`
- `resolve`
- `update`
- `delete`

## 7.1 Data model

The main ledger table is `dns_record`.

Columns:

- `name`
  - record name, used as key
- `value`
  - authority server or record value
- `type`
  - `A`, `AAAA`, `NS`
- `owner`
  - owner username
- `ttl`
  - record TTL

The user certificate store uses key-value entries with prefix:

- `USER_<username>`

## 7.2 Chaincode initialization

Typical deploy/init pattern:

```bash
peer chaincode deploy \
  -p github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover \
  -c '{"Function":"init","Args":["com:127.0.0.1:30054","cn:127.0.0.1:30055"]}'
```

Init behavior:

- creates table `dns_record`;
- inserts initial TLD -> authority server mappings;
- preloads at least `ADMIN` and `JIM` users into ledger state.

Each init argument is one string in this format:

- `tld:ip`
- `tld:ip:port`

Examples:

- `com:127.0.0.1:30054`
- `cn:127.0.0.1:30055`

## 8. DNS Chaincode Interface Reference

All DNS chaincode functions return a JSON structure shaped like:

```json
{
  "Code": 0,
  "Msg": "",
  "Data": {}
}
```

Where:

- `Code=0`
  - success
- `Code=1`
  - failure

## 8.1 `UserRegister`

Purpose:

- register a user and store the user certificate on-chain

Call type:

- `Invoke`

Arguments:

| Position | Name | Type | Required | Description |
| --- | --- | --- | --- | --- |
| 1 | `username` | string | yes | user identifier |
| 2 | `certificate` | string | yes | PEM certificate |

Example:

```bash
peer chaincode invoke -n ${zzm} -c '{"Function":"UserRegister","Args":["ADMIN","-----BEGIN CERTIFICATE-----..."]}'
```

Behavior:

- rejects duplicate usernames;
- verifies that the certificate parses as X.509;
- stores the certificate under `USER_<username>`.

## 8.2 `TopLevelQuest`

Purpose:

- resolve a domain to its top-level record from the ledger

Call type:

- `Query`

Arguments:

| Position | Name | Type | Required | Description |
| --- | --- | --- | --- | --- |
| 1 | `domain` | string | yes | domain name such as `google.com` |

Example:

```bash
peer chaincode query -n ${zzm} -c '{"Function":"TopLevelQuest","Args":["google.com"]}'
```

Success payload:

- `Data.record`
  - a `dns_record` entry

## 8.3 `TopLevelGetAll`

Purpose:

- return all TLD records stored in the chaincode table

Call type:

- `Query`

Arguments:

- none

Example:

```bash
peer chaincode query -n ${zzm} -c '{"Function":"TopLevelGetAll","Args":[]}'
```

Success payload:

- `Data.records`
  - array of table records

## 8.4 `TopLevelUpdate`

Purpose:

- register a new TLD authority record on-chain;
- in some branches, also enforce POW validation before accepting the request

Call type:

- `Invoke`

### Base form used in business branches

| Position | Name | Type | Required | Description |
| --- | --- | --- | --- | --- |
| 1 | `domain` | string | yes | domain or TLD to register |
| 2 | `value` | string | yes | authority IP or server address |
| 3 | `recordType` | string | yes | `A`, `AAAA`, `NS` |
| 4 | `ttl` | string/int | yes | TTL value |
| 5 | `owner` | string | yes | owner username |
| 6 | `signature` | string | yes | request signature |

Example:

```bash
peer chaincode invoke -n ${zzm} -c '{"Function":"TopLevelUpdate","Args":["example.com","1.1.1.1","A","86400","ADMIN","<signature>"]}'
```

### POW form used in experiment branches

| Position | Name | Type | Required | Description |
| --- | --- | --- | --- | --- |
| 1 | `domain` | string | yes | domain or TLD to register |
| 2 | `value` | string | yes | authority IP or server address |
| 3 | `recordType` | string | yes | `A`, `AAAA`, `NS` |
| 4 | `ttl` | string/int | yes | TTL value |
| 5 | `owner` | string | yes | owner username |
| 6 | `signature` | string | yes | request signature |
| 7 | `target` | string | yes | POW target |
| 8 | `nonce` | string/int | yes | POW nonce |

Example:

```bash
peer chaincode invoke -n ${zzm} -c '{"Function":"TopLevelUpdate","Args":["example.com","1.1.1.1","A","86400","ADMIN","<signature>","0000ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff","336710"]}'
```

### Validation logic

The code attempts to do the following:

- validate domain format;
- extract the top-level domain;
- validate record type;
- parse TTL;
- check request content and owner identity;
- optionally verify POW;
- insert or replace the ledger record.

### Important code-level caveats

The current workspace contains notable discrepancies:

1. Signature verification is scaffolded but not fully enforced in the checked-out code.
   - `verifyContent()` currently checks user existence, but the certificate-based signature verification path is commented out in the current workspace snapshot.
2. `verifySameUser()` does not currently behave like a true “same owner” check.
   - In the checked-out file it rejects any existing owner instead of allowing the same owner to update.
3. In POW-related branches, `TopLevelUpdate` interface shape changes from 6 args to 8 args.

Public documentation must not overstate the security guarantees unless the checked-out branch is verified and fixed first.

## 8.5 `TopLevelDelete`

Purpose:

- delete a TLD record from the ledger

Call type:

- `Invoke`

Arguments:

| Position | Name | Type | Required | Description |
| --- | --- | --- | --- | --- |
| 1 | `domain` | string | yes | target domain |
| 2 | `owner` | string | yes | owner username |
| 3 | `signature` | string | yes | request signature |

Example:

```bash
peer chaincode invoke -n ${zzm} -c '{"Function":"TopLevelDelete","Args":["example.com","ADMIN","<signature>"]}'
```

## 8.6 `resolve`

Purpose:

- query a real DNS server for a non-TLD domain using the authority server stored on-chain

Call type:

- `Query`

Arguments:

| Position | Name | Type | Required | Description |
| --- | --- | --- | --- | --- |
| 1 | `domain` | string | yes | target full domain |

Example:

```bash
peer chaincode query -n ${zzm} -c '{"Function":"resolve","Args":["www.example.com"]}'
```

Expected behavior:

- derive TLD from the full domain;
- read the authority server address from ledger state;
- send a DNS query to that server;
- return `A` records in `Data.records`.

Branch note:

- this function belongs to the BIND integration line and is most relevant on `feature/bind9-support`.

## 8.7 `update`

Purpose:

- update a non-TLD domain record on an external DNS server

Call type:

- `Invoke`

Arguments:

| Position | Name | Type | Required | Description |
| --- | --- | --- | --- | --- |
| 1 | `domain` | string | yes | full domain |
| 2 | `ip` | string | yes | target record IP |

Example:

```bash
peer chaincode invoke -n ${zzm} -c '{"Function":"update","Args":["www.example.com","2.2.2.2"]}'
```

## 8.8 `delete`

Purpose:

- delete a non-TLD domain record from an external DNS server

Call type:

- `Invoke`

Arguments:

| Position | Name | Type | Required | Description |
| --- | --- | --- | --- | --- |
| 1 | `domain` | string | yes | full domain |

Example:

```bash
peer chaincode invoke -n ${zzm} -c '{"Function":"delete","Args":["www.example.com"]}'
```

## 8.9 Wrapper chaincode: `chaincode_domain_dns`

File:

- `examples/chaincode/go/chaincode_domain_dns/chaincode_domain_dns.go`

Purpose:

- serve as a wrapper chaincode in some branches;
- verify POW before invoking `chaincode_dns_reslover`;
- forward `TopLevelUpdate`, `TopLevelDelete`, and query operations to the DNS resolver chaincode.

Important limitation:

- the wrapped chaincode ID is hard-coded in the source file and must match the deployed DNS resolver chaincode instance.

## 9. Fabric Native API Reference

The platform still exposes Fabric 0.6 native REST APIs through `core/rest/rest_api.json`.

## 9.1 Main endpoints

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/chain` | `GET` | get chain height and latest hashes |
| `/chain/blocks/{Block}` | `GET` | get block by height |
| `/transactions/{ID}` | `GET` | get transaction by TXID |
| `/chaincode` | `POST` | deploy / invoke / query chaincode through JSON-RPC |
| `/registrar` | `POST` | register with membership service |
| `/registrar/{enrollmentID}` | `GET` | check user registration |
| `/registrar/{enrollmentID}` | `DELETE` | delete local login tokens |
| `/registrar/{enrollmentID}/ecert` | `GET` | get enrollment certificate |
| `/registrar/{enrollmentID}/tcert` | `GET` | get transaction certificates |

## 9.2 `/chaincode` payload

This endpoint uses JSON-RPC 2.0 style payloads.

Request shape:

```json
{
  "jsonrpc": "2.0",
  "method": "deploy | invoke | query",
  "params": {
    "type": 1,
    "chaincodeID": {
      "path": "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover",
      "name": "<CHAINCODE_ID_FOR_INVOKE_OR_QUERY>"
    },
    "ctorMsg": {
      "args": ["TopLevelQuest", "google.com"]
    },
    "secureContext": "admin",
    "confidentialityLevel": "PUBLIC"
  },
  "id": 1
}
```

Rules:

- `deploy`
  - uses `chaincodeID.path`
- `invoke` / `query`
  - use `chaincodeID.name`
- `ctorMsg.args`
  - first item is function name, remaining items are arguments

## 10. Operational Scripts

## 10.1 User registration script

`txscripts/register.sh` pre-registers built-in users:

- `JIM`
- `ADMIN`
- `TOM`
- `JACK`
- `TAN`

It expects the environment variable:

- `zzm`
  - deployed chaincode ID

## 10.2 Transaction generator

`txscripts/startTx.py` is a concurrency driver for replaying invoke commands from a shell script file.

Main parameters:

- `-t`
  - transaction script file path
- `-s`
  - transactions per second
- `-r`
  - number of rounds

Example:

```bash
python txscripts/startTx.py -t ./txscripts/5000/5000_2.sh -s 100 -r 5
```

This tool is intended for load generation and experiments, not for public API access.

## 11. Known Risks and Gaps

These items should be stated before public release.

### 11.1 Legacy platform risk

- this code is based on Fabric 0.6;
- it uses membersrvc, PBFT plugin wiring, and old deployment flows;
- it should be positioned as a legacy / research system.

### 11.2 Branch-level API drift

- `TopLevelUpdate` does not have a single stable parameter contract across all branches;
- POW branches add `target` and `nonce`;
- BIND branches add simple domain DNS operations that do not exist in the pure on-chain branch.

### 11.3 Security code is partially commented out

In the checked-out workspace snapshot:

- signature verification is not fully enforced in `verifyContent()`;
- POW validation appears branch-dependent and is partially commented in some files;
- ownership validation logic requires branch-by-branch verification before claiming production readiness.

### 11.4 Hard-coded values

Several places contain hard-coded values:

- chaincode IDs
- user certificates
- user private keys
- Byzantine actor identity and IP
- filesystem paths inside packaging scripts

These should be moved to configuration before external release.

## 12. Suggested Public Release Positioning

If this repository is to be opened to external users, the recommended positioning is:

- publish `feature/domain-reslover` or `feature/bind9-support` as the business branch;
- publish the `v0.6-*-pow` and `v0.6-*-nopow` branches as experiment branches;
- explicitly state that this is a Fabric 0.6 research/legacy codebase;
- include a branch compatibility matrix and an interface stability note.

## 13. Suggested Next Deliverables

The next recommended deliverables are:

1. a branch comparison appendix with file-level diff highlights;
2. a cleaned deployment guide with verified commands;
3. a tested API example collection for CLI and REST;
4. a code-vs-PPT discrepancy checklist before public release.
