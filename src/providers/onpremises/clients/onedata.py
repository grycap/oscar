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
import logging
import requests

class OnedataClient():

    deployments_path = "/apis/apps/v1/namespaces/oscar/deployments"
    cdmi_path = '/cdmi/'
    cdmi_version_header = {'X-CDMI-Specification-Version': '1.1.1'}
    cdmi_container_header = {'Content-Type': 'application/cdmi-container'}
    
    def __init__(self, function_args):
        self.function_name = function_args['name']
        if 'envVars' in function_args and 'OUTPUT_BUCKET' in function_args['envVars']:    
            self.output_bucket = function_args['envVars']['OUTPUT_BUCKET'].strip('/ ')
        self.oneprovider_host = function_args['envVars']['ONEPROVIDER_HOST']
        self.onedata_access_token = function_args['envVars']['ONEDATA_ACCESS_TOKEN']
        self.onedata_space = function_args['envVars']['ONEDATA_SPACE'].strip('/ ')
        # Get k8s api host and port
        self.kubernetes_service_host = utils.get_environment_variable("KUBERNETES_SERVICE_HOST")
        if not self.kubernetes_service_host:
            self.kubernetes_service_host = "kubernetes.default"
        self.kubernetes_service_port = utils.get_environment_variable("KUBERNETES_SERVICE_PORT")
        if not self.kubernetes_service_port:
            self.kubernetes_service_port = "443"
        # Get k8s api token
        self.kube_token = utils.read_file("/var/run/secrets/kubernetes.io/serviceaccount/token")
        # Get k8s api certs 
        if os.path.isfile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"):
            self.cert_verify = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
        else:
            self.cert_verify = False
        self.deployments_url = "https://{0}:{1}{2}".format(self.kubernetes_service_host, self.kubernetes_service_port, self.deployments_path)

    @utils.lazy_property
    def kube_auth_header(self):
        return {'Authorization': 'Bearer ' + self.kube_token}

    @utils.lazy_property
    def onedata_auth_header(self):
        return {'X-Auth-Token': self.onedata_access_token}

    def folder_exists(self, folder_name):
        folder_name = '{0}/'.format(folder_name.strip('/ '))
        url = 'https://{0}{1}{2}?children'.format(self.oneprovider_host, self.cdmi_path, self.onedata_space)
        headers = {**self.cdmi_version_header, **self.onedata_auth_header}
        try:
            r = requests.get(url, headers=headers)
            if r.status_code == 200:
                if folder_name in r.json()['children']:
                    return True
        except Exception as e:
            logging.warning('Cannot check if folder "{0}" exists. Error: {1}'.format(folder_name, e))
            return False
        return False

    def create_input_folder(self):
        self.create_folder('{0}-in'.format(self.function_name))

    def create_output_folder(self):
        if not hasattr(self, 'output_bucket'):
            self.create_folder('{0}-out'.format(self.function_name))

    def create_folder(self, folder_name):
        url = 'https://{0}{1}{2}/{3}/'.format(self.oneprovider_host, self.cdmi_path, self.onedata_space, folder_name)
        headers = {**self.cdmi_version_header, **self.cdmi_container_header, **self.onedata_auth_header}
        try:
            r = requests.put(url, headers=headers)
            if r.status_code in [201, 202]:
                logging.info('Folder "{0}" created successfully in space "{1}"'.format(folder_name, self.onedata_space))
            else:
                raise Exception(r.status_code)
        except Exception as e:
            logging.warning('Unable to create folder "{0}". Error: {1}'.format(folder_name, e))

    def delete_input_folder(self):
        self.delete_folder('{0}-in'.format(self.function_name))

    def delete_output_folder(self):
        self.delete_folder('{0}-out'.format(self.function_name))                    

    def delete_folder(self, folder_name):
        url = 'https://{0}{1}{2}/{3}/'.format(self.oneprovider_host, self.cdmi_path, self.onedata_space, folder_name)
        headers = {**self.cdmi_version_header, **self.onedata_auth_header}
        try:
            r = requests.delete(url, headers=headers)
            if r.status_code == 204:
                logging.info('Folder "{0}" deleted successfully in space "{1}"'.format(folder_name, self.onedata_space))
            else:
                raise Exception(r.status_code)
        except Exception as e:
            logging.warning('Unable to delete folder "{0}". Error: {1}'.format(folder_name, e))

    def get_output_bucket_name(self):
        return self.output_bucket if hasattr(self, 'output_bucket') else '{0}-out'.format(self.function_name)
    
    def get_oneprovider_host(self):
        return self.oneprovider_host

    def get_onedata_access_token(self):
        return self.onedata_access_token
    
    def get_onedata_space(self):
        return self.onedata_space

    def deploy_onetrigger(self):
        # K8s deployment object
        deploy = {
            'apiVersion': 'apps/v1',
            'kind': 'Deployment',
            'metadata': {
                'name': '{0}-onetrigger'.format(self.function_name),
                'namespace': 'oscar',
                'labels': {
                    'app': '{0}-onetrigger'.format(self.function_name)
                }
            },
            'spec': {
                'selector': {
                    'matchLabels': {
                        'app': '{0}-onetrigger'.format(self.function_name)
                    }
                },
                'replicas': 1,
                'template': {
                    'metadata': {
                        'labels': {
                            'app': '{0}-onetrigger'.format(self.function_name)
                        }
                    },
                    'spec': {
                        'containers': [
                            {
                                'name': 'onetrigger',
                                'image': 'grycap/onetrigger:latest',
                                'imagePullPolicy': 'Always',
                                'env': [
                                    {
                                        'name': 'ONEPROVIDER_HOST',
                                        'value': self.get_oneprovider_host()
                                    },
                                    {
                                        'name': 'ONEDATA_ACCESS_TOKEN',
                                        'value': self.get_onedata_access_token()
                                    },
                                    {
                                        'name': 'ONEDATA_SPACE',
                                        'value': self.get_onedata_space()
                                    },
                                    {
                                        'name': 'ONEDATA_SPACE_FOLDER',
                                        'value': '{0}-in'.format(self.function_name)
                                    },
                                    {
                                        'name': 'ONETRIGGER_WEBHOOK',
                                        'value': 'http://event-gateway:4000/{0}'.format(self.function_name)
                                    }
                                ]
                            }
                        ]
                    }
                }
            }
        }
        try:
            r = requests.post(self.deployments_url, json=deploy, headers=self.kube_auth_header, verify=self.cert_verify)
            if r.status_code in [200, 201, 202]:
                logging.info('Deployment "{0}" created successfully'.format(deploy['metadata']['name']))
            else:
                raise Exception(r.status_code)
        except Exception as e:
            logging.error('Unable to deploy OneTrigger. Error: {0}'.format(e))

    def delete_onetrigger_deploy(self):
        url = '{0}/{1}-onetrigger'.format(self.deployments_url, self.function_name)
        try:
            r = requests.delete(url, headers=self.kube_auth_header, verify=self.cert_verify)
            if r.status_code in [200, 202]:
                logging.info('Deployment "{0}-onetrigger" deleted successfully'.format(self.function_name))
            else:
                raise Exception(r.status_code)
        except Exception as e:
            logging.error('Unable to delete deployment "{0}-onetrigger". Error: {1}'.format(self.function_name, e))
