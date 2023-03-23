# Simple in java with COMPSs and OSCAR

The source code can be found in [GitHub](https://github.com/bsc-wdc/tutorial_apps/tree/stable/python/wordcount).
This example works by introducing a number and returning the number increased by one and module 255.

This Java example program requires a `.jar` file generated with the maven command: `mvn clean package`.
The input file is a flat file that contains a number, and it will be read directly as the example command shows:

``` bash
runcompss  --classpath=/opt/simple.jar simple.Simple `cat $INPUT_FILE_PATH` > $OUTPUT_FILE
```
