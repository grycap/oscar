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
from src.providers.openfaas import dockercli
from src.providers.openfaas import eventgateway
from src.providers.openfaas import miniocli 

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
        self.endpoint = utils.get_environment_variable("OPENFAAS_ENDPOINT")
        
    @flask_response        
    def ls(self, function_name=None):
        path = self.functions_path
        if function_name:
            path = self.function_info + function_name
        return requests.get(self.endpoint + path)
    
    @flask_response    
    def init(self, **oscar_args):
        print("OSCAR ARGS: ", oscar_args)
        path = self.functions_path
        registry_image_id = dockercli.create_docker_image(**oscar_args)
        dockercli.push_docker_image(registry_image_id)
        
        function_name = oscar_args['name']
        
        event_gateway = eventgateway.EventGatewayClient()
        event_gateway.register_function(function_name)
        subscription_id = event_gateway.subscribe_event(function_name)

        mcuser = utils.get_environment_variable("MINIO_USER")
        mcpass = utils.get_environment_variable("MINIO_PASS")
        openfaas_args = {"service" : function_name,
                         "image" : registry_image_id,
                         "envProcess" : "supervisor",
                         "envVars" : { "sprocess" : "/tmp/user_script.sh",
                                       "eventgateway_sub_id" : subscription_id,
                                       "AWS_ACCESS_KEY_ID" : mcuser,
                                       "AWS_SECRET_ACCESS_KEY" : mcpass } }
        print("OPENFAAS ARGS: ", openfaas_args)        
        r = requests.post(self.endpoint + path, json=openfaas_args)
        
        minio = miniocli.MinioClient()
        webhook_id = minio.add_function_endpoint(function_name)
        minio.create_input_bucket(function_name, webhook_id)
        minio.create_output_bucket(function_name)        
        
        return r

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
    
