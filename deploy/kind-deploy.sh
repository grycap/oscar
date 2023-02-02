#!/bin/bash
CHECK="\xE2\x9C\x94"
GREEN="\e[32m"
RED="\e[31m"
ORANGE="\e[167m"
END_COLOR="\033[0m"

CONFIG_FILEPATH="$(pwd)/config.yaml"
KNATIVE_FILEPATH="$(pwd)/knative.yaml"
MINIO_HELM_NAME="minio"
NFS_HELM_NAME="nfs-server-provisioner"
OSCAR_HELM_NAME="oscar"

ARCH=`uname -m`
SO=`uname -a | awk '{print $1}' | tr '[:upper:]' '[:lower:]'`

#Generate simple random passwords for OSCAR and MinIO
OSCAR_PASSWORD=`date +%s | sha256sum | base64 | head -c 8`
sleep 1
MINIO_PASSWORD=`date +%s | sha256sum | base64 | head -c 8` 

#Not use knative by default
use_knative="n"

showInfo(){
    echo "[*] This script will install a Kubernetes cluster using Kind along with all the required OSCAR services (if not installed): "
    echo -e "\n- MinIO"
    echo -e "- Helm"
    echo -e "- Kubectl\n"
    read -p "No additional changes to your system will be performed. Would you like to continue? [y/n] " res </dev/tty

    if [ -z "$res" ]; then
        echo -e "$RED[!]$END_COLOR Error: Response cannot be empty"
        exit
    fi

    if [ $(echo $res | tr '[:upper:]' '[:lower:]') == 'n' ]; then 
        echo "Stopping execution ..."
        exit
    fi
}

#Check if Docker is installed
checkDocker(){
    if  ! command -v docker &> /dev/null; then
        echo -e "$RED[!]$END_COLOR Docker installation not found. Install Docker to run this test."
        echo -e "Stopping execution ..."
        exit
    else
        echo -e "$GREEN$CHECK$END_COLOR Docker installation found"

        rep=$(curl -s --unix-socket /var/run/docker.sock http://ping > /dev/null)
        status=$?

        if [ "$status" == "7" ]; then
            echo -e "$RED[!]$END_COLOR Error: Docker daemon is not working!"
            exit
        fi

    fi
}

#Check if kubectl is installed
checkKubectl(){
    if  ! command -v kubectl &> /dev/null; then
        echo -e "$ORANGE[*]$END_COLOR kubectl installation not found."
        #Installation here
            curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/$SO/amd64/kubectl"
            if [ $SO == "darwin" ]; then
                chmod +x ./kubectl
                sudo mv ./kubectl /usr/local/bin/kubectl
                sudo chown root: /usr/local/bin/kubectl
            else
                sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
                rm kubectl
            fi
    else
        echo -e "$GREEN$CHECK$END_COLOR kubectl client found"
    fi
}

#Check if helm is installed
checkHelm(){
    if ! command -v helm &> /dev/null; then
        echo -e "$ORANGE[*]$END_COLOR Helm installation not found."
        #Installation here
            curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
            chmod 700 get_helm.sh
            ./get_helm.sh
    else
        echo -e "$GREEN$CHECK$END_COLOR Helm installation found"
    fi
}

#Check if kind is installed
checkKind(){
    if  ! command -v kind &> /dev/null; then
        echo -e "$ORANGE[*]$END_COLOR Kind installation not found."
        #Forced to accept insecure cert
        curl -k -Lo ./kind https://kind.sigs.k8s.io/dl/v0.12.0/kind-$SO-amd64
        chmod +x ./kind

        if `whoami` 2>/dev/null != "root"; then
            sudo mv ./kind /usr/local/bin/kind
        else
            mv ./kind /usr/local/bin/kind
        fi
    else
        echo -e "$GREEN$CHECK$END_COLOR kind installation found"
    fi
}

checkIngressStatus(){
    timeout=500
    echo -e "\n[*] Waiting for running ingress-controller pod ..."
    sleep 5
    start=$(date +%s)
    while [ "$ing_status" != "Running" ]; do
        ing_status=`kubectl get pods -n ingress-nginx 2>/dev/null | awk '/controller/ {print $3}'`
        actual=$(date +%s)
        if [ `expr $actual - $start` -gt $timeout ]; then
            echo -e "\n$RED[!]$END_COLOR Error: Timeout: Pod status: $status"
            exit
        fi
    done
    echo -e "\n[$GREEN$CHECK$END_COLOR] ingress-controller pod running correctly"
}

checkOSCARDeploy(){
    timeout=100
    start=$(date +%s)
    while [ "$status" != "Running" ]; do
        status=`kubectl get pods -n oscar 2>/dev/null | awk '/oscar/ {print $3}'`
        actual=$(date +%s)
        if [ `expr $actual - $start` -gt $timeout ]; then
            echo -e "\n$RED[!]$END_COLOR Error: Timeout: Pod status: $status"
            exit
        fi
    done
    echo -e "\n[$GREEN$CHECK$END_COLOR] OSCAR platform deployed correctly"
    echo -e "\n > You can now acces to the OSCAR web interface through https://localhost with the following credentials: "
    echo "  - username: oscar"
    echo "  - password: $OSCAR_PASSWORD"
    echo -e "\n > You can now access to MinIO console through http://localhost:30300 with the following credentials: "
    echo "  - username: minio"
    echo "  - password: $MINIO_PASSWORD"
    echo -e "\n[*] Note: To delete the cluster type 'kind delete cluster --name=oscar-test'\n"
}

deployKnative(){
    echo -e "\n[*] Deploying Knative Serving ..."
    kubectl apply -f https://github.com/knative/operator/releases/download/knative-v1.6.0/operator.yaml
    cat  > $KNATIVE_FILEPATH <<EOF
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
  version: 1.6.0
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

    kubectl apply -f $KNATIVE_FILEPATH
}

createKindCluster(){
    echo -e "\n[*] Creating kind cluster"
    kind create cluster --config=$CONFIG_FILEPATH --name=oscar-test

    if [ ! `kubectl cluster-info --context kind-oscar-test` &> /dev/null ]; then
        echo -e "$RED[*]$END_COLOR Kind cluster not found."
        echo "Stopping execution ...."
        if [ -f $CONFIG_FILEPATH ]; then 
            rm $CONFIG_FILEPATH
        fi
        exit
    fi
}

showInfo

echo -e "\n[*] Checking prerequisites ..."
checkDocker
checkKubectl
checkHelm
checkKind

echo -e "\n"
read -p "Do you want to use Knative Serving as Serverless Backend? [y/n] " use_knative </dev/tty
read -p "Do you want suport for local docker images? [y/n] " local_reg </dev/tty

#Deploy Knative Serving
if [ `echo $local_reg | tr '[:upper:]' '[:lower:]'` == "y" ]; then 
    reg_name='local-registry'
    reg_port='5001'

    # create registry container unless it already exists
    if [ "$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)" != 'true' ]; then
        docker run -d --restart=always -p "127.0.0.1:${reg_port}:5000" --name "${reg_name}" registry:2
    fi

# Kind cluster definition with local registry
cat > $CONFIG_FILEPATH <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_name}:5000"]
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
    #Create kind cluster
    createKindCluster

    # connect the registry to the cluster network if not already connected
    if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
    docker network connect "kind" "${reg_name}"
    fi

