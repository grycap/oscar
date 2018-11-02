# SCAR - Serverless Container-aware ARchitectures
# Copyright (C) 2011 - GRyCAP - Universitat Politecnica de Valencia
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

import src.utils as utils 
import requests
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
        self.function_args = function_args
        if 'name' in self.function_args:
            self.function_args["service"] = self.function_args['name']
        self.function_args["envProcess"] = "supervisor"
        if "envVars" not in self.function_args:    
            self.function_args["envVars"] = self.openfaas_envvars
        else:
            self.function_args["envVars"].update(self.openfaas_envvars)         
    
    def get_functions_info(self, json_response=False):
        url = "{0}/{1}".format(self.endpoint, self.functions_path)
        if 'name' in self.function_args:
            url = "{0}/{1}/{2}".format(self.endpoint, self.function_info, self.function_args['name'])
        response = requests.get(url)
        return json.loads(response.text) if json_response else response
    
    def create_function(self):
        return requests.post("{0}/{1}".format(self.endpoint, self.functions_path), json=self.function_args)
    
    def delete_function(self):
        payload = { 'functionName' : self.function_args['name'] }
        return requests.delete("{0}/{1}".format(self.endpoint, self.functions_path), json=payload)
    
    def update_function(self):
        pass
    
    def invoke_function(self, body, asynch=False):
        function_path = self.invoke_async_function if asynch else self.invoke_req_response_function
        url = "{0}/{1}/{2}".format(self.endpoint, function_path, self.function_args['name'])
        return requests.post(url, data=body)
    