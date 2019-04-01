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
import minio

class MinioClient():
    
    def __init__(self, function_args, minio_id):
        self.function_name = function_args['name']
        if minio_id and 'envVars' in function_args and 'STORAGE_PATH_OUTPUT_'.format(minio_id) in function_args['envVars']:    
            self.output_bucket = function_args['envVars']['STORAGE_PATH_OUTPUT_'.format(minio_id)]
        self.access_key = utils.get_environment_variable("MINIO_USER")
        self.secret_key = utils.get_environment_variable("MINIO_PASS")
        self.client = minio.Minio(utils.get_environment_variable("MINIO_ENDPOINT"),
                                  access_key=self.access_key,
                                  secret_key=self.secret_key,
                                  secure=False)
             
    def create_input_bucket(self):
        self._create_bucket('{0}-in'.format(self.function_name))
        self._set_bucket_event_notification('{0}-in'.format(self.function_name))

    def create_output_bucket(self):
        if not hasattr(self, 'output_bucket'):
            self._create_bucket('{0}-out'.format(self.function_name))

    def _create_bucket(self, bucket_name):
        try:
            self.client.make_bucket(bucket_name)
        except minio.error.BucketAlreadyOwnedByYou as err:
            print(err)
        except minio.error.ResponseError as err:
            print(err)        

    def _set_bucket_event_notification(self, bucket_name):
        try:
            notification = {'QueueConfigurations': [
                                {'Arn': 'arn:minio:sqs::1:webhook', 
                                 'Events': ['s3:ObjectCreated:*']}
                            ]}
            self.client.set_bucket_notification(bucket_name, notification)
        except minio.error.ResponseError as err:
            print(err)
            
    def delete_input_bucket(self):
        self._delete_bucket_event_notification('{0}-in'.format(self.function_name))
        self._delete_bucket('{0}-in'.format(self.function_name))
        
    def delete_output_bucket(self):
        self._delete_bucket('{0}-out'.format(self.function_name))                    
     
    def _delete_bucket_files(self, bucket_name):
        try:
            for file in self.client.list_objects_v2(bucket_name):
                self.client.remove_object(bucket_name, file.object_name)
        except minio.error.ResponseError as err:
            print(err)     
           
    def _delete_bucket(self, bucket_name):
        try:
            self._delete_bucket_files(bucket_name)
            self.client.remove_bucket(bucket_name)
        except minio.error.ResponseError as err:
            print(err)
            
    def _delete_bucket_event_notification(self, bucket_name):
        try:
            notification = {'QueueConfigurations': []}
            self.client.set_bucket_notification(bucket_name, notification)
        except minio.error.ResponseError as err:
            print(err)            
    
    def get_input_bucket_name(self):
        return self.output_bucket if hasattr(self, 'input_bucket') else '{0}-in'.format(self.function_name)    
    
    def get_output_bucket_name(self):
        return self.output_bucket if hasattr(self, 'output_bucket') else '{0}-out'.format(self.function_name)
    
    def get_access_key(self):
        return self.access_key
    
    def get_secret_key(self):
        return self.secret_key
    
        