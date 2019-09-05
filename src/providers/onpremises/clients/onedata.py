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

import requests
import src.logger as logger
import src.utils as utils


class OnedataClient():

    namespace = 'oscar'
    cdmi_path = '/cdmi/'
    cdmi_version_header = {'X-CDMI-Specification-Version': '1.1.1'}
    cdmi_container_header = {'Content-Type': 'application/cdmi-container'}

    def __init__(self, function_args, onedata_id):
        self.function_name = function_args['name']
        self.onedata_id = onedata_id
        self.endpoint = utils.get_environment_variable('OPENFAAS_ENDPOINT')
        self.onetrigger_version = utils.get_environment_variable('ONETRIGGER_VERSION')
        if not self.onetrigger_version:
            self.onetrigger_version = 'latest'
        if ('envVars' in function_args and
                'OUTPUT_BUCKET' in function_args['envVars']):
            self.output_bucket = function_args['envVars']['OUTPUT_BUCKET'].strip('/ ')
        self.oneprovider_host = function_args['envVars']['ONEPROVIDER_HOST']
        self.onedata_access_token = function_args['envVars']['ONEDATA_ACCESS_TOKEN']
        self.onedata_space = function_args['envVars']['ONEDATA_SPACE'].strip('/ ')

    @utils.lazy_property
    def onedata_auth_header(self):
        return {'X-Auth-Token': self.onedata_access_token}

    def check_connection(self):
        if (self.oneprovider_host in [None, ''] or
                self.onedata_access_token in [None, ''] or
                self.onedata_space in [None, '']):
            return False
        else:
            url = f'https://{self.oneprovider_host}/api/v3/oneprovider/spaces'
            try:
                r = requests.get(url, headers=self.onedata_auth_header)
                if r.status_code == 200:
                    for space in r.json():
                        if self.onedata_space == space['name']:
                            return True
                    return False
                elif r.status_code == 401:
                    raise Exception('The provided Onedata access token is not valid. Skipping Onedata configuration.')
                else:
                    raise Exception(f'Error: {r.text} - {r.status_code}')
            except Exception as e:
                logger.error(e)
                return False

    def folder_exists(self, folder_name):
        folder_name = '{0}/'.format(folder_name.strip('/ '))
        url = 'https://{0}{1}{2}?children'.format(self.oneprovider_host,
                                                  self.cdmi_path,
                                                  self.onedata_space)
        headers = {**self.cdmi_version_header, **self.onedata_auth_header}
        try:
            r = requests.get(url, headers=headers)
            if r.status_code == 200:
                if folder_name in r.json()['children']:
                    return True
        except Exception as e:
            logger.warning(f'Cannot check if folder "{folder_name}" exists. Error: {e}')
            return False
        return False

    def create_input_folder(self):
        self._create_folder(f'{self.function_name}-in')

    def create_output_folder(self):
        if not hasattr(self, 'output_bucket'):
            self._create_folder(f'{self.function_name}-out')

    def _create_folder(self, folder_name):
        url = 'https://{0}{1}{2}/{3}/'.format(self.oneprovider_host,
                                              self.cdmi_path,
                                              self.onedata_space,
                                              folder_name)
        headers = {
            **self.cdmi_version_header,
            **self.cdmi_container_header,
            **self.onedata_auth_header
        }
        try:
            r = requests.put(url, headers=headers)
            if r.status_code in [201, 202]:
                logger.info(f'Folder "{folder_name}" created successfully in space "{self.onedata_space}"')
            else:
                raise Exception(r.status_code)
        except Exception as e:
            logger.warning(f'Unable to create folder "{folder_name}". Error: {e}')

    def delete_input_folder(self):
        self.delete_folder(f'{self.function_name}-in')

    def delete_output_folder(self):
        self.delete_folder(f'{self.function_name}-out')

    def delete_folder(self, folder_name):
        url = 'https://{0}{1}{2}/{3}/'.format(self.oneprovider_host,
                                              self.cdmi_path,
                                              self.onedata_space,
                                              folder_name)
        headers = {**self.cdmi_version_header, **self.onedata_auth_header}
        try:
            r = requests.delete(url, headers=headers)
            if r.status_code == 204:
                logger.info(f'Folder "{folder_name}" deleted successfully in space "{self.onedata_space}"')
            else:
                raise Exception(r.status_code)
        except Exception as e:
            logger.warning(f'Unable to delete folder "{folder_name}". Error: {e}')

    def get_output_bucket_name(self):
        if hasattr(self, 'output_bucket'):
            return self.output_bucket
        return f'{self.function_name}-out'

    def get_oneprovider_host(self):
        return self.oneprovider_host

    def get_onedata_access_token(self):
        return self.onedata_access_token

    def get_onedata_space(self):
        return self.onedata_space

    def deploy_onetrigger(self, kubernetes_client):
        # K8s deployment object
        deploy = {
            'apiVersion': 'apps/v1',
            'kind': 'Deployment',
            'metadata': {
                'name': f'{self.function_name}-onetrigger',
                'namespace': self.namespace,
                'labels': {
                    'app': f'{self.function_name}-onetrigger'
                }
            },
            'spec': {
                'selector': {
                    'matchLabels': {
                        'app': f'{self.function_name}-onetrigger'
                    }
                },
                'replicas': 1,
                'template': {
                    'metadata': {
                        'labels': {
                            'app': f'{self.function_name}-onetrigger'
                        }
                    },
                    'spec': {
                        'containers': [
                            {
                                'name': 'onetrigger',
                                'image': f'grycap/onetrigger:{self.onetrigger_version}',
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
                                        'value': f'{self.function_name}-in'
                                    },
                                    {
                                        'name': 'ONETRIGGER_WEBHOOK',
                                        'value': f'{self.endpoint}/async-function/{self.function_name}'
                                    }
                                ]
                            }
                        ]
                    }
                }
            }
        }
        kubernetes_client.create_deployment(deploy,
                                            self.function_name,
                                            self.namespace)

    def delete_onetrigger_deploy(self, kubernetes_client):
        kubernetes_client.delete_deployment(
            f'{self.function_name}-onetrigger',
            self.namespace)
