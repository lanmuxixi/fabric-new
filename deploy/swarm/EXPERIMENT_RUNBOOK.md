# 10-Node Swarm Experiment Runbook

This file is the execution order for the current `feature/swarm-adaptation`
branch after pulling it onto the 4 VMs.

## 1. Pull the latest code

On every VM that hosts the repository:

```bash
cd /root/go/src/github.com/hyperledger/fabric
git checkout feature/swarm-adaptation
git pull origin feature/swarm-adaptation
```

Expected result:

- local HEAD reaches the latest commit on `feature/swarm-adaptation`

Common failures:

- local dirty working tree
- wrong remote branch
- repository path differs from the expected Fabric GOPATH path

Check:

```bash
git log --oneline -n 3
git status --short
```

## 2. Build the peer image

Run this on a node that has Docker and the Fabric build toolchain available:

```bash
./scripts/swarm/build-peer-image.sh
```

Expected result:

- `make peer-image` succeeds
- the resulting image is retagged as `${FABRIC_PEER_IMAGE}`

Common failures:

- `make` toolchain incomplete
- Docker daemon not reachable
- old Fabric build dependencies missing

Check:

```bash
docker images | grep fabric-dns-peer
```

## 3. Distribute the image to all 4 VMs

Use a registry or export/load manually. One workable manual path is:

```bash
docker save "${FABRIC_PEER_IMAGE}" -o /tmp/fabric-dns-peer-swarm.tar
scp /tmp/fabric-dns-peer-swarm.tar dns-fabric-02:/tmp/
scp /tmp/fabric-dns-peer-swarm.tar dns-fabric-03:/tmp/
scp /tmp/fabric-dns-peer-swarm.tar dns-bind-01:/tmp/
```

Then on each target VM:

```bash
docker load -i /tmp/fabric-dns-peer-swarm.tar
```

Expected result:

- all 4 VMs show the same `${FABRIC_PEER_IMAGE}`

Common failures:

- image tag mismatch with `.env`
- image only exists on manager and not on workers

## 4. Prepare `.env`

On the Swarm manager (`dns-fabric-01`):

```bash
cp deploy/swarm/.env.example deploy/swarm/.env
vi deploy/swarm/.env
```

Minimum values to verify:

- `FABRIC_PEER_IMAGE`
- `CORE_DNS_SUBNET=10.92.2.0/24`
- `VP0_NODE` to `VP9_NODE`
- `VP0_ENDPOINT=10.92.2.138:7051`
- `BIND_*` host directories
- malicious marker flags such as `VP0_BYZANTINE`, `VP4_BYZANTINE`, `VP8_BYZANTINE`

Expected result:

- `.env` matches the current 4-VM layout

Common failures:

- stale IPs from old experiments
- wrong node hostname values
- `VP0_ENDPOINT` pointing to the wrong host

## 5. Prepare Bind9 on dns-bind-01

On `dns-bind-01`:

```bash
cd /root/go/src/github.com/hyperledger/fabric
git checkout feature/swarm-adaptation
git pull origin feature/swarm-adaptation
cp deploy/swarm/.env.example deploy/swarm/.env
./scripts/swarm/prepare-bind-layout.sh
```

Expected result:

- config appears under `${BIND_CONFIG_DIR}`
- zone files appear under `${BIND_RECORDS_DIR}`

Common failures:

- target directories not writable
- wrong `.env` values on `dns-bind-01`

Check:

```bash
ls -R /opt/fabric-dns/bind
```

## 6. Deploy the stack

On the Swarm manager:

```bash
./scripts/swarm/deploy-stack.sh
```

Expected result:

- overlay network exists
- services `vp0` to `vp9` plus `bind9` appear in the stack

Common failures:

- Swarm inactive on the manager
- overlay network name mismatch
- node placement constraint does not match actual Swarm hostnames
- image missing on one or more workers

Check:

```bash
docker stack services "${STACK_NAME:-fabricdns}"
docker service ps "${STACK_NAME:-fabricdns}_vp0"
docker service ps "${STACK_NAME:-fabricdns}_bind9"
```

Logs:

```bash
docker service logs -f "${STACK_NAME:-fabricdns}_vp0"
docker service logs -f "${STACK_NAME:-fabricdns}_bind9"
```

## 7. Identify and enter the main peer container

`vp0` is the main deployment/query peer.

On the manager:

```bash
./scripts/swarm/get-service-container.sh vp0
./scripts/swarm/exec-vp0.sh
```

Expected result:

- the first command prints a running container ID
- the second command opens a shell inside `vp0`

Common failures:

- `vp0` service not scheduled
- `vp0` restarting repeatedly due PBFT or network config

## 8. Deploy the DNS chaincode

On the manager:

```bash
./scripts/swarm/deploy-dns-chaincode.sh
```

Expected result:

- deploy returns a chaincode name
- deploy output is written to `deploy/swarm/last-chaincode-deploy.log`
- detected chaincode name is written to `deploy/swarm/last-chaincode-id.txt`

Common failures:

- `VP0_ENDPOINT` unreachable
- peer cannot build the chaincode path
- Fabric 0.6 Docker-in-Docker network mismatch

Check:

```bash
cat deploy/swarm/last-chaincode-deploy.log
cat deploy/swarm/last-chaincode-id.txt
```

## 9. Query TopLevelGetAll

On the manager:

```bash
./scripts/swarm/query-top-levels.sh
```

Expected result:

- JSON-like response showing at least `com` and `cn` authority mappings

Common failures:

- chaincode name file missing
- chaincode deploy did not finish correctly
- `vp0` gRPC endpoint incorrect

Manual fallback:

```bash
docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode query \
    -n "$(cat deploy/swarm/last-chaincode-id.txt)" \
    -c '{"Function":"TopLevelGetAll","Args":[]}'
```

## 10. Update a DNS record

After deploy, use the returned chaincode name and run a chaincode invoke from the manager.
Example for `www.example.com -> 10.92.2.138`:

```bash
docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode invoke \
    -n "$(cat deploy/swarm/last-chaincode-id.txt)" \
    -c '{"Function":"update","Args":["www.example.com","10.92.2.138"]}'
```

Expected result:

- invoke succeeds without PBFT timeout

Common failures:

- Bind9 not yet reachable at `10.92.2.140:53`
- zone not configured for dynamic update

## 11. Verify through Bind9

On any host with `dig`:

```bash
./scripts/swarm/dig-authority.sh www.example.com
```

Expected result:

- the updated A record is returned

Common failures:

- Bind9 service not listening on host port 53
- chaincode invoke succeeded on ledger side but update did not reach Bind9
- DNS cache confusion during repeated tests

Manual fallback:

```bash
dig @10.92.2.140 www.example.com +short
```

## 12. Main risk list

- `GetLocalIP` and `CORE_DNS_SUBNET` must stay aligned with `10.92.2.0/24`
- `feature/v0.6-multi-host` YAMLs are reference-only and cannot be deployed directly in Swarm
- old scripts with hardcoded `zzm`, paths, or IPs must not be reused
- Fabric 0.6 chaincode launch depends on host Docker networking and is sensitive to the overlay network name
- the repository currently does not include a standalone reusable `addToZone` service, so Bind9 is the authority endpoint for now
