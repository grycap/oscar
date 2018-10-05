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
import json

# MISSING DELETE FUNCTIONS
class MinioClient():
    
    get_config_command = ['mc', 'admin', 'config', 'get', 'myminio']
    
    def __init__(self):
        self.endpoint = utils.get_environment_variable("MINIO_ENDPOINT")
        self.mcuser = utils.get_environment_variable("MINIO_USER")
        self.mcpass = utils.get_environment_variable("MINIO_PASS")
        config_command = ['mc', 'config', 'host', 'add', 'myminio']
        config_command.extend([self.endpoint, self.muser, self.mcpass])
        print(utils.execute_command_and_return_output(config_command))
    
    def get_minio_config(self):
        return json.loads(utils.execute_command_and_return_output(self.get_config_command))
    
    def add_function_endpoint(self, function_name):
        eventgateway_endpoint = utils.get_environment_variable("EVENTGATEWAY_EVENTS_ENDPOINT")
        config = self.get_minio_config()
        webhook_id = len(config['notify']['webhook']) + 1
        config['notify']['webhook'][str(webhook_id)] = {'enable': True, 
                                                        'endpoint': '{0}/{1}'.format(eventgateway_endpoint,
                                                                                     function_name)}
        set_config_command = ['mc', 'admin', 'config', 'set', 'myminio']
        print(utils.execute_command_with_input_and_return_output(set_config_command, json.dumps(config).encode('utf-8')))
        
        while utils.execute_command(self.get_config_command) != 0:
            print('Waiting for minio configuration')
        
    def create_input_bucket(self, function_name):
        create_bucket_command = ['mc', 'mb', 'myminio/{0}-in'.format(function_name)]
        print(utils.execute_command_and_return_output(create_bucket_command))
        enable_webhook_command = ['mc', 'events', 'add', 'myminio/{0}-in'.format(function_name), 'arn:minio:sqs::1:webhook', '--events', 'put']
        print(utils.execute_command_and_return_output(enable_webhook_command))
    
    def create_output_bucket(self, function_name):
        create_bucket_command = ['mc', 'mb', 'myminio/{0}-out'.format(function_name)]
        print(utils.execute_command_and_return_output(create_bucket_command))
        
        