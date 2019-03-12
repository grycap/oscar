# Copyright (C) GRyCAP - I3M - UPV
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import json
import logging
import os

loglevel = logging.INFO
if "LOG_LEVEL" in os.environ:
    loglevel = os.environ["LOG_LEVEL"]
FORMAT = '%(asctime)s - %(name)s - %(levelname)s - %(message)s'
logging.basicConfig(level=loglevel, format=FORMAT)
logger = logging.getLogger('oscar')

def debug(cli_msg, log_msg=None):
    if loglevel == logging.DEBUG:
        print(cli_msg)
    logger.debug(log_msg) if log_msg else logger.debug(cli_msg)

def info(cli_msg=None, log_msg=None):
    if cli_msg and loglevel == logging.INFO:
        print(cli_msg)
    logger.info(log_msg) if log_msg else logger.info(cli_msg)

def warning(cli_msg, log_msg=None):
    print(cli_msg)
    logger.warning(log_msg) if log_msg else logger.warning(cli_msg)

def error(cli_msg, log_msg=None):
    if log_msg:
        print(log_msg)
        logger.error(log_msg)
    else:
        print(cli_msg)
        logger.error(cli_msg)
        
def exception(msg):
    logger.exception(msg)        

def log_exception(error_msg, exception):
    error(error_msg, error_msg + ": {0}".format(exception))

def print_json(value):
    print(json.dumps(value))

def info_json(cli_msg, log_msg=None):
    print_json(cli_msg)
    logger.info(log_msg) if log_msg else logger.info(cli_msg)

def warning_json(cli_msg, log_msg=None):
    print_json(cli_msg)
    logger.warning(log_msg) if log_msg else logger.warning(cli_msg)

def error_json(cli_msg, log_msg=None):
    print_json(cli_msg)
    logger.error(log_msg) if log_msg else logger.error(cli_msg)          
