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
from src.providers.onpremises import dockercli
from src.providers.onpremises import eventgateway
from src.providers.onpremises import miniocli 

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
        self.create_docker_image(oscar_args)
        function_name = oscar_args['name']
        openfaas_args = self.get_openfaas_args()
        openfaas_args = self.manage_event_gateway(function_name, openfaas_args)
        openfaas_args = self.manage_minio(function_name, openfaas_args)
        print("OPENFAAS ARGS: ", openfaas_args)
        r = requests.post(self.endpoint + self.functions_path, json=openfaas_args)
        return r
    
    def create_docker_image(self, oscar_args):
        registry_image_id = dockercli.create_docker_image(**oscar_args)
        dockercli.push_docker_image(registry_image_id)
    
    def get_openfaas_args(self, registry_image_id, oscar_args):
        func_args = {"service" : oscar_args['name'],
                     "image" : registry_image_id,
                     "envProcess" : "supervisor",
                     "envVars" : {"sprocess" : "/tmp/user_script.sh",
                                  "read_timeout": "90",
                                  "write_timeout": "90" }
                     }
        return self. merge_dicts(func_args, oscar_args)
    
    def manage_event_gateway(self, function_name, func_args):
        event_gateway = eventgateway.EventGatewayClient()
        event_gateway.register_function(function_name)
        subscription_id = event_gateway.subscribe_event(function_name)
        func_args["envVars"]["eventgateway_sub_id"] = subscription_id
    
    def manage_minio(self, function_name, func_args):
        minio = miniocli.MinioClient(function_name)
        minio.create_input_bucket()
        minio.create_output_bucket()
        func_args["envVars"]["AWS_ACCESS_KEY_ID"] = minio.get_access_key()
        func_args["envVars"]["AWS_SECRET_ACCESS_KEY"] = minio.get_secret_key()
        func_args["envVars"]["OUTPUT_BUCKET"] = minio.get_output_bucket_name()
        return func_args        

    @flask_response
    def process_minio_event(self, minio_event):
        # Remove the bucketname'-in' part
        function_name = minio_event["Records"]["s3"]["bucket"]["name"][:-3]
        return eventgateway.EventGatewayClient().send_event(function_name, minio_event)

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
    
    def merge_dicts(self, d1, d2):
        '''
        Merge 'd1' and 'd2' dicts into 'd1'.
        'd1' has precedence over 'd2'
        '''
        for k,v in d2.items():
            if v:
                if k not in d1:
                    d1[k] = v
                elif type(v) is dict:
                    d1[k] = self.merge_dicts(d1[k], v)
                elif type(v) is list:
                    d1[k] += v
        return d1    
    
