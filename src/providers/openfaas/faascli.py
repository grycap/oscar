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
from os import makedirs, path
import src.utils as utils

def create_function(function_properties):
    # faas-cli new imagemagick --lang dockerfile
    
    func_name = function_properties.name
    # Create function folders (main folder, function name folder)
    func_folder = utils.join_paths(utils.get_temp_dir(), utils.get_random_uuid4_str())
    makedirs(func_folder , exist_ok=True)
    makedirs(utils.join_paths(func_folder, func_name) , exist_ok=True)
    
    # Copy yml and Dockerfile
    root_path = path.dirname(path.dirname(path.dirname(path.dirname(path.abspath(__file__)))))
    func_def_path = utils.join_paths(root_path, "function_template")
    # Get function definition paths
    func_def_path = utils.join_paths(func_def_path, "function.yml")
    func_def_dest_path = utils.join_paths(func_folder, "function.yml")
    #utils.copy_file(func_def_path, func_def_dest_path)
    # Get function Dockerfile paths
    func_dockerfile_path = utils.join_paths(func_def_path, "Dockerfile")
    func_dockerfile_dest_path = utils.join_paths(func_folder, func_name, "Dockerfile")
    #utils.copy_file(func_dockerfile_path, func_dockerfile_dest_path)

    # Modify function definition
    with open(func_def_path, 'r') as f_in:
        with open(func_def_dest_path, 'w') as f_out:
            for line  in f_in:
                f_out.write(line.replace("function_name", func_name))

    # Modify Dockerfile
    image_id = function_properties.image_id
    with open(func_dockerfile_path, 'r') as f_in:
        with open(func_dockerfile_dest_path, 'w') as f_out:
            for line  in f_in:
                f_out.write(line.replace("ubuntu", image_id))

    # Copy required files
    # fwatchdog
    fwatchdog_path = utils.join_paths(root_path, "bin", "fwatchdog-0.9.6")
    fwatchdog_path_dest = utils.join_paths(func_dockerfile_dest_path, "fwatchdog")
    utils.copy_file(fwatchdog_path, fwatchdog_path_dest)
    # supervisor.py
    supervisor_path = utils.join_paths(root_path, "src", "providers", "openfaas", "cloud", "supervisor.py")
    supervisor_path_dest = utils.join_paths(func_dockerfile_dest_path, "supervisor.py")
    utils.copy_file(supervisor_path, supervisor_path_dest)
    # script.sh
    func_script = function_properties.script
    script_path_dest = utils.join_paths(func_dockerfile_dest_path, "script.sh")   
    utils.create_file_with_content(script_path_dest, func_script)
    
    return func_folder

def build_function(func_yml):
    # faas-cli build -f imagemagick.yml
    return utils.execute_command_and_return_output(['faas-cli', 'build', '-f', func_yml])

def push_function(func_yml):
    # faas-cli push -f imagemagick.yml
    return  utils.execute_command_and_return_output(['faas-cli', 'push', '-f', func_yml])

def deploy_function(func_yml):
    # faas-cli deploy -f imagemagick.yml
    return  utils.execute_command_and_return_output(['faas-cli', 'deploy', '-f', func_yml])     
