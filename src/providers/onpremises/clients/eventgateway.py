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
import requests
import json

class EventGatewayClient():
    
    space_name = 'oscar'
    event_type_path = 'v1/spaces/{0}/eventtypes'.format(space_name)
    func_reg_path = 'v1/spaces/{0}/functions'.format(space_name)
    subscription_path = 'v1/spaces/{0}/subscriptions'.format(space_name)

    def __init__(self, function_args):
        self.function_name = function_args['name']
        self.config_endpoint = utils.get_environment_variable("EVENTGATEWAY_CONFIG_ENDPOINT")
        self.events_endpoint = utils.get_environment_variable("EVENTGATEWAY_EVENTS_ENDPOINT")
        self.openfaas_endpoint = utils.get_environment_variable("OPENFAAS_ENDPOINT")
        self.subscription_id = ""
            
    def is_http_eventype(self):
        r = requests.get("{0}/{1}".format(self.config_endpoint, self.event_type_path))
        j = json.loads(r.text)
        if 'eventTypes' in j:
            for event_type in j['eventTypes']:
                if 'name' in event_type and event_type['name'] == 'http':
                    return True
        return False
    
    def create_http_eventype(self):
        event_def = { "name": "http" }
        return requests.post("{0}/{1}".format(self.config_endpoint, self.event_type_path),
                          json=event_def)

    def get_register_function_json(self):
        return {"functionId": self.function_name,
                "type": "http",
                "provider": { "url": "{0}/function/{1}".format(self.openfaas_endpoint, self.function_name) }
                }        

    def register_function(self):
        if not self.is_http_eventype():
            self.create_http_eventype()        
        
        return requests.post("{0}/{1}".format(self.config_endpoint,self.func_reg_path),
                             json=self.get_register_function_json())
        
    def deregister_function(self):
        return requests.delete("{0}/{1}/{2}".format(self.config_endpoint,
                                                    self.func_reg_path,
                                                    self.function_name))
        
    def get_event_subscription_json(self):
        return {"functionId": self.function_name,
                "type": "sync",
                "eventType": "http",
                "method": "POST",
                "path": "/{0}".format(self.function_name) } 
        
    def subscribe_event(self):
        response = requests.post("{0}/{1}".format(self.config_endpoint, self.subscription_path),
                                 json=self.get_event_subscription_json())
        event_info = json.loads(response.text)
        self.subscription_id = event_info['subscriptionId']
        return response
    
    def unsubscribe_event(self, subscription_id):
        return requests.delete("{0}/{1}/{2}".format(self.config_endpoint,
                                                  self.subscription_path,
                                                  subscription_id))
        
    def send_event(self, json_body):
        return requests.post("{0}/{1}".format(self.events_endpoint, self.function_name), json=json_body)

