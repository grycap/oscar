# Deployment with the IM Dashboard

An OSCAR cluster can be easily deployed on multiple Cloud back-ends without
requiring any installation by using the
[Infrastructure Manager](https://www.grycap.upv.es/im)'s
Dashboard
([IM Dashboard](https://appsgrycap.i3m.upv.es:31443/im-dashboard/login)). This
is a managed service provided by the [GRyCAP](https://www.grycap.upv.es)
research group at the [Universitat Politècnica de València](https://www.upv.es)
to deploy customized virtual infrastructures across many Cloud providers.

Using the IM Dashboard is the easiest and most convenient approach to deploy
an OSCAR cluster. It also automatically allocates a DNS entry and TLS
certificates to support HTTPS-based access to the OSCAR cluster and companion
services (e.g. MinIO).

This example shows how to deploy an OSCAR cluster on
[Amazon Web Services (AWS)](https://aws.amazon.com) with two nodes. Thanks to
the IM, the very same procedure applies to deploy the OSCAR cluster in an
on-premises Cloud (such as OpenStack) or any other Cloud provider supported
by the IM.

These are the steps:

### 1. Access the [IM Dashboard](https://appsgrycap.i3m.upv.es:31443/im-dashboard/login)
![login](images/im-dashboard/im-dashboard-00.png)

You will need to authenticate via [EGI Check-In](https://www.egi.eu/services/check-in/), which supports mutiple Identity Providers (IdP).

### 2. Configure the Cloud Credentials

Once logged in, you need to define the access credentials to the Cloud on which the OSCAR cluster will be deployed. These should be temporary credentials under the [principle of least privilege (PoLP)](https://searchsecurity.techtarget.com/definition/principle-of-least-privilege-POLP).
![credentials](images/im-dashboard/im-dashboard-00-2.png)
![credentials](images/im-dashboard/im-dashboard-00-3.png)
![credentials](images/im-dashboard/im-dashboard-00-4.png)

In our case we indicate an identifier for the set of credentials, [the Access Key ID and the Secret Access Key](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html) for an [IAM](https://aws.amazon.com/iam/) user that has privileges to deploy Virtual Machines in [Amazon EC2](https://aws.amazon.com/ec2).

### 3. Select the OSCAR template
![template](images/im-dashboard/im-dashboard-01.png)

### 4. Customize and deploy the OSCAR cluster

On the first panel you can specify the number of Working Nodes (WNs) of the cluster together with the computational requirements for each node. We leave the default values.
- `Number of WNs in the oscar cluster`: number of working nodes.
- `Number of CPUs for the front-end node`: number of CPUs in the primary node.
- `Amount of Memory for the front-end node`: RAM in the primary node.
- `Flavor name of the front-end node` (only required in case of special flavors i.e. with GPUs): type of instance that will be selected in the front node.
- `Number of CPUs for the WNs`: number of CPUs per working node.
- `Amount of Memory for the WNs`: RAM per working node.
- `Flavor name of the WNs` (only required in case of special flavors i.e. with GPUs): type of instance that will be selected in the working nodes.
- `Size of the extra HD added to the instance`: extra memory in the primary node.

![template-hw](images/im-dashboard/im-dashboard-02.png)

On the next panel, specify the passwords to be employed to access the Kubernetes Web UI (Dashboard), to access the OSCAR web UI and to access the MinIO dashboard. These tokens can also be used for programmatic access to the respective services.

- `Access Token for the Kubernetes admin user`: it is the token to connect to the Dashboard of Kubernetes.
- `OSCAR password`: password to OSCAR.
- `MinIO password`: password to MinIO. 8 characters min.
- `Email to be used in the Lets Encrypt issuer`: it is an email linked with the certificates in case the user has any questions.
- `ID of the user that creates the infrastructure`: unique identifier. Do not touch.
- `VO to support`: it supports OIDC log in. If left blank, only the user who deploys the cluster can connect to in. In case there is a VO, it can be the user who deploys and all people in the VO.
- `Flag to add NVIDIA support`: if you want to use NVIDIA.
- `Flag to install Apache YuniKorn`: if you are going to use YuniKorn.

![template-param](images/im-dashboard/im-dashboard-03.png)

Finally you have to choose the Cloud provider. The ID specified when creating the Cloud credentials will be shown. You will also need to specify the [Amazon Machine Image (AMI) identifier](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html). We chose an AMI based on Ubuntu 20.04 provided by Canonical whose identifier for the us-east-1 region is: ami-09e67e426f25ce0d7

*NOTE: You should obtain the AMI identifier for the latest version of the OS. This way, security patches will be already installed. You can obtain this AMI identifier from the AWS Marketplace or the Amazon EC2 service.*

Give the infrastructure a name and press **Submit**.

![template-param](images/im-dashboard/im-dashboard-04.png)


### 5. Check the status of the deployment OSCAR cluster

You will be redirected to the **Infrastructures** view where you will see all the infrastructures you have already deployed and the OSCAR cluster in the **runing** state. It will not be fully deployed until the **configured** state is reached.

![status-general](images/im-dashboard/im-dashboard-05.png)

If you are interested in understanding what is happening under the hood, you can see the logs by clicking on the button in the right:

![status-log](images/im-dashboard/im-dashboard-06.png)

### 6. Accessing the OSCAR cluster

Once reached the **configured** state, see the **Outputs** to obtain the different endpoints:

* `console_minio_endpoint`: this endpoint brings access to the MinIO web user interface.
* `dashboard_endpoint`: this endpoint redirects to the Kubernetes dashboard where the OSCAR cluster is deployed.
* `local_oscarui_endpoint`: this endpoint is where the OSCAR backend is listening. It supports authentication only via basic-auth.
* `minio_endpoint`: endpoint where the MinIO API is listening. If you access it through a web browser, you will be redirected to `console_minio_endpoint`.
* `oscarui_endpoint`: public endpoint of the OSCAR web user interface. It supports OIDC connections via EGI Check-in, as well as basic auth.

    ![outputs](images/im-dashboard/im-dashboard-07.png)

The OSCAR UI can be accessed with the username ``oscar`` and the password you specified at deployment time.

![access-oscar](images/im-dashboard/im-dashboard-08.png)

The MinIO UI can be accessed with the username ``minio`` and the password you specified at deployment time.

![access-minio](images/im-dashboard/im-dashboard-09.png)

The Kubernetes Dashboard can be accessed with the token you specified at deployment time where you can obtain statistics about the Kubernetes cluster:.
![access-kubernetes](images/im-dashboard/im-dashboard-10.png)

![access-kubernetes-2](images/im-dashboard/im-dashboard-11.png)

### 7. Terminating the OSCAR cluster

You can terminate the OSCAR cluster from the IM Dashboard by clicking on the button dropdown and selecting the **Delete** option:
![terminate](images/im-dashboard/im-dashboard-12.png)
