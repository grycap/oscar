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
import os
import stat
import requests
import logging
import time

class KanikoClient():

    jobs_path = "/apis/batch/v1/namespaces/kaniko-builds/jobs"
    
    def __init__(self, function_args):
        self.registry_name = utils.get_environment_variable("DOCKER_REGISTRY")
        self.function_args = function_args
        self.function_image_folder = utils.join_paths("/pv/kaniko-builds", utils.get_random_uuid4_str())
        self.root_path = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))))
        self.job_name = "{0}-build-job".format(function_args['name'])
        # Get k8s api host and port
        self.kubernetes_service_host = utils.get_environment_variable("KUBERNETES_SERVICE_HOST")
        if not self.kubernetes_service_host:
            self.kubernetes_service_host = "kubernetes.default"
        self.kubernetes_service_port = utils.get_environment_variable("KUBERNETES_SERVICE_PORT")
        if not self.kubernetes_service_port:
            self.kubernetes_service_port = "443"
        # Get k8s api token
        self.token = utils.read_file("/var/run/secrets/kubernetes.io/serviceaccount/token")
        # Get k8s api certs 
        if os.path.isfile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"):
            self.cert_verify = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
        else:
            self.cert_verify = False
        self.jobs_url = "https://{0}:{1}{2}".format(self.kubernetes_service_host, self.kubernetes_service_port, self.jobs_path)

    @utils.lazy_property
    def auth_header(self):
        return {'Authorization': 'Bearer ' + self.token}

    def copy_dockerfile(self):
        # Get function Dockerfile paths
        func_dockerfile_path = utils.join_paths(self.root_path, "src", "providers", "onpremises", "function_template", "Dockerfile")
        func_dockerfile_dest_path = utils.join_paths(self.function_image_folder, "Dockerfile")
        # Modify Dockerfile
        with open(func_dockerfile_path, 'r') as f_in:
            with open(func_dockerfile_dest_path, 'w') as f_out:
                for line in f_in:
                    f_out.write(line.replace("FROM ubuntu", "FROM {0}".format(self.function_args['image'])))
                    
    def download_binaries(self):
        # Download latest fwatchdog binary and set exec permissions
        utils.download_github_asset('openfaas', 'faas', 'fwatchdog', self.function_image_folder)
        fwatchdog_path = os.path.join(self.function_image_folder, 'fwatchdog')
        fwatchdog_st = os.stat(fwatchdog_path)
        os.chmod(fwatchdog_path, fwatchdog_st.st_mode | stat.S_IEXEC)
        # Download latest faas-supervisor binary and set exec permissions
        utils.download_github_asset('grycap', 'faas-supervisor', 'supervisor', self.function_image_folder)
        supervisor_path = os.path.join(self.function_image_folder, 'supervisor')
        supervisor_st = os.stat(supervisor_path)
        os.chmod(supervisor_path, supervisor_st.st_mode | stat.S_IEXEC)
        
    def copy_user_script(self):
        utils.create_file_with_content(utils.join_paths(self.function_image_folder, "user_script.sh"),
                                       utils.base64_to_utf8_string(self.function_args['script']))       

    def copy_required_files(self):
        os.makedirs(self.function_image_folder , exist_ok=True)
        # Get function Dockerfile paths
        self.copy_dockerfile()   
        # Download required binaries
        self.download_binaries()
        # Create user script
        self.copy_user_script()
    
    def delete_image_files(self):
        # Delete all the temporal files created for the image creation
        utils.delete_folder(self.function_image_folder)

    def wait_until_build_finishes(self):
        while True:
            r = requests.get("{0}/{1}".format(self.jobs_url, self.job_name), verify=self.cert_verify, headers=self.auth_header)
            if r.status_code != 200:
                logging.error("Error obtaining {0} info - {1}\n{2}".format(self.job_name, str(r.status_code), str(r.content)))
                break
            job = r.json()
            if utils.is_value_in_dict(job['status'], 'succeeded') and utils.is_value_in_dict(job['spec'], 'completions'):
                if job['status']['succeeded'] >= job['spec']['completions']:
                    # Delete succeeded job
                    self.delete_job()
                    break
            if utils.is_value_in_dict(job['status'], 'failed') and utils.is_value_in_dict(job['spec'], 'backoffLimit'):
                if job['status']['failed'] >= job['spec']['backoffLimit']:
                    logging.error("{0} failed! See pod logs for details.".format(self.job_name))
                    break
            time.sleep(5)


    def create_job_definition(self):
        self.registry_image_id = "{0}/{1}".format(self.registry_name, self.function_args['name'])
        job = {
            'apiVersion': 'batch/v1',
            'kind': 'Job',
            'metadata': {
                'name': self.job_name,
                'namespace': 'kaniko-builds',
            },
            'spec': {
                'template': {
                    'spec': {
                        'containers': [
                            {
                                'name': 'build',
                                'image': 'gcr.io/kaniko-project/executor:latest',
                                'args': ["-c", "/workspace/", "-d", self.registry_image_id, "--skip-tls-verify"],
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

    def delete_job(self):
        r = requests.delete("{0}/{1}".format(self.jobs_url, self.job_name), 
                            verify=self.cert_verify, 
                            headers=self.auth_header,
                            params={'propagationPolicy': 'Background'})
        if r.status_code not in [200, 202]:
            logging.error("Error deleting {0} - {1}\n{2}".format(self.job_name, str(r.status_code), str(r.content)))

    def create_and_push_docker_image(self):
        # Copy/create function required files
        self.copy_required_files()    
        # Build the docker image       
        job = self.create_job_definition()
        # Send request to the k8s api
        r = requests.post(self.jobs_url, json=job, verify=self.cert_verify, headers=self.auth_header)
        if r.status_code not in [200, 201, 202]:
            logging.error("Error creating {0} - {1}\n{2}".format(self.job_name, str(r.status_code), str(r.content)))
        # Wait until build finishes
        self.wait_until_build_finishes()
        # Avoid storing unnecessary files
        self.delete_image_files()
