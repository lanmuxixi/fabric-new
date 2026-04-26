#!/usr/bin/env python3

import hashlib
import sys


def usage() -> int:
    print("Usage: calc-top-level-pow.py <domain> <authority-server> <target-hex>", file=sys.stderr)
    return 1


def main() -> int:
    if len(sys.argv) != 4:
        return usage()

    domain = sys.argv[1]
    authority = sys.argv[2]
    target_hex = sys.argv[3].strip().lower().removeprefix("0x")

    try:
        target = int(target_hex, 16)
    except ValueError:
        print("invalid target hex", file=sys.stderr)
        return 2

    payload = f"TopLevelUpdate|{domain}|{authority}"
    nonce = 0
    while True:
        digest = hashlib.sha256(f"{nonce}|{payload}".encode("utf-8")).digest()
        if int.from_bytes(digest, byteorder="big") <= target:
            print(nonce)
            return 0
        nonce += 1


if __name__ == "__main__":
    raise SystemExit(main())
