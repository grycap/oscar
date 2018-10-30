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

# MISSING DELETE FUNCTIONS

class EventGatewayClient():
    
    event_type_path = '/v1/spaces/default/eventtypes'
    func_reg_path = '/v1/spaces/default/functions'
    event_subs_path = '/v1/spaces/default/subscriptions'

    def __init__(self):
        self.config_endpoint = utils.get_environment_variable("EVENTGATEWAY_CONFIG_ENDPOINT")
        self.events_endpoint = utils.get_environment_variable("EVENTGATEWAY_EVENTS_ENDPOINT")
        self.openfaas_endpoint = utils.get_environment_variable("OPENFAAS_ENDPOINT")
        
        if not self.is_http_eventype():
            self.create_http_eventype()
            
    def is_http_eventype(self):
        r = requests.get(self.config_endpoint + self.event_type_path)
        j = json.loads(r.text)
        if 'eventTypes' in j:
            for event_type in j['eventTypes']:
                if 'name' in event_type and event_type['name'] == 'http':
                    return True
        return False
    
    def create_http_eventype(self):
        event_def = { "name": "http" }
        r = requests.post(self.config_endpoint + self.event_type_path, json=event_def)
        print(r.text)

    def register_function(self, function_name):
        func_def = {"functionId": function_name,
                    "type": "http",
                    "provider": { 
                        "url": "{0}/function/{1}".format(self.openfaas_endpoint, function_name) 
                    }
                   }
        r = requests.post(self.config_endpoint + self.func_reg_path, json=func_def)
        print(r.text)
        
    def subscribe_event(self, function_name):
        event_sub = {"functionId": function_name, 
                     "type": "sync",
                     "eventType": "http",
                     "method": "POST",
                     "path": "/{0}".format(function_name) }        
        r = requests.post(self.config_endpoint + self.event_subs_path, json=event_sub)
        print(r.text)
        j = json.loads(r.text)
        return j['subscriptionId']
        
    def send_event(self, function_name, json_body):
        return requests.post(self.events_endpoint + "/{0}".format(function_name), json=json_body)

