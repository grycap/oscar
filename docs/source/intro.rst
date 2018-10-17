Deploy
======

In order to deploy the Kubernetes cluster with all OSCAR components you have to use `ec3 <https://github.com/grycap/ec3>`_ (Installation details can be found `here <https://ec3.readthedocs.io/en/latest/intro.html#installation>`_).
  git clone https://github.com/grycap/ec3

Download the template into the ec3/templates folder:
  cd ec3/
  wget -P templates https://raw.githubusercontent.com/grycap/oscar/master/templates/kubernetes_oscar.radl

Make an `auth.txt <https://ec3.readthedocs.io/en/devel/ec3.html#authorization-file>`_ file with the credentials of your cloud provider.

Deploy the cluster! Example with Amazon EC2:
  ./ec3 launch mycluster kubernetes_oscar ubuntu-ec2 -a auth.txt 

