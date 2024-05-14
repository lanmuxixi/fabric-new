import argparse
import subprocess
import time
import traceback

transaction_speed = 100  # how many transaction per second
file_location = './10000_pow.sh'
second_ns = 10 ** 9


def run_command(line):
    peer_command = line.split()
    command_result = subprocess.Popen(peer_command, stdout=subprocess.PIPE)
    print(command_result.returncode)
    print(command_result.communicate()[0])


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description="Process some integers.")
    parser.add_argument('-t', type=str, help='The file location', default=file_location)
    parser.add_argument('-s', type=int, help='The transaction speed (tx per second)', default=transaction_speed)
    args = parser.parse_args()
    file_location = args.t
    transaction_speed = args.s
    start_time = time.time()  # get the start time
    print("start_time: {}".format(start_time))
    # start the transaction for a

    transaction_number = 0
    try:
        with open(file_location, 'r') as file:
            while True:
                if time.time() - start_time > 1 or transaction_number >= transaction_speed:
                    break
                line = file.readline()
                run_command(line)
                transaction_number += 1

    except Exception as e:
        traceback.print_exc()

    end_time = time.time()  # get the end time after the transaction
    time_difference_ms = (end_time - start_time) * 10 ** 3  # calculate the time difference in milliseconds

    print("Execute 1 transaction, total use {}ms".format(int(time_difference_ms)))
