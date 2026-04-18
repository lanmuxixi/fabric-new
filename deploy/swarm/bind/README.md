# Bind9 Layout for the Swarm Experiment

These files are the starting point for the `bind9` service in `deploy/swarm/stack.yml`.

They assume:

- the authority DNS node is `dns-bind-01`
- zone files live under `/var/lib/bind`
- the Fabric DNS chaincode talks to Bind9 directly through dynamic update and query traffic

## Install on dns-bind-01

From the pulled repository on `dns-bind-01`:

```bash
./scripts/swarm/prepare-bind-layout.sh
```

That script copies the sample config and zone files into the host paths configured by:

- `BIND_CONFIG_DIR`
- `BIND_CACHE_DIR`
- `BIND_RECORDS_DIR`

## Dynamic update policy

The sample zone config currently uses:

```txt
allow-update { any; };
```

That is deliberately permissive for an internal experiment. Tighten it before using this outside the lab.

## Files

- `config/named.conf`: main Bind9 include file
- `config/named.conf.options`: daemon options
- `config/named.conf.local`: `com` and `cn` authority zones
- `records/db.com`: initial `com` zone
- `records/db.cn`: initial `cn` zone
