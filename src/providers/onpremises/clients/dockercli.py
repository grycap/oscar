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
from os import makedirs, path
import docker

class DockerClient():

    @utils.lazy_property
    def client(self):
        # Requires the DOCKER_HOST variable available in the environment
        client = docker.from_env()
        return client
    
    def __init__(self, function_args):
        self.registry_name = utils.get_environment_variable("DOCKER_REGISTRY")
        self.function_args = function_args
        self.function_image_folder = utils.join_paths(utils.get_temp_dir(), utils.get_random_uuid4_str())
        self.root_path = path.dirname(path.dirname(path.dirname(path.dirname(path.dirname(path.abspath(__file__))))))     

    def copy_dockerfile(self):
        # Get function Dockerfile paths
        func_dockerfile_path = utils.join_paths(self.root_path, "src", "providers", "onpremises", "function_template", "Dockerfile")
        func_dockerfile_dest_path = utils.join_paths(self.function_image_folder, "Dockerfile")
        # Modify Dockerfile
        with open(func_dockerfile_path, 'r') as f_in:
            with open(func_dockerfile_dest_path, 'w') as f_out:
                for line in f_in:
                    f_out.write(line.replace("FROM ubuntu", "FROM {0}".format(self.function_args['image'])))
                    
    def copy_binaries(self):
        bin_path = utils.join_paths(self.root_path, "bin")
        utils.copy_file(utils.join_paths(bin_path, "fwatchdog-0.9.6"),
                        utils.join_paths(self.function_image_folder, "fwatchdog"))
        utils.copy_file(utils.join_paths(bin_path, "supervisor"),
                        utils.join_paths(self.function_image_folder, "supervisor"))
        
    def copy_user_script(self):
        utils.create_file_with_content(utils.join_paths(self.function_image_folder, "user_script.sh"),
                                       utils.base64_to_utf8_string(self.function_args['script']))       

    def copy_required_files(self):
        makedirs(self.function_image_folder , exist_ok=True)
        # Get function Dockerfile paths
        self.copy_dockerfile()   
        # Copy required binaries
        self.copy_binaries()
        # Create user script
        self.copy_user_script()
    
    def delete_image_files(self):
        # Delete all the temporal files created for the image creation
        utils.delete_folder(self.function_image_folder)

    def create_docker_image(self):
        # Copy/create function required files
        self.copy_required_files()    
        # Build the docker image
        self.registry_image_id = "{0}/{1}".format(self.registry_name, self.function_args['name'])        
        self.client.images.build(path=self.function_image_folder, tag=self.registry_image_id)
        # Avoid storing unnecessary files
        self.delete_image_files()
        
    def push_docker_image(self):
        self.client.images.push(self.registry_image_id)