# -- necessary? --
# Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
name: local-registry-hosting
namespace: kube-public
data:
localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

else

# Default Kind cluster definition
cat > $CONFIG_FILEPATH <<EOF
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
    #Create kind cluster
    createKindCluster
fi

#Deploy nginx ingress
echo -e "\n[*] Deploying NGINX Ingress ..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/deploy/static/provider/kind/deploy.yaml
checkIngressStatus

#Deploy MinIO
echo -e "\n[*] Deploying MinIO storage provider ..."
helm repo add --force-update minio https://charts.min.io
helm install minio minio/minio --namespace minio --set rootUser=minio,rootPassword=$MINIO_PASSWORD,service.type=NodePort,service.nodePort=30300,consoleService.type=NodePort,consoleService.nodePort=30301,mode=standalone,resources.requests.memory=512Mi,environment.MINIO_BROWSER_REDIRECT_URL=http://localhost:30301 --create-namespace --version 4.0.7

#Deploy NFS server provisioner
echo -e "\n[*] Deploying NFS server provider ..."
helm repo add --force-update nfs-ganesha-server-and-external-provisioner https://kubernetes-sigs.github.io/nfs-ganesha-server-and-external-provisioner/
if [ $ARCH == "arm64" ]; then
    helm install nfs-server-provisioner nfs-ganesha-server-and-external-provisioner/nfs-server-provisioner --set image.repository=ghcr.io/grycap/nfs-provisioner-arm64 --set image.tag=latest
else
    helm install nfs-server-provisioner nfs-ganesha-server-and-external-provisioner/nfs-server-provisioner --set image.tag=v3.0.1
fi

#Deploy Knative Serving
if [ `echo $use_knative | tr '[:upper:]' '[:lower:]'` == "y" ]; then 
    deployKnative
fi

echo -e "\n[*] Creating namespaces ..."
#Create namespaces
kubectl apply -f https://raw.githubusercontent.com/grycap/oscar/master/deploy/yaml/oscar-namespaces.yaml

#Deploy oscar using helm
echo -e "\n[*] Deploying OSCAR ..."
helm repo add --force-update grycap https://grycap.github.io/helm-charts/
if [ `echo $use_knative | tr '[:upper:]' '[:lower:]'` == "y" ]; then 
    helm install --namespace=oscar oscar grycap/oscar --set authPass=$OSCAR_PASSWORD --set service.type=ClusterIP --set ingress.create=true --set volume.storageClassName=nfs --set minIO.endpoint=http://minio.minio:9000 --set minIO.TLSVerify=false --set minIO.accessKey=minio --set minIO.secretKey=$MINIO_PASSWORD --set serverlessBackend=knative
else
    helm install --namespace=oscar oscar grycap/oscar --set authPass=$OSCAR_PASSWORD --set service.type=ClusterIP --set ingress.create=true --set volume.storageClassName=nfs --set minIO.endpoint=http://minio.minio:9000 --set minIO.TLSVerify=false --set minIO.accessKey=minio --set minIO.secretKey=$MINIO_PASSWORD
fi

#Wait for OSCAR deployment
checkOSCARDeploy

rm $CONFIG_FILEPATH

if [ `echo $use_knative | tr '[:upper:]' '[:lower:]'` == "y" ]; then 
    rm $KNATIVE_FILEPATH
fi
