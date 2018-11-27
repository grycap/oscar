# SCAR - Serverless Container-aware ARchitectures
# Copyright (C) 2018 - GRyCAP - Universitat Politecnica de Valencia
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
from flask import Response
from src.providers.onpremises.clients.dockercli import DockerClient
from src.providers.onpremises.clients.eventgateway import EventGatewayClient
from src.providers.onpremises.clients.minio import MinioClient
from src.providers.onpremises.clients.openfaas import OpenFaasClient

class CustomResponse():
    def __init__(self, content=None, status_code=None, headers=None):
        self.content = content if content else ''
        self.status_code = status_code if status_code else 500
        self.headers = headers if headers else {}

def flask_response(func):
    ''' Decorator used to create a flask Response '''
    def wrapper(*args, **kwargs):
        r = func(*args, **kwargs)
        kwargs = {'response' : r.content, 'status' : str(r.status_code), 'headers' : r.headers.items()}
        return Response(**kwargs)
    return wrapper

class OnPremises(Commands):
    
    @utils.lazy_property
    def openfaas(self):
        openfaas = OpenFaasClient(self.function_args)
        return openfaas
    
    @utils.lazy_property
    def event_gateway(self):
        event_gateway = EventGatewayClient(self.function_args)
        return event_gateway
    
    @utils.lazy_property
    def minio(self):
        minio = MinioClient(self.function_args)
        return minio    
    
    @utils.lazy_property
    def docker(self):
        docker = DockerClient(self.function_args)
        return docker
    
    def __init__(self, function_args=None):
        self.function_args = function_args if function_args else {}
    
    @flask_response    
    def init(self):
        function_exists, response = self.openfaas().is_function_created()
        if function_exists:
            return response
        else:
            yield CustomResponse(status_code=200)
            
            # Create docker image
            self.create_docker_image()
            self.set_docker_variables()
            # Create eventgateway connections
            self.manage_event_gateway()
            self.set_eventgateway_variables()
            # Create minio buckets
            self.create_minio_buckets()
            self.set_minio_variables()
            # Create openfaas function
            return self.openfaas.create_function(self.function_args)

    @flask_response
    def process_minio_event(self, minio_event):
        # Remove the bucketname'-in' part
        self.function_args['name'] = minio_event["Records"][0]["s3"]["bucket"]["name"][:-3]
        return self.event_gateway.send_event(minio_event)

    @flask_response        
    def ls(self):
        return self.openfaas.get_functions_info()

    @flask_response
    def invoke(self, body, asynch=True):
        return self.openfaas.invoke_function(body, asynch)
    
    def run(self):
        pass
    
    def update(self):
        pass   
    
    @flask_response    
    def rm(self):
        # Delete minio buckets (if selected)
        if 'deleteBuckets' in self.function_args and self.function_args['deleteBuckets']:
            self.minio.delete_input_bucket()
            self.minio.delete_output_bucket()
        # Delete event gateway registers
        self.event_gateway.deregister_function()
        self.event_gateway.unsubscribe_event(self.get_function_subscription_id())
        return self.openfaas.delete_function()

    def log(self):
        pass

    def put(self):
        pass

    def get(self):
        pass    
    
    def parse_arguments(self, args):
        pass
    
    def add_function_environment_variable(self, key, value):
        if "envVars" in self.function_args:
            self.function_args["envVars"][key] = value
        else:
            self.function_args["envVars"] = { key: value }
    
    def add_function_annotation(self, key, value):
        if "annotations" in self.function_args:
            self.function_args["annotations"][key] = value
        else:
            self.function_args["annotations"] = { key: value }        
    
    def create_docker_image(self):
        self.docker.create_docker_image()
        self.docker.push_docker_image()

    def set_docker_variables(self):  
        # Override the function image name
        self.function_args["image"] = self.docker.registry_image_id    
    
    def manage_event_gateway(self):
        self.event_gateway.register_function()
        self.event_gateway.subscribe_event()
        
    def set_eventgateway_variables(self):  
        self.add_function_annotation("eventgateway.subscription.id", self.event_gateway.subscription_id)      
    
    def create_minio_buckets(self):
        self.minio.create_input_bucket()
        self.minio.create_output_bucket()
        
    def set_minio_variables(self):
        self.add_function_environment_variable("AWS_ACCESS_KEY_ID", self.minio.get_access_key())
        self.add_function_environment_variable("AWS_SECRET_ACCESS_KEY", self.minio.get_secret_key())
        self.add_function_environment_variable("OUTPUT_BUCKET", self.minio.get_output_bucket_name())
        
    def get_function_subscription_id(self):
        function_info = self.openfaas.get_functions_info(json_response=True)
        return function_info["annotations"]["eventgateway.subscription.id"]     
