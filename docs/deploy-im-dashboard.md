# Deployment with IM

An OSCAR cluster can be easily deployed on multiple Cloud platforms via the
[Infrastructure Manager](https://www.grycap.upv.es/im)'s
Dashboard
([IM Dashboard](https://im.egi.eu)). This
is a managed service provided by [EGI](https://www.egi.eu) and operated by the [GRyCAP](https://www.grycap.upv.es)
research group at the [Universitat Politècnica de València](https://www.upv.es)
to deploy customized virtual infrastructures across many Cloud providers.

Using the IM Dashboard is the easiest and most convenient approach to deploy
an OSCAR cluster. It also automatically allocates a DNS entry and TLS
certificates to support HTTPS-based access to the OSCAR cluster and companion
services (e.g. MinIO).

This example shows how to deploy an OSCAR cluster on
[Amazon Web Services (AWS)](https://aws.amazon.com) with two nodes. Thanks to
the IM, the very same procedure allows to deploy the OSCAR cluster in an
on-premises Cloud (such as OpenStack) or any other Cloud provider supported
by the IM.

These are the steps:

1. Access the [IM Dashboard](https://im.egi.eu)

    ![login](images/im-dashboard/im-dashboard-00.png)

    You will need to authenticate via
    [EGI Check-In](https://www.egi.eu/services/check-in/), which supports
    mutiple Identity Providers (IdP). There is no need to register and the service is provided free of charge.

1. Configure the Cloud Credentials

    Once logged in, you need to define the access credentials to the Cloud on
    which the OSCAR cluster will be deployed. These should be temporary
    credentials under the
    [principle of least privilege (PoLP)](https://searchsecurity.techtarget.com/definition/principle-of-least-privilege-POLP).

    ![credentials](images/im-dashboard/im-dashboard-00-2.png)

    ![credentials](images/im-dashboard/im-dashboard-00-3.png)

    ![credentials](images/im-dashboard/im-dashboard-00-4.png)

    In our case, we indicate an identifier for the set of credentials,
    [the Access Key ID and the Secret Access Key](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html)
    for an [IAM](https://aws.amazon.com/iam/) user that has privileges to
    deploy Virtual Machines in [Amazon EC2](https://aws.amazon.com/ec2). With the default values indicated in this tutorial, you will need privileges to deploy the following instance types: ```t3a.xlarge``` for the front-end node and ```t3a.medium``` for the working node.

1. Select the OSCAR template

    ![template](images/im-dashboard/im-dashboard-01.png)
    

   There are optional features than can be included in the OSCAR cluster to fit particular user needs. We'll skip them.

   ![template-config](images/im-dashboard/im-dashboard-01-2.png)

    

1. Customize and deploy the OSCAR cluster

    In this panel you can specify the number of Working Nodes (WNs) of the
    cluster together with the computational requirements for each node. We
    leave the default values.

    - Number of WNs in the OSCAR cluster.
    - Number of CPUs for the front-end node.
    - Amount of Memory (RAM) for the front-end node
    - Flavor name of the front-end node. This is only required in case of special flavors (i.e. with GPUs): Instance type that will be selected for the front-end node.
    - Number of CPUs for the WNs (Working Nodes).
    - Amount of Memory (RAM) for the WNs.
    - Flavor name of the WNs. Again, this is only required in case of special flavors
    - Size of the extra HD (Hard Disk) added to the node.
    ![template-hw](images/im-dashboard/im-dashboard-02.png)

    In the following panel, specify the passwords to be employed to access the
    Kubernetes Web UI (Dashboard), to access OSCAR and to access
    the MinIO dashboard. These passwords/tokens can also be used for programmatic access
    to the respective services.

    - Access Token for the Kubernetes admin user: Used to connect to the Kubernetes Dashboard.
    - OSCAR password: To log in to the OSCAR cluster as an admin user.
    - MinIO password (8 characters min.).
    - Email to be used in the Let's Encrypt issuer.
    - ID of the user that creates the infrastructure.
    - VO (Virtual Organization) to support: It supports OIDC (OpenID Connect) log in. If empty, only the user who deploys the cluster can log in. If a VO is specified, all the members of the VO can log in the OSCAR cluster.
    - Flag to add NVIDIA support: if you want to use NVIDIA.
    - Flag to install Apache YuniKorn: if you are going to use YuniKorn.
    ![template-param](images/im-dashboard/im-dashboard-03.png)

    Now, choose the Cloud provider. The ID specified when creating the Cloud
    credentials will be shown.
    You will also need to specify the
    [Amazon Machine Image (AMI) identifier](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html).
    We chose an AMI based on Ubuntu 20.04 provided by Canonical whose
    identifier for the us-east-1 region is: ami-09e67e426f25ce0d7

    NOTE: You should obtain the AMI identifier for the latest version of the
    OS. This way, security patches will be already installed. You can obtain
    this AMI identifier from the AWS Marketplace or the Amazon EC2 service.

    ![template-param](images/im-dashboard/im-dashboard-04.png)

    Give the infrastructure a name and press "Submit".

1. Check the status of the deployment OSCAR cluster

    You will see that the OSCAR cluster is being deployed and the
    infrastructure reaches the status "running". The process will finish
    when it reaches the state "configured".

    ![status-general](images/im-dashboard/im-dashboard-05.png)

    If you are interested in understanding what is happening under the hood
    you can see the logs:

    ![status-log](images/im-dashboard/im-dashboard-06.png)

1. Accessing the OSCAR cluster

    Once reached the "configured" state, see the "Outputs" to obtain the
    different endpoints:

    * console_minio_endpoint: To access the MinIO web UI.
    * dashboard_endpoint: To access the Kubernetes dashboard.
    * local_oscarui_endpoint: To access the OSCAR Dashboard. It supports username/password authentication.
    * minio_endpoint: Endpoint where the MinIO API is listening. If you
        access it through a web browser, you will be redirected to
        "console_minio_endpoint".
    * oscarui_endpoint: To access the OSCAR Dashboard. This one supports both username/password authentication and authentication via EGI Check-In for the user who deployed the OSCAR cluster and the users belonging to the VO specified at deployment time, if any.  

    ![outputs](images/im-dashboard/im-dashboard-07.png)

    The OSCAR Dashboard can be accessed with the username ``oscar`` and the password
    you specified at deployment time.

    ![access-oscar](images/im-dashboard/im-dashboard-08.png)

    The MinIO UI can be accessed with the username ``minio`` and the password
    you specified at deployment time.

    ![access-minio](images/im-dashboard/im-dashboard-09.png)

    The Kubernetes Dashboard can be accessed with the token you specified at
    deployment time.
    ![access-kubernetes](images/im-dashboard/im-dashboard-10.png)

    You can obtain statistics about the Kubernetes cluster:
    ![access-kubernetes-2](images/im-dashboard/im-dashboard-11.png)

2. Terminating the OSCAR cluster

    You can terminate the OSCAR cluster from the IM Dashboard:
    ![terminate](images/im-dashboard/im-dashboard-12.png)
