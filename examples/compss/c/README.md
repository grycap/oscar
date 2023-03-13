# Increment in C with COMPSS and OSCAR

The source code can be found in [GitHub](https://github.com/bsc-wdc/tutorial_apps/tree/stable/c/increment).
This example works by introducing four numbers: the first is the number of times to increase the counters the other three are the counters.
The return will be the three counters incremented by the first number.

The first step in this example is to parse the input file and set the arguments:

```bash
file=$(cat $INPUT_FILE_PATH)
incrementNumber=$(echo "$file" | cut -f 1 -d ';')
counter1=$(echo "$file" | cut -f 2 -d ';')
counter2=$(echo "$file" | cut -f 3 -d ';')
counter3=$(echo "$file" | cut -f 4 -d ';')
```

In C language, it is necessary to change the directory and compile the project in a run time:

``` bash
cd /opt/increment
compss_build_app increment
```

The compilation process will create a folder with the name `master`.
This folder contains the binary program:

``` bash
runcompss --lang=c --project=./xml/templates/project.xml  master/increment $incrementNumber $counter1 $counter2  $counter3 > $OUTPUT_FILE
```
