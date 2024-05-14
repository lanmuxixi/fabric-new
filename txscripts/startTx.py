import argparse
import time
import traceback
import os
import threading

transaction_speed = 10  # how many transaction per second
file_location = './test_data_tld_update.sh'
times = 1
second_ns = 10 ** 9


def run_command(line):
    return_code = os.system(line)
    print("Return code:", return_code)


def now_time_ns():
    return int(time.time() * 10 ** 9)


if __name__ == '__main__':
    print(time.time())
    total_tx_number = 0
    parser = argparse.ArgumentParser(description="Process some integers.")
    parser.add_argument('-t', type=str, help='The file location', default=file_location)
    parser.add_argument('-s', type=int, help='The transaction speed (tx per second)', default=transaction_speed)
    parser.add_argument('-r', type=int, help='The round should send', default=times)
    args = parser.parse_args()
    file_location = args.t
    transaction_speed = args.s
    times = args.r
    # start the transaction for a
    try:
        with open(file_location, 'r') as file:
            for idx in range(0, times):
                print("Current is round: {}".format(idx))
                start_time = time.time()  # get the start time
                print("start_time: {}".format(start_time))
                threads = []
                transaction_number = 0
                while True:  
                    if time.time() - start_time > 1 or transaction_number >= transaction_speed:
                        break
                    line = file.readline()
                    thread = threading.Thread(target=run_command, args=(line,))
                    thread.start()
                    threads.append(thread)
                    total_tx_number += 1
                    transaction_number += 1
                for thread in threads:
                    thread.join()
                end_time = time.time()  # get the end time after the transaction
                time_difference_ms = (end_time - start_time) * 10 ** 3  # calculate the time difference in milliseconds
                print("Execute {} transaction, total use {}ms".format(transaction_number, int(time_difference_ms)))
                time.sleep((1000 - time_difference_ms) / 10 ** 3)
            print("Total Execute{} tx".format(total_tx_number))
    except Exception as e:
        traceback.print_exc()