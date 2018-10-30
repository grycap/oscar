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
import minio

# MISSING DELETE FUNCTIONS
class MinioClient():
    
    def __init__(self, function_name):
        self.function_name = function_name
        self.access_key = utils.get_environment_variable("MINIO_USER")
        self.secret_key = utils.get_environment_variable("MINIO_PASS")
        self.client = minio.Minio(utils.get_environment_variable("MINIO_ENDPOINT"),
                            access_key=self.access_key,
                            secret_key=self.secret_key,
                            secure=False)
             
    def create_input_bucket(self):
        self.create_bucket('{0}-in'.format(self.function_name))
        self.set_bucket_event_notification('{0}-in'.format(self.function_name))
        
    def create_output_bucket(self):
        self.create_bucket('{0}-out'.format(self.function_name))        
        
    def create_bucket(self, bucket_name):
        try:
            self.client.make_bucket(bucket_name)
        except minio.error.BucketAlreadyOwnedByYou as err:
            print(err)
        except minio.error.ResponseError as err:
            print(err)        
        
    def set_bucket_event_notification(self, bucket_name):
        try:
            notification = {'QueueConfigurations': [
                                {'Arn': 'arn:minio:sqs::1:webhook', 
                                 'Events': ['s3:ObjectCreated:*']
                                 }
                            ]}
            self.client.set_bucket_notification(bucket_name, notification)
        except minio.error.ResponseError as err:
            print(err)
    
    def get_output_bucket_name(self):
        return '{0}-out'.format(self.function_name)
    
    def get_access_key(self):
        return self.access_key
    
    def get_secret_key(self):
        return self.secret_key
    
        