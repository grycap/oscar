## How to create the supervisor binary

To create the supervisor binary you will need the supervisor library dependencies (python3 and boto3) and the pyinstaller binary.

To ease the process you can use a docker container with all the required packages:

You only have to execute:

```sh
docker run --rm grycap/jenkins:pyinstaller https://raw.githubusercontent.com/grycap/oscar/master/src/providers/onpremises/cloud/supervisor.py > supervisor
```

And the container will create the binary in your current folder