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

from flask import Response
from src.cmdtemplate import Commands
from src.providers.onpremises.clients.kaniko import KanikoClient
from src.providers.onpremises.clients.kubernetes import KubernetesClient
from src.providers.onpremises.clients.minio import MinioClient
from src.providers.onpremises.clients.onedata import OnedataClient
from src.providers.onpremises.clients.openfaas import OpenFaasClient
from threading import Thread
import json
import random
import src.logger as logger
import src.utils as utils

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
        logger.debug("Initializing OpenFaas client")
        openfaas = OpenFaasClient(self.function_args)
        return openfaas

    @utils.lazy_property
    def minio(self):
        logger.debug("Initializing Minio client")
        if not hasattr(self, 'minio_id'):
            self.minio_id = self._get_storage_provider_id('MINIO')
        minio = MinioClient(self.function_args, self.minio_id)
        return minio

    @utils.lazy_property
    def onedata(self):
        logger.debug("Initializing Onedata client")
        onedata = OnedataClient(self.function_args, self.onedata_id)
        return onedata 
    
    @utils.lazy_property
    def kaniko(self):
        logger.debug("Initializing Kaniko client")
        kaniko = KanikoClient(self.function_args)
        return kaniko

    @utils.lazy_property
    def kubernetes(self):
        logger.debug("Initializing Kubernetes client")
        kubernetes = KubernetesClient()
        return kubernetes

    def __init__(self, function_args=None):
        if function_args:
            logger.debug("Function creation arguments received: {}".format(function_args))
        self.function_args = function_args if function_args else {}
        self.get_function_environment_variables()
    
    def init(self):
        function_exists, response = self.openfaas.is_function_created()
        if function_exists:
            logger.info("Function with name '{}' found".format(self.function_args['name']))
            kwargs = {'response' : response.content,
                      'status' : str(response.status_code),
                      'headers' : response.headers.items()}
            return Response(**kwargs)
        else:
            logger.info("Initialize asynchronous function creation")
            # Start initializing the function
            init_t = Thread(target=self.asynch_init)
            init_t.start()
            # Return response received
            kwargs = {'response' : 'Initializing function', 'status' : '200'}
            return Response(**kwargs)

    def asynch_init(self):
        # Create docker image
        logger.info("Creating docker image with kaniko")
        self.kaniko.create_and_push_docker_image(self.kubernetes)
        # Override the function image name with the new image_id
        self.function_args["image"] = self.kaniko.registry_image_id 
        # Create minio buckets
        logger.info("Creating minio buckets")
        self._set_minio_variables()
        self._create_minio_buckets()
        # Onedata stuff
        if self._is_onedata_defined():
            logger.info('Creating Onedata folders')
            self._create_onedata_folders()
            logger.info('Creating OneTrigger deployment')
            self.onedata.deploy_onetrigger(self.kubernetes)

        # Create openfaas function
        logger.info("Creating OpenFaas function")
        self._parse_output(self.openfaas.create_function(self.function_args))

    @flask_response
    def process_minio_event(self, minio_event):
        # Remove the bucketname'-in' part
        self.function_args['name'] = minio_event["Records"][0]["s3"]["bucket"]["name"][:-3]
        return self.openfaas.invoke_function(json.dumps(minio_event))

    @flask_response        
    def ls(self):
        logger.info("Retrieving functions information")
        return self.openfaas.get_functions_info()

    @flask_response
    def invoke(self, body, asynch=True):
        logger.info("Invoking '{}' function".format(self.function_args['name']))
        return self.openfaas.invoke_function(body, asynch)

    def run(self):
        pass

    @flask_response
    def update(self):
        logger.info("Update functionality not implemented yet")
        # Service not implemented (yet)
        return CustomResponse(content='Update functionality not implemented', status_code=501)
    
    @flask_response    
    def rm(self):
        # Delete minio buckets (if selected)
        if 'deleteBuckets' in self.function_args and self.function_args['deleteBuckets']:
            logger.info("Deleting Minio buckets")
            self.minio.delete_input_bucket()
            self.minio.delete_output_bucket()
        # Delete Onetrigger deployment and Onedata folders (if selected)
        if self._is_onedata_defined():
            logger.info("Deleting OneTrigger deployment")
            self.onedata.delete_onetrigger_deploy(self.kubernetes)
            if 'deleteBuckets' in self.function_args and self.function_args['deleteBuckets']:
                logger.info("Deleting Onedata folders")
                self.onedata.delete_input_folder()
                self.onedata.delete_output_folder()
        logger.info("Deleting OpenFaas function")
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

    def get_function_environment_variables(self):
        if 'envVars' not in self.function_args or len(self.function_args['envVars']) == 0:
            if 'name' in self.function_args:
                deploy_envvars = self.kubernetes.get_deployment_envvars(self.function_args['name'], 'openfaas-fn')
                for envvar in deploy_envvars:
                    if 'name' in envvar and 'value' in envvar:
                        self.add_function_environment_variable(envvar['name'], envvar['value'])

    def add_function_annotation(self, key, value):
        if "annotations" in self.function_args:
            self.function_args["annotations"][key] = value
        else:
            self.function_args["annotations"] = { key: value }        

    def _create_minio_buckets(self):
        self.minio.create_input_bucket()
        self.minio.create_output_bucket()

    def _is_onedata_defined(self):
        # Check if variables are defined in the function creation (without storage id...)
        # or in the remove function process.
        # In the first case would be needed to generate a new storage id and update 
        # variable names and values with the new definition before 'check_connection()'
        if 'envVars' in self.function_args:
            self.onedata_id = self._get_storage_provider_id('ONEDATA')
            if self.onedata_id and 'STORAGE_AUTH_ONEDATA_{}_HOST'.format(self.onedata_id) in self.function_args['envVars'] and \
               'STORAGE_AUTH_ONEDATA_{}_TOKEN'.format(self.onedata_id) in self.function_args['envVars'] and \
               'STORAGE_AUTH_ONEDATA_{}_SPACE'.format(self.onedata_id) in self.function_args['envVars']:
                return self.onedata.check_connection()
        return False

    def _get_storage_provider_id(self, storage_provider):
        '''
        Reads the global variables to get the provider's id.
        Variable schema:  STORAGE_AUTH_$1_$2_$3
        $1: MINIO | S3 | ONEDATA
        $2: STORAGE_ID (Specified in the function definition file, is unique for each storage defined)
        $3: USER | PASS | TOKEN | SPACE | HOST
        
        e.g.: STORAGE_AUTH_MINIO_12345_USER
        '''
        for envvar in self.function_args['envVars']:
            if envvar.startswith('STORAGE_AUTH_{}_'.format(storage_provider)):
                '''
                The provider_id can be composed by several fields but it's always between the position [3:-1]
                e.g.:
                  - "STORAGE_AUTH_MINIO_123_456_USER" -> ['STORAGE', 'AUTH', 'MINIO', '123', '456', 'USER']
                  - "STORAGE_AUTH_MINIO_123-456_USER" -> ['STORAGE', 'AUTH', 'MINIO', '123-456', 'USER']
                '''
                return "_".join(envvar.split("_")[3:-1])        

    def _create_onedata_folders(self):
        self.onedata.create_input_folder()
        self.onedata.create_output_folder()
        self._set_io_folder_variables(self._get_storage_provider_id('ONEDATA'))
        
    def _set_minio_variables(self):
        self.minio_id = random.randint(1,1000001)
        self.add_function_environment_variable("STORAGE_AUTH_MINIO_{}_USER".format(self.minio_id), self.minio.get_access_key())
        self.add_function_environment_variable("STORAGE_AUTH_MINIO_{}_PASS".format(self.minio_id), self.minio.get_secret_key())
        self._set_io_folder_variables(self.minio_id)
        
    def _set_io_folder_variables(self, provider_id):
        self.add_function_environment_variable("STORAGE_PATH_INPUT_{}".format(provider_id), self.minio.get_input_bucket_name())
        self.add_function_environment_variable("STORAGE_PATH_OUTPUT_{}".format(provider_id), self.minio.get_output_bucket_name())        

    def _parse_output(self, response):
        if response:
            if response.status_code == 200:
                logger.info("Request petition successful")
            else:
                logger.info("Request call returned code '{0}': {1}".format(response.status_code, response.text))
