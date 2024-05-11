import argparse
import sys

# 创建 ArgumentParser 对象
parser = argparse.ArgumentParser(description='一个用来生成fabric节点配置文件的脚本')

# 添加命令行参数
parser.add_argument('-n', help='log文件路径')

# 解析命令行参数
args = parser.parse_args()

# 打印解析后的参数值
n = int(args.n)

print(f"正在生成 {n} 个节点的配置文件...")

config_name = f"{n}-peers.yml"

with open(config_name, 'w') as fp:
  sys.stdout = fp
  print('''version: '2'
        
services:
# validating node as the root
# vp0 will also be used for client interactive operations
# If you want to run fabric command on the host, then map 7051:7051 to host
# port, or use like `CORE_PEER_ADDRESS=172.17.0.2:7051` to specify peer addr.
  vp0:
    extends:
      file: peer.yml
      # service: vp
      service: vp_debug
    hostname: vp0
    environment:
      - CORE_PEER_ID=vp0

      # 恶意节点
      # - CORE_PBFT_GENERAL_BYZANTINE=true

    ports:
      - "7050:7050"
      # - "7051:7051" 
''')
  for i in range(1, n):
    print("  # validating node")
    print(f"  vp{i}:")
    print('''    extends:
      file: peer.yml
      service: vp
      # service: vp_debug''')
    print(f"    hostname: vp{i}")
    print("    environment:")
    print(f"      - CORE_PEER_ID=vp{i}", )
    print('''      - CORE_PEER_DISCOVERY_ROOTNODE=vp0:7051
    links:
      - vp0
''')
