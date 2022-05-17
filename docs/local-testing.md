# Local Testing with kind

The easiest way to test the OSCAR platform locally is using [kind](https://kind.sigs.k8s.io/). Kind allows the deployment of Kubernetes clusters inside Docker containers and automatically configures `kubectl` to access them.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/), required by kind to launch the Kubernetes nodes on containers.
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) to communicate with the Kubernetes cluster.
- [Helm](https://helm.sh/docs/intro/install/) to easily deploy applications on Kubernetes.
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) to deploy the local Kubernetes cluster.

## Automated local testing

To set up the enviroment for the platform testing you can run the following command. This script automatically executes all the necessary steps to deploy the local cluster and the OSCAR platform along with all the required tools.

``` sh
curl -sSL http://go.oscar.grycap.net | bash
```
## Steps for manual local testing

If you want to do it manualy you can follow the listed steps.

### Create the cluster

To create a single node cluster with MinIO and Ingress controller ports locally accessible, run:

```sh
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
  - containerPort: 30300
    hostPort: 30300
    protocol: TCP
  - containerPort: 30301
    hostPort: 30301
    protocol: TCP
EOF
```

### Deploy NGINX Ingress

To enable Ingress support for accessing the OSCAR server, we must deploy the [NGINX Ingress](https://kubernetes.github.io/ingress-nginx/):

```sh
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/deploy/static/provider/kind/deploy.yaml
```

### Deploy MinIO

OSCAR depends on [MinIO](https://min.io/) as a storage provider and function trigger. The easy way to run MinIO in a Kubernetes cluster is by installing its [helm chart](https://github.com/minio/charts). To  install the helm MinIO repo and install the chart, run the following commands replacing `<MINIO_PASSWORD>` with a password. It must have at least 8 characters:

```sh
helm repo add minio https://charts.min.io
helm install minio minio/minio --namespace minio --set rootUser=minio,rootPassword=<MINIO_PASSWORD>,service.type=NodePort,service.nodePort=30300,consoleService.type=NodePort,consoleService.nodePort=30301,mode=standalone,resources.requests.memory=512Mi,environment.MINIO_BROWSER_REDIRECT_URL=http://localhost:30301 --create-namespace
```

*Note that the deployment has been configured to use the rootUser `minio` and the specified password as rootPassword. The NodePort service type has been used in order to allow access from `http://localhost:30300` (API) and `http://localhost:30301` (Console).*

### Deploy NFS server provisioner

NFS server provisioner is required for the creation of `ReadWriteMany` PersistentVolumes in the kind cluster. This is needed by the OSCAR services to mount the volume with the [FaaS Supervisor](https://github.com/grycap/faas-supervisor) inside the job containers.

To deploy it you can use [this chart](https://github.com/kubernetes-sigs/nfs-ganesha-server-and-external-provisioner/tree/master/deploy/helm) executing:

```sh
helm repo add nfs-ganesha-server-and-external-provisioner https://kubernetes-sigs.github.io/nfs-ganesha-server-and-external-provisioner/
helm install nfs-server-provisioner nfs-ganesha-server-and-external-provisioner/nfs-server-provisioner
```

*Some Linux distributions may have [problems](https://github.com/kubernetes-sigs/kind/issues/1487#issuecomment-694920754) using the [NFS server provisioner](https://github.com/kubernetes-sigs/nfs-ganesha-server-and-external-provisioner) with kind due to its default configuration of kernel-limit file descriptors. To workaround it, please run `sudo sysctl -w fs.nr_open=1048576`.*

### Deploy Knative Serving as Serverless Backend (OPTIONAL)

OSCAR supports [Knative Serving](https://knative.dev/docs/serving/) as Serverless Backend to process [synchronous invocations](invoking.md#synchronous-invocations). If you want to deploy it in the kind cluster, first you must deploy the [Knative Operator](https://knative.dev/docs/install/operator/knative-with-operators/)

```
kubectl apply -f https://github.com/knative/operator/releases/download/knative-v1.3.1/operator.yaml
```

*Note that the above command deploys the version `v1.3.1` of the Operator. You can check if there are new versions [here](https://github.com/knative/operator/releases).*

Once the Operator has been successfully deployed, you can install the Knative Serving stack with the following command:

```
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Namespace
metadata:
  name: knative-serving
---
apiVersion: operator.knative.dev/v1beta1
kind: KnativeServing
metadata:
  name: knative-serving
  namespace: knative-serving
spec:
  version: 1.3.0
  ingress:
    kourier:
      enabled: true
      service-type: ClusterIP
  config:
    config-features:
      kubernetes.podspec-persistent-volume-claim: enabled
      kubernetes.podspec-persistent-volume-write: enabled
    network:
      ingress-class: "kourier.ingress.networking.knative.dev"
EOF
```

### Deploy OSCAR

First, create the `oscar` and `oscar-svc` namespaces by executing:

```sh
kubectl apply -f https://raw.githubusercontent.com/grycap/oscar/master/deploy/yaml/oscar-namespaces.yaml
```

Then, add the [grycap helm repo](https://github.com/grycap/helm-charts) and deploy by running the following commands replacing `<OSCAR_PASSWORD>` with a password of your choice and `<MINIO_PASSWORD>` with the MinIO rootPassword, and remember to add the flag `--set serverlessBackend=knative` if you deployed it in the previous step:

```sh
helm repo add grycap https://grycap.github.io/helm-charts/
helm install --namespace=oscar oscar grycap/oscar --set authPass=<OSCAR_PASSWORD> --set service.type=ClusterIP --set ingress.create=true --set volume.storageClassName=nfs --set minIO.endpoint=http://minio.minio:9000 --set minIO.TLSVerify=false --set minIO.accessKey=minio --set minIO.secretKey=<MINIO_PASSWORD>
```

Now you can access to the OSCAR web interface through `https://localhost` with user `oscar` and the specified password.

*Note that the OSCAR server has been configured to use the ClusterIP service of MinIO for internal communication. This blocks the MinIO section in the OSCAR web interface, so to download and upload files you must connect directly to MinIO (`http://localhost:30300`).*

### Delete the cluster

Once you have finished testing the platform, you can remove the local kind cluster by executing:

```sh
kind delete cluster
```

*Remember that if you have more than one cluster created, it may be required to set the `--name` flag to specify the name of the cluster to be deleted.*


### Using OSCAR-CLI in localhost

To use OSCAR-CLI in localhost deployment, you should change the ssl_verify option to false in the file $HOME/.oscar-cli/config.yaml

```
oscar:
  oscar-cluster:
    endpoint: https://localhost:443
    auth_user: $oscar_username
    auth_password: $oscar_password
    ssl_verify: false
    memory: 256Mi
    log_level: INFO
default: oscar-cluster

```