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

from src.cmdtemplate import Commands
import src.utils as utils 
from flask import Response
from src.providers.onpremises.clients.kaniko import KanikoClient
from src.providers.onpremises.clients.eventgateway import EventGatewayClient
from src.providers.onpremises.clients.minio import MinioClient
from src.providers.onpremises.clients.openfaas import OpenFaasClient
from threading import Thread
import logging

loglevel = logging.DEBUG
FORMAT = '%(asctime)s - %(name)s - %(levelname)s - %(message)s'
logging.basicConfig(format=FORMAT, level=loglevel)

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
        logging.debug("Initializing OpenFaas client")
        openfaas = OpenFaasClient(self.function_args)
        return openfaas
    
    @utils.lazy_property
    def event_gateway(self):
        logging.debug("Initializing EventGateway client")
        event_gateway = EventGatewayClient(self.function_args)
        return event_gateway
    
    @utils.lazy_property
    def minio(self):
        logging.debug("Initializing Minio client")
        minio = MinioClient(self.function_args)
        return minio    
    
    @utils.lazy_property
    def kaniko(self):
        logging.debug("Initializing Kaniko client")
        kaniko = KanikoClient(self.function_args)
        return kaniko
    
    def __init__(self, function_args=None):
        if function_args:
            logging.debug("Function creation arguments received: {}".format(function_args))
        self.function_args = function_args if function_args else {}
    
    def init(self):
        function_exists, response = self.openfaas.is_function_created()
        if function_exists:
            logging.info("Function with name '{}' found".format(self.function_args['name']))
            kwargs = {'response' : response.content,
                      'status' : str(response.status_code),
                      'headers' : response.headers.items()}
            return Response(**kwargs)
        else:
            logging.info("Initialize asynchronous function creation")
            # Start initializing the function
            init_t = Thread(target=self.asynch_init)
            init_t.start()
            # Return response received
            kwargs = {'response' : 'Initializing function', 'status' : '200'}
            return Response(**kwargs)

    def asynch_init(self):
        # Create docker image
        logging.info("Creating docker image with kaniko")
        self.kaniko.create_and_push_docker_image()
        self.set_docker_variables()
        # Create eventgateway connections
        logging.info("Creating event gateway connections")
        self.manage_event_gateway()
        self.set_eventgateway_variables()
        # Create minio buckets
        logging.info("Creating minio buckets")
        self.create_minio_buckets()
        self.set_minio_variables()
        # Create openfaas function
        logging.info("Creating OpenFaas function")
        self._parse_output(self.openfaas.create_function(self.function_args))

    @flask_response
    def process_minio_event(self, minio_event):
        # Remove the bucketname'-in' part
        self.function_args['name'] = minio_event["Records"][0]["s3"]["bucket"]["name"][:-3]
        return self.event_gateway.send_event(minio_event)

    @flask_response        
    def ls(self):
        logging.info("Retrieving functions information")
        return self.openfaas.get_functions_info()

    @flask_response
    def invoke(self, body, asynch=True):
        logging.info("Invoking '{}' function".format(self.function_args['name']))
        return self.openfaas.invoke_function(body, asynch)
    
    def run(self):
        pass
    
    @flask_response
    def update(self):
        logging.info("Update functionality not implemented yet")
        # Service not implemented (yet)
        return CustomResponse(content='Update functionality not implemented', status_code=501)
    
    @flask_response    
    def rm(self):
        # Delete minio buckets (if selected)
        if 'deleteBuckets' in self.function_args and self.function_args['deleteBuckets']:
            logging.info("Deleting Minio buckets")
            self.minio.delete_input_bucket()
            self.minio.delete_output_bucket()
        # Delete event gateway registers
        logging.info("Deleting EventGateway subscriptions and registers")
        self.event_gateway.unsubscribe_event(self.get_function_subscription_id())
        self.event_gateway.deregister_function()
        logging.info("Deleting OpenFaas function")
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

    def set_docker_variables(self):  
        # Override the function image name
        self.function_args["image"] = self.kaniko.registry_image_id    
    
    def manage_event_gateway(self):
        if not self.event_gateway.is_function_registered():
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
    
    def _parse_output(self, response):
        if response:
            if response.status_code == 200:
                logging.info("Request petition successful")
            else:
                logging.error("Request call returned code '{0}': {1}".format(response.status_code, response.text))
