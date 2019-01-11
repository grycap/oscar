# OSCAR - On-premises Serverless Container-aware ARchitectures
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
import faassupervisor.utils as utils
from faassupervisor.supervisor import Supervisor

logger = utils.get_logger()
logger.info('SCAR: Loading OpenFaas function')

def get_input():
    f_input = str(utils.get_environment_variable('OSCAR_EVENT')) if utils.is_variable_in_environment('OSCAR_EVENT') else utils.get_stdin()
    return json.loads(f_input)

def function_handler():
    f_input = get_input()
    print("Received input: {0}".format(f_input))    
    supervisor = Supervisor('openfaas', event=f_input)
    try:
        supervisor.parse_input()
        supervisor.execute_function()                                      
        supervisor.parse_output()
    except Exception as ex:
        exception_msg = "Exception launched:\n {0}".format(ex)
        print(exception_msg)

if __name__ == "__main__":
    function_handler()
