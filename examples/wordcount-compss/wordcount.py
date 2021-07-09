#!/usr/bin/python
#
#  Copyright 2002-2019 Barcelona Supercomputing Center (www.bsc.es)
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
#

# -*- coding: utf-8 -*-

"""Wordcount self read"""

import sys
import pickle
import time
from pycompss.api.api import compss_wait_on
from pycompss.api.task import task
from pycompss.api.parameter import INOUT


#@task(returns=dict)
def wordcount_f(path_file, start, size_block):
    fp = open(path_file)
    fp.seek(start)
    aux = fp.read(size_block)
    fp.close()
    data = aux.strip().split(" ")
    partial_result = {}
    for entry in data:
        if entry not in partial_result:
            partial_result[entry] = 1
        else:
            partial_result[entry] += 1
    return partial_result


#@task(dic1=INOUT)
def merge(dic1, dic2):
    for k in dic2:
        if k in dic1:
            dic1[k] += dic2[k]
        else:
            dic1[k] = dic2[k]


@task()
def main(path_file, result_file, size_block):

    size_block = int(size_block)

    print("Start")
    start = time.time()
    data = open(path_file)
    data.seek(0, 2)
    file_size = data.tell()
    data.close()
    ind = 0

    result = {}
    while ind < file_size:
        partial_result = wordcount_f(path_file, ind, size_block)
        merge(result, partial_result)
        ind += int(size_block)
    result = compss_wait_on(result)

    print("Elapsed Time: %s" % (time.time() - start))

    with open(result_file, 'w') as fd:
        for k, v in result.items():
            fd.write(str(k) + " : " + str(v) + "\n")


if __name__ == "__main__":
    path_file = sys.argv[1]
    result_file = sys.argv[2]
    size_block = sys.argv[3]

    main(path_file, result_file, size_block)
