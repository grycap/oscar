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

import boto3
import os
import uuid
import sys
import json
import tempfile
import subprocess
from urllib.parse import unquote_plus

os_tmp_folder = tempfile.gettempdir() + "/" + str(uuid.uuid4().hex)
output_folder = os_tmp_folder + "/output"

def is_s3_event(event):
    if is_key_and_value_in_dictionary('data', event) \
    and is_key_and_value_in_dictionary('body', event['data']) \
    and is_key_and_value_in_dictionary('Records', event['data']['body']):
        record = event['data']['body']['Records'][0]
        if is_key_and_value_in_dictionary('s3', record):
            print("Is S3 event")
            return True
    else:
        print("Not a S3 event")

def get_s3_record(event):
    return event['data']['body']['Records'][0]['s3']

def download_s3_file(event):
    '''Downloads the file from the S3 bucket and returns the path were the download is placed'''
    s3_info = get_s3_record(event)
    bucket = s3_info['bucket']['name']
    key = unquote_plus(s3_info['object']['key'])
    file_name = os.path.splitext(key)[0]
    file_download_path = "{0}/{1}".format(os_tmp_folder, file_name) 
    print("Downloading item from bucket '{0}' with key '{1}'".format(bucket, key))
    if not os.path.isdir(os_tmp_folder):
        os.makedirs(os.path.dirname(file_download_path), exist_ok=True)
    with open(file_download_path, 'wb') as data:
        boto3.client('s3',
                     endpoint_url='http://minio-service.minio:9000',
                     aws_access_key_id=os.environ['AWS_ACCESS_KEY_ID'],
                     aws_secret_access_key=os.environ['AWS_SECRET_ACCESS_KEY']
        ).download_fileobj(bucket, key, data)
    print("Successful download of file '{0}' from bucket '{1}' in path '{2}'".format(key, bucket, file_download_path))
    return file_download_path

def upload_output():
    output_files_path = get_all_files_in_directory(output_folder)
    output_bucket = os.environ['OUTPUT_BUCKET']
    print("UPLOADING FILES {0}".format(output_files_path))
    for file_path in output_files_path:
        file_name = file_path.replace("{0}/".format(output_folder), "")
        output_file_name = "{0}-out{1}".format(os.path.splitext(file_name)[0],''.join(os.path.splitext(file_name)[1:]))
        upload_file(output_bucket, file_path, output_file_name)

def get_all_files_in_directory(dir_path):
    files = []
    for dirname, _, filenames in os.walk(dir_path):
        for filename in filenames:
            files.append(os.path.join(dirname, filename))
    return files
        
def upload_file(bucket_name, file_path, file_key):
    print("Uploading file  '{0}' to bucket '{1}'".format(file_key, bucket_name))
    with open(file_path, 'rb') as data:
        boto3.client('s3',
                     endpoint_url='http://minio-service.minio:9000',
                     aws_access_key_id=os.environ['AWS_ACCESS_KEY_ID'],
                     aws_secret_access_key=os.environ['AWS_SECRET_ACCESS_KEY']
        ).upload_fileobj(data, bucket_name, file_key)
    # print("Changing ACLs for public-read for object in bucket {0} with key {1}".format(bucket_name, file_key))
    # obj = boto3.resource('s3').Object(bucket_name, file_key)
    # obj.Acl().put(ACL='public-read')

def is_key_and_value_in_dictionary(key, dictionary):
    return (key in dictionary) and dictionary[key] and dictionary[key] != ""    

def get_stdin():
    buf = ""
    for line in sys.stdin:
        buf = buf + line
    return buf

def launch_user_script():
    print("Executing user_script.sh")
    print(subprocess.call(os.environ['sprocess'], stderr=subprocess.STDOUT))

if(__name__ == "__main__"):
    if 'OSCAR_EVENT' in os.environ:
        f_input = json.loads(os.environ['OSCAR_EVENT'])
    else:
        f_input = json.loads(get_stdin())
    print("Received input: {0}".format(f_input))
    if is_s3_event(f_input):
        os.environ['SCAR_INPUT_FILE'] = download_s3_file(f_input)
        os.environ['SCAR_OUTPUT_FOLDER'] = output_folder
        print('SCAR_INPUT_FILE: {0}'.format(os.environ['SCAR_INPUT_FILE']))
    if is_key_and_value_in_dictionary('sprocess', os.environ):
        os.makedirs(output_folder, exist_ok=True)
        launch_user_script()
    if is_s3_event(f_input):        
        upload_output()
