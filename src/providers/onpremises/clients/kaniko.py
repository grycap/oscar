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

import os
import stat
import src.utils as utils


class KanikoClient():

    namespace = 'kaniko-builds'

    def __init__(self, function_args):
        self.registry_name = utils.get_environment_variable("DOCKER_REGISTRY")
        self.function_args = function_args
        self.function_image_folder = utils.join_paths(
            '/pv/kaniko-builds',
            utils.get_random_uuid4_str())
        self.root_path = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))
        self.job_name = '{0}-build-job'.format(function_args['name'])

    def _copy_dockerfile(self):
        # Get function Dockerfile paths
        func_dockerfile_path = utils.join_paths(self.root_path,
                                                'src',
                                                'providers',
                                                'onpremises',
                                                'function_template',
                                                'Dockerfile')
        func_dockerfile_dest_path = utils.join_paths(
            self.function_image_folder,
            'Dockerfile')
        # Modify Dockerfile
        with open(func_dockerfile_path, 'r') as f_in:
            with open(func_dockerfile_dest_path, 'w') as f_out:
                for line in f_in:
                    f_out.write(line.replace(
                        'FROM ubuntu',
                        'FROM {0}'.format(self.function_args['image'])))

    def _download_binaries(self):
        # Download latest fwatchdog binary and set exec permissions
        utils.download_github_asset('openfaas',
                                    'faas',
                                    'fwatchdog',
                                    self.function_image_folder)
        fwatchdog_path = os.path.join(self.function_image_folder, 'fwatchdog')
        fwatchdog_st = os.stat(fwatchdog_path)
        os.chmod(fwatchdog_path, fwatchdog_st.st_mode | stat.S_IEXEC)
        # Download faas-supervisor binary and set exec permissions
        release = utils.get_environment_variable('SUPERVISOR_VERSION')
        utils.download_github_asset('grycap', 
                                    'faas-supervisor',
                                    'supervisor',
                                    self.function_image_folder,
                                    release=release)
        supervisor_path = os.path.join(self.function_image_folder, 'supervisor')
        supervisor_st = os.stat(supervisor_path)
        os.chmod(supervisor_path, supervisor_st.st_mode | stat.S_IEXEC)

    def _copy_user_script(self):
        utils.create_file_with_content(
            utils.join_paths(self.function_image_folder, 'user_script.sh'),
            utils.base64_to_utf8_string(self.function_args['script']))

    def _copy_required_files(self):
        os.makedirs(self.function_image_folder, exist_ok=True)
        # Get function Dockerfile paths
        self._copy_dockerfile()
        # Download required binaries
        self._download_binaries()
        # Create user script
        self._copy_user_script()

    def _delete_image_files(self):
        # Delete all the temporal files created for the image creation
        utils.delete_folder(self.function_image_folder)

    def _create_kaniko_job_definition(self):
        self.registry_image_id = '{0}/{1}'.format(self.registry_name,
                                                  self.function_args['name'])
        job = {
            'apiVersion': 'batch/v1',
            'kind': 'Job',
            'metadata': {
                'name': self.job_name,
                'namespace': self.namespace,
            },
            'spec': {
                'template': {
                    'spec': {
                        'containers': [
                            {
                                'name': 'build',
                                'image': 'gcr.io/kaniko-project/executor:latest',
                                'args': [
                                    '-c',
                                    '/workspace/',
                                    '-d',
                                    self.registry_image_id,
                                    '--skip-tls-verify',
                                    '--skip-tls-verify-pull'
                                ],
                                'resources': {
                                    'requests': {
                                        'memory': '256Mi',
                                        'cpu': '250m'
                                    }
                                },
                                'volumeMounts': [
                                    {
                                        'name': 'build-context',
                                        'mountPath': '/workspace'
                                    }
                                ]
                            }
                        ],
                        'restartPolicy': 'Never',
                        'volumes': [
                            {
                                'name': 'build-context',
                                'hostPath': {
                                    'path': self.function_image_folder,
                                    'type': 'Directory'
                                }
                            }
                        ]
                    }
                }
            }
        }
        return job

    def create_and_push_docker_image(self, kubernetes_client):
        # Copy/create function required files
        self._copy_required_files()
        # Build the docker image
        job = self._create_kaniko_job_definition()
        # Send request to the k8s api
        kubernetes_client.create_job(job, self.job_name, self.namespace)
        # Wait until build finishes
        kubernetes_client.wait_job(self.job_name, self.namespace, delete=True)
        # Avoid storing unnecessary files
        self._delete_image_files()
