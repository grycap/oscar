# Wordcount in python with COMPSs and OSCAR

The source code can be found in [github](https://github.com/bsc-wdc/tutorial_apps/tree/stable/python/wordcount)
This example works by introducing a folder name, and it will read the files inside the folder and count the words.

The input file that will be put in a MinIO bucket is a `tar` file that contains all the files.

A has been created a folder inside the Docker `/opt/folder`
In this auxiliary folder will be extracted all the input files
and it will be an argument to run COMPSs.

**This example does not support zip files, just tar**

The complete command to run the COMPSs is:

``` bash
runcompss --pythonpath=$(pwd) --python_interpreter=python3 /opt/wordcount_merge.py /opt/folder  > $OUTPUT_FILE
```
