import argparse
import re

# 创建 ArgumentParser 对象
parser = argparse.ArgumentParser(description='提取log文件中块大小并计算平均值')

# 添加命令行参数
parser.add_argument('-f', help='log文件路径')

# 解析命令行参数
args = parser.parse_args()

# 打印解析后的参数值
file_path = str(args.f)

print(f"正在打开{file_path}")

pattern = r"Block size is (\d+) bytes"
cnt = 3
sum = 0
k = 0

with open(file_path, 'r') as fp:
    # for _ in range(618):
        # fp.readline()
    for line in fp:
        # if "Block size is" in line:
        if "vp0-1" in line and "Block size is" in line:
            if cnt > 0:
                cnt -= 1
                continue
            k += 1
            
            # if(cnt > 0):
            #     cnt -= 1
            #     continue
            match = re.search(pattern, line)
            # 如果匹配成功，返回匹配到的数字，否则返回 None
            if match:
            #     # print(int(match.group(1)))
            #     k += 1
                print(int(match.group(1)))
                sum += int(match.group(1))
            if k >= 210:
                break

print(f"sum = {sum}, k = {k}")
avg = sum / k
print(f"avg = {avg}")