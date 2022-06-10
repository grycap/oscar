# Functions Definition Language Composer (OSCAR)

Writing an entire workflow in plain text could be tough to make it. To simplify the process, use FDL-Composer. FDL-Composer is a tool developed by [GryCAP](https://www.grycap.upv.es/) that creates YAML files for [OSCAR](https://oscar.grycap.net/) and [SCAR](https://scar.readthedocs.io) platforms from a graphic design workflow.

## How to access

It does not require installation, just access to [fdl-composer web](https://composer.oscar.grycap.net/). If you would prefer to execute on your computer instead of using the web. Clone the repository GitHub:

``` sh
git clone https://github.com/grycap/fdl-composer
```

Run the app with npm:

``` sh
npm start
```

## Basic Elements

### OSCAR services

It requires filling at least the name, image, and script fields inputs. Using variable field input to assign a value, use "=" and separate the different variable names using ",". In case we want to create a variable with the name "firstName" and add the value "John" the input field should fill with "firstName=John". If it should be declared more than one variable, for example, firstName and lastName with the values John and Keats, the input field should fill with "firstName=John,lastName=Keats".

### Storage Provider and Buckets

Three types of storage providers (MinIo, S3, and One Data) can be used in OSCAR. At first, they must be configured. Drag the storage provider from the menu to the canvas. Double click on the item created, and a window with a single input will show up. Then introduce the path of the folder name. To edit one of the storage providers, move the mouse over the item and select the edit option.

### Download and Load state

The graphic workflow can be saved in a file using the Download state Button. The OSCAR services, Storage Providers, and Buckets are kept in the file. The graphic workflow can be changed later by loading the file content with the Load state Button.

### Create a YAML file

Export Yaml Button download a YAML file. In case of an error, a window will pop up, warning about that.

## Connect components

All components have four ports, up and left input ports and right and down ports as output. When a service is connected with storage will be linked in the same direction as the port service. So if both inputs connect storage and service, that storage folder will be the input of the service. When two services are connected between them, both will declare in YAML the file, but they will work separately, and there will be no workflow between them. If two storage are connected between themself, it will not have any effect. Both storages will be declared.

Edited storage providers could have problems with the same storage nodes printed in the canvas. So delete them and drag the storage nodes again.

## SCAR Options

FDL-Composer can create a YAML file for the SCAR tool. It can make a workflow that can execute on Edge Computing, Private Cloud, AWS Lambda Function, and AWS Batch Jobs.

## Example

There is an example of fdl-composer implementing the [video-process](https://github.com/grycap/oscar/tree/master/examples/video-process) use case in our [blog](https://oscar.grycap.net/blog/post-oscar-fdl-composer/).
