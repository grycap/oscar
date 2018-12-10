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

import src.utils as utils 
import requests
import os.path
import json

class OpenFaasClient():
    
    functions_path = 'system/functions'
    function_info = 'system/function'
    invoke_req_response_function = 'function'
    invoke_async_function = 'async-function'
    
    def __init__(self, function_args):
        self.endpoint = utils.get_environment_variable("OPENFAAS_ENDPOINT")
        self.openfaas_envvars = {"sprocess": "/tmp/user_script.sh",
                                 "read_timeout": "90",
                                 "write_timeout": "90"}
        self.openfaas_labels = {"com.openfaas.scale.zero": "true"}
        self.set_function_args(function_args)
        self.basic_auth = None
        if (os.path.isfile('/var/secrets/basic-auth-user') and
           os.path.isfile('/var/secrets/basic-auth-password')):
            self.basic_auth = (utils.read_file('/var/secrets/basic-auth-user'),
                               utils.read_file('/var/secrets/basic-auth-password'))
        
    def set_function_args(self, function_args):
        self.function_args = function_args
        if 'name' in self.function_args:
            self.function_args["service"] = self.function_args['name']
        self.function_args["envProcess"] = "supervisor"
        if "envVars" not in self.function_args:    
            self.function_args["envVars"] = self.openfaas_envvars
        else:
            self.function_args["envVars"].update(self.openfaas_envvars)
        # Set 'com.openfaas.scale.zero=true' label to enable zero-scale
        if "labels" not in self.function_args:
            self.function_args["labels"] = self.openfaas_labels
        else:
            self.function_args["labels"].update(self.openfaas_labels)

    def get_functions_info(self, json_response=False):
        url = "{0}/{1}".format(self.endpoint, self.functions_path)
        if 'name' in self.function_args:
            url = "{0}/{1}/{2}".format(self.endpoint, self.function_info, self.function_args['name'])
        response = requests.get(url, auth=self.basic_auth)
        return json.loads(response.text) if json_response else response
    
    def create_function(self, function_args):
        self.set_function_args(function_args)
        return requests.post("{0}/{1}".format(self.endpoint, self.functions_path), json=self.function_args, auth=self.basic_auth)
    
    def delete_function(self):
        payload = { 'functionName' : self.function_args['name'] }
        return requests.delete("{0}/{1}".format(self.endpoint, self.functions_path), json=payload, auth=self.basic_auth)
    
    def update_function(self):
        pass
    
    def invoke_function(self, body, asynch=True):
        function_path = self.invoke_async_function if asynch else self.invoke_req_response_function
        url = "{0}/{1}/{2}".format(self.endpoint, function_path, self.function_args['name'])
        return requests.post(url, data=body)
    
    def is_function_created(self):
        function_path = self.invoke_req_response_function
        url = "{0}/{1}/{2}".format(self.endpoint, function_path, self.function_args['name'])
        response = requests.get(url)
        return (True, response) if response.status_code == 200 else (False, response)
    