import boto3
import os
import uuid
import sys
import json
import tempfile
import subprocess

os_tmp_folder = tempfile.gettempdir()
output_folder = tempfile.gettempdir() + "/output"

def is_s3_event(event):
    if is_key_and_value_in_dictionary('Records', event):
        record = event['Records'][0]
        if is_key_and_value_in_dictionary('s3', record):
            print("Is S3 event")
            return True

def get_s3_record(event):
    return event['Records'][0]['s3']

def download_s3_file(event):
    '''Downloads the file from the S3 bucket and returns the path were the download is placed'''
    s3_info = get_s3_record(event)
    bucket = s3_info['bucket']['name']
    key = s3_info['object']['key']
    file_download_path = "{0}/{1}".format(os_tmp_folder, get_temp_file()) 
    print("Downloading item from bucket '{0}' with key '{1}'".format(bucket, key))
    if not os.path.isdir(os_tmp_folder):
        os.makedirs(os.path.dirname(file_download_path), exist_ok=True)
    with open(file_download_path, 'wb') as data:
        boto3.client('s3').download_fileobj(bucket, key, data)
    print("Successful download of file '{0}' from bucket '{1}' in path '{2}'".format(key, bucket, file_download_path))
    return file_download_path

def upload_output():
    output_files_path = get_all_files_in_directory(output_folder)
    output_bucket = os.environ['OUTPUT_BUCKET']
    output_bucket_folder = os.environ['OUTPUT_FOLDER']
    print("UPLOADING FILES {0}".format(output_files_path))
    for file_path in output_files_path:
        file_name = file_path.replace("{0}/".format(output_folder), "")
        file_key = "{0}/{1}".format(output_bucket_folder, file_name)
        upload_file(output_bucket, file_path, file_key)

def get_all_files_in_directory(dir_path):
    files = []
    for dirname, _, filenames in os.walk(dir_path):
        for filename in filenames:
            files.append(os.path.join(dirname, filename))
    return files
        
def upload_file(bucket_name, file_path, file_key):
    print("Uploading file  '{0}' to bucket '{1}'".format(file_key, bucket_name))
    with open(file_path, 'rb') as data:
        boto3.client('s3').upload_fileobj(data, bucket_name, file_key)
    print("Changing ACLs for public-read for object in bucket {0} with key {1}".format(bucket_name, file_key))
    obj = boto3.resource('s3').Object(bucket_name, file_key)
    obj.Acl().put(ACL='public-read')
    
def get_temp_file():
    return str(uuid.uuid4().hex)
    
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
    f_input = json.loads(get_stdin())
    print("Received input: {0}".format(f_input))
    if is_s3_event(f_input):
        os.environ['SCAR_INPUT_FILE'] = download_s3_file(f_input)
        print('SCAR_INPUT_FILE: {0}'.format(os.environ['SCAR_INPUT_FILE']))
        if is_key_and_value_in_dictionary('sprocess', os.environ):
            os.makedirs(output_folder, exist_ok=True)
            launch_user_script()
            upload_output()
        
    
