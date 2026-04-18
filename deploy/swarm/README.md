# Swarm Experiment Runbook

This directory now targets the 10-node Docker Swarm experiment described in
the task brief:

- `vp0` `vp1` `vp2` on `dns-fabric-01`
- `vp3` `vp4` `vp5` on `dns-fabric-02`
- `vp6` `vp7` on `dns-fabric-03`
- `vp8` `vp9` plus Bind9 on `dns-bind-01`

The stack keeps the multi-host PBFT shell from `feature/v0.6-multi-host`, but
switches the DNS business logic to `examples/chaincode/go/chaincode_dns_reslover`
from `feature/domain-reslover`.

## Why the old compose files are not deployable as-is

The historical files such as `peer.yml`, `4-peers.yml`, `10-peers.yml`, and
`10vp_1nvp.yml` are useful reference material, but they are still Compose-era
artifacts and cannot be used directly with `docker stack deploy` because they
depend on:

- `extends`
- `links`
- single-host Compose semantics
- hardcoded image names and bootstrap assumptions

The Swarm migration keeps their peer naming, PBFT sizing, and root-node
discovery pattern, but rewrites the services as explicit Swarm services.

## Networking constraints that matter

Fabric 0.6 launches chaincode through the host Docker daemon. In Swarm, that
means three things must agree:

- the attachable overlay network name in `stack.yml`
- `CORE_VM_DOCKER_HOSTCONFIG_NETWORKMODE`
- the `dns.subnet` / `CORE_DNS_SUBNET` value used by `GetLocalIP()`

Without that alignment, peers on multi-NIC hosts often advertise the wrong IP
or launch chaincode containers onto a network the peers cannot reach.

## Current chaincode interface

The DNS chaincode on this branch now exposes:

- `init`
- `resolve`
- `update`
- `delete`
- `TopLevelQuest`
- `TopLevelUpdate`
- `TopLevelDelete`
- `TopLevelGetAll`

`TopLevelGetAll` is used as the basic verification query after deployment.

## Files

- `stack.yml`: 10 validating peers plus one Bind9 service
- `.env.example`: deployment and placement variables
- `bind/`: sample Bind9 config and initial `com` / `cn` zones
- `../../scripts/swarm/build-peer-image.sh`: build and retag the peer image
- `../../scripts/swarm/deploy-stack.sh`: create overlay network and deploy the stack
- `../../scripts/swarm/remove-stack.sh`: remove the stack
- `../../scripts/swarm/deploy-dns-chaincode.sh`: deploy the DNS chaincode through `vp0`
- `../../scripts/swarm/query-top-levels.sh`: run the `TopLevelGetAll` verification query
- `../../scripts/swarm/get-service-container.sh`: resolve the running container for a Swarm service
- `../../scripts/swarm/exec-vp0.sh`: enter the `vp0` container shell
- `../../scripts/swarm/prepare-bind-layout.sh`: install the sample Bind9 config on `dns-bind-01`
- `../../scripts/swarm/dig-authority.sh`: verify DNS answers through Bind9

## Build

The old `build_image.sh` from `feature/v0.6-multi-host` is not directly usable:
it hardcodes old paths, an old `zzm`, and copies files into a separate image
workspace. The replacement for this branch is:

```bash
./scripts/swarm/build-peer-image.sh
```

That script runs `make peer-image` in-repo and retags the resulting image for
Swarm use. `membersrvc-image` is not required for this experiment because the
current setup runs with peer security disabled.

## Deploy the stack

1. Copy `.env.example` to `.env` on the Swarm manager and adjust the image,
   peer placement, and Bind9 paths.
2. Make sure the peer image exists on every Swarm node.
3. Make sure the Bind9 directories already exist on `dns-bind-01`:
   - `${BIND_CONFIG_DIR}`
   - `${BIND_CACHE_DIR}`
   - `${BIND_RECORDS_DIR}`
   Or just run:

```bash
./scripts/swarm/prepare-bind-layout.sh
```

4. Deploy:

```bash
./scripts/swarm/deploy-stack.sh
```

5. Check status:

```bash
docker stack services "${STACK_NAME:-fabricdns}"
docker service ps "${STACK_NAME:-fabricdns}_vp0"
docker service logs -f "${STACK_NAME:-fabricdns}_vp0"
```

## Deploy the DNS chaincode

After `vp0` is healthy, deploy from the manager:

```bash
./scripts/swarm/deploy-dns-chaincode.sh
```

The default ctor seeds:

- `com -> 10.92.2.140:53`
- `cn -> 10.92.2.140:53`

The deploy command prints the generated chaincode name. Save that value as
`${zzm}` or another shell variable for later queries.

The helper also writes:

- the raw deploy output to `deploy/swarm/last-chaincode-deploy.log`
- the detected chaincode name to `deploy/swarm/last-chaincode-id.txt`

## Verification query

The shortest path is now:

```bash
./scripts/swarm/query-top-levels.sh
```

If you want to run it manually, replace `<CHAINCODE_NAME>` with the value returned by deploy:

```bash
docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode query \
    -n <CHAINCODE_NAME> \
    -c '{"Function":"TopLevelGetAll","Args":[]}'
```

## Locate and enter vp0

The main deployment/query peer is `vp0`. To resolve its container ID on the manager:

```bash
./scripts/swarm/get-service-container.sh vp0
```

To enter that container directly:

```bash
./scripts/swarm/exec-vp0.sh
```

## Bind9 / addToZone note

The repository does not currently contain a standalone reusable `addToZone`
daemon. The current design therefore treats Bind9 itself as the authoritative
DNS update/query endpoint and lets the chaincode talk to it directly through
dynamic DNS update/query calls. If a separate sync helper is introduced later,
it should consume the deployed chaincode name and run on `dns-bind-01`.

Basic DNS verification from any host that has `dig`:

```bash
./scripts/swarm/dig-authority.sh www.example.com
```

## Teardown

```bash
./scripts/swarm/remove-stack.sh
```
