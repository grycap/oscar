# Functions Definition Language Composer (OSCAR)

Writing an entire workflow in plain text could be tough to make it. To simplify the process, use FDL-Composer. FDL-Composer is a tool developed by [GryCAP](https://www.grycap.upv.es/) that creates YAML files for [OSCAR](https://oscar.grycap.net/) and [SCAR](https://scar.readthedocs.io) platforms from a graphic design workflow.

## OSCAR functions

It requires filling at least the name, image, and script fields inputs. Using variable field input to assign a value, use "=" and separate the different variable names using ",". In case we want to create a variable with the name "firstName" and add the value "John" the input field should fill with "firstName=John". In the case it should be declared more than one variable, for example, firstName and lastName with the values John and Keats, the input field should fill with "firstName=John,lastName=Keats".

## Storage Provider and Buckets

Three types of storage providers (MinIo, S3, and One Data) can be used in OSCAR. At first, they must be configured. Drag the storage provider from the menu to the canvas. Double click on the item created, and a window with a single input will show up. Then introduce the path of the folder name. Make the connection between buckets and OSCAR function nodes. To edit one of the storage providers, move the mouse over the item and select the edit option. Delete the nodes in the canvas and drag the storage nodes again.

## Download and Load state

The graphic workflow can be saved in a file using the Download state Button. The OSCAR functions, Storage Provider, and Buckets are kept in the file. The graphic workflow can be changed by loading the file content with the Load state Button.

## Create a YAML file

Export Yaml Button download a YAML file. In case of an error, a window will pop up, warning about that.

## SCAR Options

FDL-Composer can create a YAML file for the SCAR tool. It can make a workflow that can execute on Edge Computing, Private Cloud, AWS Lambda Function, and AWS Batch Jobs.
