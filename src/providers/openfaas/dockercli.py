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
from os import makedirs, path

def create_docker_image(**oscar_args):
    func_folder = utils.join_paths(utils.get_temp_dir(), utils.get_random_uuid4_str())
    makedirs(func_folder , exist_ok=True)
    root_path = path.dirname(path.dirname(path.dirname(path.dirname(path.abspath(__file__)))))
    # Get function Dockerfile paths
    func_dockerfile_path = utils.join_paths(root_path, "src", "providers", "openfaas", "function_template", "Dockerfile")
    func_dockerfile_dest_path = utils.join_paths(func_folder, "Dockerfile")

    # Modify Dockerfile
    with open(func_dockerfile_path, 'r') as f_in:
        with open(func_dockerfile_dest_path, 'w') as f_out:
            for line  in f_in:
                f_out.write(line.replace("FROM ubuntu", "FROM {0}".format(oscar_args['image'])))    
    
    # Copy required binaries
    bin_path = utils.join_paths(root_path, "bin")
    utils.copy_file(utils.join_paths(bin_path, "fwatchdog-0.9.6"), utils.join_paths(func_folder, "fwatchdog"))
    utils.copy_file(utils.join_paths(bin_path, "supervisor"), utils.join_paths(func_folder, "supervisor"))
    utils.copy_file(utils.join_paths(bin_path, "mc"), utils.join_paths(func_folder, "mc"))
    # Create user script
    utils.create_file_with_content(utils.join_paths(func_folder, "user_script.sh"),
                                   utils.base64_to_utf8_string(oscar_args['script']))    
    
    # docker build -t registry.docker-registry/function_name -f Dockerfile .
    build_command = ['docker', 'build']
    registry_image_id = "registry.docker-registry/{0}".format(oscar_args['name'])
    build_command.extend(["-t", registry_image_id])
    build_command.extend([func_folder])
    utils.execute_command_and_return_output(build_command)
    return registry_image_id
    
def push_docker_image(registry_image_id):    
    # docker push registry.docker-registry/function_name
    push_command = ['docker', 'push', registry_image_id]
    return utils.execute_command_and_return_output(push_command)
