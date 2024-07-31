python - << EOF
import json
import base64
import string
import random
import os
from subprocess import run, PIPE

# Read input file
FILE_PATH=os.getenv("INPUT_FILE_PATH")
if [[ "$FILE_PATH" != *.json ]]; then
	run(["mv", "$INPUT_FILE_PATH", "$INPUT_FILE_PATH.json"])
	FILE_PATH="$FILE_PATH"+".json"
fi

DEEPAAS_CLI_VERSION=[2,3,1]
STR_VERSION='2.3.1'
DEEPAAS_CLI_VCOMMAND=['deepaas-cli', '--version']

DEEPAAS_CLI_COMMAND=['deepaas-cli', 'predict']
OSCAR_FILES="oscar-files"

def check_deepaas_version():
    v = run(DEEPAAS_CLI_VCOMMAND, stdout=PIPE)
    version = v.stdout.decode("utf-8").strip('\n').split(".")
    if len(version) > 3: version.pop()
    for i, ver in enumerate(version):
        if int(ver) < DEEPAAS_CLI_VERSION[i]:
            print(f"Error: 'deepaas-cli' version must be >={STR_VERSION}. Current version is: {print_version(version)}")
            exit(1)

def print_version(version):
    return '.'.join(version)

def add_arg(key, value):
   DEEPAAS_CLI_COMMAND.append("--"+key)
   DEEPAAS_CLI_COMMAND.append(value)

def decode_b64(filename, data):
    with open(filename, "wb") as f:
        f.write(base64.b64decode(data))

def parse_files(files):
        for file in files:
                rnd_str = ''.join(random.choice(string.ascii_lowercase) for i in range(5))
                filename=''.join(["tmp-file-", rnd_str, ".", file["file_format"]])
                add_arg(file["key"], filename)
                decode_b64(filename, file["data"])

# Check the deepaas-cli version
check_deepaas_version()

# Process input
with open(FILE_PATH, "r") as f:
 params = json.loads(f.read())

for k, v in params.items():
    # If param is 'oscar-files' decode the array of files
    if k == "oscar-files":
       parse_files(v)
    else:
        if isinstance(v, int): 
            v = str(v)
        add_arg(k, v)
run(DEEPAAS_CLI_COMMAND)

EOF
