# FDL Composer

OSCAR Services can be aggregated into data-driven workflows where the output data of one service is stored in the object store that triggers another service, potentially in a different OSCAR cluster. This allows to execute the different phases of the workflow in disparate computing infrastructures.


However, writing an entire workflow in an [FDL](fdl.md) file can be a difficult task for some users.


To simplify the process you can use
[FDL Composer](http://composer.oscar.grycap.net), a web-based application
to facilitate the definition of [FDL](https://docs.oscar.grycap.net/fdl/) YAML
files for [OSCAR](https://oscar.grycap.net/) and
[SCAR](https://scar.readthedocs.io).

![fdl-composer-workflow.png](images/fdl-composer/fdl-composer-workflow.png)

## How to access FDL Composer


Just access [FDL Composer](https://composer.oscar.grycap.net/) which is a Single Page Application (SPA) running entirely in your browser. If you prefer to
execute it on your computer instead of using the web, clone the git repository
by using the following command:

``` sh
git clone https://github.com/grycap/fdl-composer
```

And the run the app with `npm`:

``` sh
npm start
```

## Basic elements

Workflows are composed of `OSCAR services` and `Storage providers`:

### OSCAR services

`OSCAR services` are responsible for processing the data uploaded to
`Storage providers`.

Defining a new `OSCAR service`  requires filling at least the `name`, `image`,
and `script` fields.

To define environment variables you must add them as a comma separated string of
*key=value* entries. For example,  to create a variable with the name
`firstName` and the value `John`, the "Environment variables" field should
look like `firstName=John`. If you want to assign more than one variable, for
example, `firstName` and `lastName` with the values `John` and `Keats`, the
input field should include them all separated by commas (e.g.,
`firstName=John,lastName=Keats`).

### Storage providers and buckets/folders

`Storage providers` are object storage systems  responsible for storing both
the input files to be processed by `OSCAR services` and the output files
generated as a result of the processing.

Three types of storage providers can be used in OSCAR
[FDLs](https://docs.oscar.grycap.net/fdl/): [MinIO](https://min.io),
[Amazon S3](https://aws.amazon.com/s3), and [OneData](https://onedata.org).

To configure them, drag the storage
provider from the menu to the canvas and double click on the item created. A
window with a single input will appear. Then, insert the path of the folder
name. To edit one of the storage providers, move the mouse over the item and
select the edit option.

**Remember that only MinIO can be used as input storage provider for OSCAR
services.**

### Download and load state

The defined workflow can be saved in a file using the "Download state" button.
OSCAR services, Storage Providers, and Buckets are kept in the file. The
graphic workflow can be edited later by loading it with the "Load state" button.

### Create a YAML file

 You can easily download the workflow's FDL file (in YAML) through the "Export
 YAML" button.

## Connecting components

All components have four ports: The up and left ones are input ports while the
right and down ports are used as output. `OSCAR Services` can only be
connected with `Storage providers`, always linked in the same direction
(the output of one element with the input of the other).

When two services are connected, both
will be declared in the FDL file, but they will work separately, and
there will be no workflow between them. If two storage providers are connected
between them, it will have no effect, but both storages will be declared.

## SCAR options

FDL Composer can also create FDL files for
[SCAR](https://github.com/grycap/scar). This allows to
define workflows that can be executed on the Edge or in on-premises Clouds
through OSCAR, and on the public Cloud (AWS Lambda and/or AWS Batch) through
SCAR.

## Example

There is an example of FDL Composer implementing the
[video-process](https://github.com/grycap/oscar/tree/master/examples/video-process)
use case in our [blog](https://oscar.grycap.net/blog/post-oscar-fdl-composer/).
