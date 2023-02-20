# COMPSS with OSCAR

## Dockerfile

The first step is the creation of the Docker image
The Docker image has to start from an image allocated in the [compss profile at dockerhub](https://hub.docker.com/u/compss)
with all the dependencies

``` Docker
FROM compss/compss:{version}
```

Copy the program inside the Docker container:

- C/C++ Applications should be the entire project folder (the compilation will create the binary in execution time) compss_build_app
- Java Application, the `.jar` file should be copy
- In Python Applications, introduce all the code

## Script

Create the name of the output file and save it into the `$OUTPUT_FILE` variable.

``` bash
FILE_NAME=`basename "$INPUT_FILE_PATH" | cut -f 1 -d '.'`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME.txt"
```

Some services receive a compressed file that needs to be uncompressed, while others require parsing the input file. Once the input is parsed, the ssh server needs to be initialized.

``` bash
/etc/init.d/ssh start
```

Finally, run COMPSs. Select the command more appropriate according to your language program.
C programs need to be built first with `compss_build_app increment`.
Redirect the output to `$OUTPUT_FILE`.

``` bash
runcompss --pythonpath=$(pwd) --python_interpreter=python3 {path_to_the_python_program.py} {input_variables}  > $OUTPUT_FILE
runcompss  --classpath={path_to_the_jar.jar} {in_java_folder/MainClass} {input_variables} > $OUTPUT_FILE
runcompss --lang=c --project=./xml/templates/project.xml  master/{name_program} {input_variables} > $OUTPUT_FILE
```

### Output warning redirect

When COMPSs starts the execution, a warning message will appear in the logs.
This message can be ignored. `WARNING: COMPSs Properties file is null. Setting default values`
