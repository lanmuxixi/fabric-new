# Swarm Experiment Runbook

This directory contains the minimal deployment assets needed to run the
current `feature/swarm-adaptation` branch on Docker Swarm.

The stack is intentionally small:

- 4 validating peers (`vp0` to `vp3`)
- PBFT consensus
- one attachable overlay network shared by peer services and dynamically
  launched chaincode containers
- one pinned peer per Swarm node hostname

## Why this is different from the old compose files

Fabric 0.6 launches chaincode as plain Docker containers through the host
daemon. In Swarm, the peer service and the chaincode container must still end
up on the same L2/L3 fabric. This stack does that by aligning:

- the external overlay network name in `stack.yml`
- `CORE_VM_DOCKER_HOSTCONFIG_NETWORKMODE`
- the `dns.subnet` filter that `GetLocalIP()` now honors

Without those three pieces matching, peers in multi-NIC hosts often advertise
the wrong IP or chaincode ends up on the wrong network.

## Current branch API

The checked-out branch is still based on `feature/bind9-support`, so the
business chaincode interface here is the simpler one:

- `init`
- `add`
- `delete`
- `resolveDomain`

It does not expose the later `TopLevelUpdate` / `TopLevelGetAll` interface from
the attack-defense branches.

## Files

- `stack.yml`: 4-peer Swarm stack
- `.env.example`: deployment variables you should copy and edit
- `../../scripts/swarm/build-peer-image.sh`: build and retag the peer image
- `../../scripts/swarm/deploy-stack.sh`: create overlay network and deploy stack
- `../../scripts/swarm/remove-stack.sh`: remove the stack
- `../../scripts/swarm/deploy-dns-chaincode.sh`: deploy the DNS chaincode to
  `vp0`

## Quick Start

1. On the Swarm manager, copy `.env.example` to `.env` and adjust:
   - `FABRIC_PEER_IMAGE`
   - `VP0_NODE` to `VP3_NODE`
   - `CORE_DNS_SUBNET`
   - `VP0_ENDPOINT`
2. Build the peer image from this repo:

```bash
./scripts/swarm/build-peer-image.sh
```

3. Make the image available on every Swarm node:
   - either push it to a registry and update `FABRIC_PEER_IMAGE`
   - or export/load it manually on every node
4. Deploy the stack:

```bash
./scripts/swarm/deploy-stack.sh
```

5. Confirm the services are healthy:

```bash
docker stack services "${STACK_NAME:-fabricdns}"
docker service logs -f "${STACK_NAME:-fabricdns}_vp0"
```

## Deploy the DNS chaincode

After `vp0` is up, deploy the chaincode from the manager:

```bash
./scripts/swarm/deploy-dns-chaincode.sh
```

Default ctor in `.env.example` initializes two top-level mappings:

- `com -> 10.92.2.140`
- `cn -> 10.92.2.140`

Update `CHAINCODE_CTOR` if your authority server lives elsewhere.

The deploy command prints the generated chaincode name. Save that value for
subsequent invoke/query commands.

## Example query

Replace `<CHAINCODE_NAME>` with the name returned by the deploy command:

```bash
docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode query \
    -n <CHAINCODE_NAME> \
    -c '{"Function":"resolveDomain","Args":["www.example.com"]}'
```

## Teardown

```bash
./scripts/swarm/remove-stack.sh
```
