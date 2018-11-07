Deploy
======

In order to deploy the Kubernetes cluster with all the OSCAR components use `EC3 <https://github.com/grycap/ec3>`_, a tool that deploys elastic virtual clusters. EC3 uses the `Infrastructure Manager (IM) <https://www.grycap.upv.es/im>`_ to deploy such clusters on multiple Cloud back-ends.
The installation details can be found `here <https://ec3.readthedocs.io/en/latest/intro.html#installation>`_), though this section includes the relevant information to get you started:



1. Clone the EC3 repository:
::

  git clone https://github.com/grycap/ec3

2. Download the OSCAR template into the ``ec3/templates`` folder:
::

  cd ec3
  wget -P templates https://raw.githubusercontent.com/grycap/oscar/master/templates/kubernetes_oscar.radl

3. Create an ``auth.txt`` `authorization file <https://ec3.readthedocs.io/en/devel/ec3.html#authorization-file>`_ with valid credentials to access your Cloud provider.  
As an example, to deploy on an OpenNebula-based Cloud site the contents of the file would be:
::

  type = OpenNebula; host = opennebula-host:2633; username = your-user; password = you-password

4. Modify the corresponding `RADL <https://imdocs.readthedocs.io/en/latest/radl.html#resource-and-application-description-language-radl>`_ template in order to determine the appropriate configuration for your deployment:

  * Virtual Machine Image identifiers 
  * Hardware Configuration

 As an example, to deploy in OpenNebula, one would modify the `ubuntu-opennebula.radl` (or create a new one).

5. Deploy the cluster:
::

  ./ec3 launch oscar-cluster kubernetes_oscar ubuntu-opennebula -a auth.txt 

This will take several minutes until the Kubernetes cluster and all the required services have been deployed.
You will obtain the IP of the front-end of the cluster and a confirmation message that the front-end is ready.
Notice that it will still take few minutes before the services in the Kubernetes cluster are up & running.

6. Check the cluster state.

The cluster will be fully configured when all the Kubernetes pods are in the `Running` state.
:: 

 ./ec3 ssh oscar-cluster
 sudo kubectl get pods --all-namespaces 

Notice that initially only the front-end node of the cluster is deployed. 
As soon as the OSCAR framework is deployed, together with its services, the CLUES elasticity manager powers on a new (working) node on which these services will be run.

You can see the status of the provisioned node(s) by issuing:
::

 clues status


which obtains:
::

  node                          state    enabled   time stable   (cpu,mem) used   (cpu,mem) total
  -----------------------------------------------------------------------------------------------
  wn1.localdomain                used    enabled     00h00'49"    0.0,825229312      1,1992404992
  wn2.localdomain                 off    enabled     00h06'43"      0,0              1,1073741824
  wn3.localdomain                 off    enabled     00h06'43"      0,0              1,1073741824
  wn4.localdomain                 off    enabled     00h06'43"      0,0              1,1073741824
  wn5.localdomain                 off    enabled     00h06'43"      0,0              1,1073741824

The working nodes transition from `off` to `pwon` and, finally, to the `used` status. 