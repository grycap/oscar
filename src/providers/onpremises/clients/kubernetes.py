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
import time
import requests
import src.logger as logger
import src.utils as utils


class KubernetesClient():

    jobs_path = '/apis/batch/v1/namespaces/{0}/jobs'
    deployments_path = '/apis/apps/v1/namespaces/{0}/deployments'

    def __init__(self):
        # Get k8s api host and port
        self.kubernetes_service_host = utils.get_environment_variable('KUBERNETES_SERVICE_HOST')
        if not self.kubernetes_service_host:
            self.kubernetes_service_host = 'kubernetes.default'
        self.kubernetes_service_port = utils.get_environment_variable('KUBERNETES_SERVICE_PORT')
        if not self.kubernetes_service_port:
            self.kubernetes_service_port = '443'
        # Get k8s api token
        self.kube_token = utils.read_file('/var/run/secrets/kubernetes.io/serviceaccount/token')
        # Get k8s api certs 
        if os.path.isfile('/var/run/secrets/kubernetes.io/serviceaccount/ca.crt'):
            self.cert_verify = '/var/run/secrets/kubernetes.io/serviceaccount/ca.crt'
        else:
            self.cert_verify = False

    @utils.lazy_property
    def auth_header(self):
        return {'Authorization': 'Bearer ' + self.kube_token}

    def create_job(self, definition, name, namespace):
        jobs_path = self.jobs_path.format(namespace)
        url = 'https://{0}:{1}{2}'.format(self.kubernetes_service_host,
                                          self.kubernetes_service_port,
                                          jobs_path)
        try:
            r = requests.post(url,
                              json=definition,
                              verify=self.cert_verify,
                              headers=self.auth_header)
            if r.status_code not in [200, 201, 202]:
                raise Exception(f'Error creating job {name} - {str(r.status_code)}\n{str(r.content)}')
        except Exception as e:
            logger.error(e)

    def delete_job(self, name, namespace):
        jobs_path = self.jobs_path.format(namespace)
        url = 'https://{0}:{1}{2}/{3}'.format(self.kubernetes_service_host,
                                              self.kubernetes_service_port,
                                              jobs_path,
                                              name)
        params = {'propagationPolicy': 'Background'}
        try:
            r = requests.delete(url,
                                verify=self.cert_verify,
                                headers=self.auth_header,
                                params=params)
            if r.status_code not in [200, 202]:
                raise Exception(f'Error deleting {name} - {str(r.status_code)}\n{str(r.content)}')
        except Exception as e:
            logger.error(e)

    def wait_job(self, name, namespace, delete=False, sleep=5):
        jobs_path = self.jobs_path.format(namespace)
        url = 'https://{0}:{1}{2}/{3}'.format(self.kubernetes_service_host,
                                              self.kubernetes_service_port,
                                              jobs_path,
                                              name)
        while True:
            try:
                r = requests.get(url,
                                 verify=self.cert_verify,
                                 headers=self.auth_header)
                if r.status_code != 200:
                    raise Exception(f'Error obtaining {name} info - {str(r.status_code)}\n{str(r.content)}')
                job = r.json()
                if (utils.is_value_in_dict(job['status'], 'succeeded') and
                        utils.is_value_in_dict(job['spec'], 'completions')):
                    if job['status']['succeeded'] >= job['spec']['completions']:
                        # Delete succeeded jobs if delete=True
                        if delete:
                            self.delete_job(name, namespace)
                        break
                if (utils.is_value_in_dict(job['status'], 'failed') and
                        utils.is_value_in_dict(job['spec'], 'backoffLimit')):
                    if job['status']['failed'] >= job['spec']['backoffLimit']:
                        logger.error(f'{name} failed! See pod logs for details')
                        break
                time.sleep(sleep)
            except Exception as e:
                logger.error(e)
                break

    def create_deployment(self, definition, name, namespace):
        deployments_path = self.deployments_path.format(namespace)
        url = 'https://{0}:{1}{2}'.format(self.kubernetes_service_host,
                                          self.kubernetes_service_port,
                                          deployments_path)
        try:
            r = requests.post(url,
                              json=definition,
                              verify=self.cert_verify,
                              headers=self.auth_header)
            if r.status_code not in [200, 201, 202]:
                raise Exception(f'Error creating deployment {name} - {str(r.status_code)}\n{str(r.content)}')
        except Exception as e:
            logger.error(e)

    def delete_deployment(self, name, namespace):
        deployments_path = self.deployments_path.format(namespace)
        url = 'https://{0}:{1}{2}/{3}'.format(self.kubernetes_service_host,
                                              self.kubernetes_service_port,
                                              deployments_path,
                                              name)
        try:
            r = requests.delete(url,
                                verify=self.cert_verify,
                                headers=self.auth_header)
            if r.status_code not in [200, 202]:
                raise Exception(f'Error deleting {name} - {str(r.status_code)}\n{str(r.content)}')
        except Exception as e:
            logger.error(e)

    def get_deployment_envvars(self, name, namespace):
        deployments_path = self.deployments_path.format(namespace)
        url = 'https://{0}:{1}{2}/{3}'.format(self.kubernetes_service_host,
                                              self.kubernetes_service_port,
                                              deployments_path,
                                              name)
        try:
            r = requests.get(url,
                             verify=self.cert_verify,
                             headers=self.auth_header)
            if r.status_code != 200:
                raise Exception(f'Error reading deployment {name} - {str(r.status_code)}\n{str(r.content)}')
            deploy = r.json()
            if len(deploy['spec']['template']['spec']['containers']) > 1:
                logger.warning('The function have more than one container. Getting environment variables from container 0')
            container_info = deploy['spec']['template']['spec']['containers'][0]
            envvars = container_info['env'] if 'env' in container_info else []
            return envvars
        except Exception as e:
            logger.error(e)
            return []
