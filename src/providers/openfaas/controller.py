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

from src.cmdtemplate import Commands
import src.utils as utils 
import requests
from flask import Response
from . import faascli 

def flask_response(func):
    '''
    Decorator used to create a flask Response
    '''
    def wrapper(*args, **kwargs):
        r = func(*args, **kwargs)
        kwargs = {'response' : r.content, 'status' : str(r.status_code), 'headers' : r.headers.items()}
        return Response(**kwargs)
    return wrapper

class OpenFaas(Commands):
    
    functions_path = '/system/functions'
    function_info = '/system/function/'
    invoke_req_response_function = '/function/'
    invoke_async_function = '/async-function/'
    system_info = '/system/info'
    
    def __init__(self):
        self.endpoint = utils.get_environment_variable("OPENFAAS_URL")
        
    @flask_response        
    def ls(self, function_name=None):
        path = self.functions_path
        if function_name:
            path = self.function_info + function_name
        return requests.get(self.endpoint + path)
    
    @flask_response    
    def init(self, **kwargs):
        print(kwargs)
        path = self.functions_path
        
        func_name = kwargs['name']
        func_folder = faascli.create_function(**kwargs)
        func_yml = utils.join_paths(func_folder, '{0}.yml'.format(func_name))
        print(faascli.build_function(func_yml))
        print(faascli.push_function(func_yml))
        print(faascli.deploy_function(func_yml))
          
#         r = requests.post(self.endpoint + path, json=kwargs)
#         return r

    @flask_response
    def invoke(self, function_name, body, asynch=False):
        path = self.invoke_req_response_function
        if asynch:
            path = self.invoke_async_function
        return requests.post(self.endpoint + path + function_name, data=body)
    
    def run(self):
        pass
    
    def update(self):
        pass    
    
    @flask_response    
    def rm(self, function_name):
        payload = { 'functionName' : function_name }
        return requests.delete(self.endpoint + self.functions_path, json=payload)

    def log(self):
        pass

    def put(self):
        pass

    def get(self):
        pass    
    
    def parse_arguments(self, args):
        pass
    
