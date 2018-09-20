# OSCAR - On-premises Serverless Container-aware ARchitectures

[![License](https://img.shields.io/badge/license-Apache%202-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)

## Deploy
- In order to deploy the Kubernetes cluster with all OSCAR components you have to use [ec3](https://github.com/grycap/ec3) (Installation details can be found [here](https://ec3.readthedocs.io/en/latest/intro.html#installation)).
	```
	git clone https://github.com/grycap/ec3
	```
- Download the template into the ec3/templates folder:
	```
	cd ec3/
	wget -P templates https://raw.githubusercontent.com/grycap/oscar/master/templates/kubernetes_oscar.radl
	```
- Make an [auth.txt](https://ec3.readthedocs.io/en/devel/ec3.html#authorization-file) file with the credentials of your cloud provider.
- Deploy the cluster! Example with Amazon EC2:
	```
	./ec3 launch mycluster kubernetes_oscar ubuntu-ec2 -a auth.txt 
	```