#!/bin/bash
CHECK="\e[32m\xE2\x9C\x94\e[0m"
RED="\e[31m"
END_COLOR="\e[0m"

CONFIG_FILEPATH="/tmp/config.yaml"
KNATIVE_FILEPATH="/tmp/knative.yaml"
MINIO_HELM_NAME="minio"
NFS_HELM_NAME="nfs-server-provisioner"
OSCAR_HELM_NAME="oscar"
MIN_PASS_CHAR=8

#Check if Docker is installed
checkDocker(){
    if  ! command -v docker &> /dev/null; then
    echo -e "$RED[!]$END_COLOR Docker installation not found. Install Docker to run this test."
    echo -e "Stopping execution ..."
    exit
    else
    echo -e "$CHECK Docker installation found"
    #check docker not sudo
    fi
}

#Check if kubectl is installed
checkKubectl(){
    if  ! command -v kubectl &> /dev/null; then
    echo -e "$RED[*]$END_COLOR kubectl installation not found."
    read -s "Kubectl is required to communicate with the Kubernetes cluster. Do you want to install it? [y/n]" res
    #Installation here
        if [ `echo $res | tr '[:upper:]' '[:lower:]'` == "y" ]; then
            curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/$(uname -a | awk '{print $1}' | tr '[:upper:]' '[:lower:]')/amd64/kubectl"
        else
            "Stopping execution ... "
            exit
        fi
    else
    echo -e "$CHECK kubectl client found"
    fi
}

#Check if helm is installed
checkHelm(){
    if ! command -v helm &> /dev/null; then
    echo -e "$RED[*]$END_COLOR Helm installation not found."
    read -p "Helm is required to deploy applications in kubernetes. Do you want to install it? [y/n] " res
    #Installation here
        if [ `echo $res | tr '[:upper:]' '[:lower:]'` == "y" ]; then
            curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
            chmod 700 get_helm.sh
            ./get_helm.sh
        else 
            "Stopping execution ... "
            exit
        fi
    else
    echo -e "$CHECK Helm installation found"
    fi
}

#Check if kind is installed
checkKind(){
    if  ! command -v kind &> /dev/null; then
    echo -e "$RED[*]$END_COLOR Kind installation not found."
    read -s "Kind allows you to create a local kubernetes cluster easly. Do you want to install it? [y/n]"
    #Installation here
        if [ `echo $res | tr '[:upper:]' '[:lower:]'` == "y" ]; then
            curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.12.0/kind-$(uname -a | awk '{print $1}' | tr '[:upper:]' '[:lower:]')-amd64
            chmod +x ./kind

            if `whoami` 2>/dev/null != "root"; then
                sudo mv ./kind /usr/local/bin/kind
            else
                mv ./kind /usr/local/bin/kind
            fi
        else
        echo "Stopping execution ... "
        exit
        fi

    else
    echo -e "$CHECK kind installation found"
    fi
}

checkIngressStatus(){ #TODO add timeout
    echo -e "\n[*] Waiting for running ingress-controller pod ..."
    sleep 5
    while [ "$status" != "Running" ]; do
        status=`kubectl get pods -n ingress-nginx 2>/dev/null | awk '/controller/ {print $3}'`
    done
    echo -e "\n[$CHECK ] ingress-controller pod running correctly"
}

checkOSCARDeploy(){ #TODO add timeout
    while [ "$status" != "Running" ]; do
        status=`kubectl get pods -n oscar 2>/dev/null | awk '/oscar/ {print $3}'`
    done
    echo -e "\n[$CHECK ] OSCAR platform deployed correctly"
    echo -e "\n > You can now acces to the OSCAR web interface through https://localhost"
    echo -e " > You can now access to MinIO console through https://localhost:30300 \n"
}

deployKnative(){
    echo -e "\n[*] Deploying Knative Serving ..."
    kubectl apply -f https://github.com/knative/operator/releases/download/knative-v1.3.1/operator.yaml
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

    kubectl apply -f $KNATIVE_FILEPATH
}


echo "[*] Checking prerequisites ..."
checkDocker
checkKubectl
checkHelm
checkKind

echo -e "\n"
read -sp "Enter a password for Oscar: "$'\n' OSCAR_PASSWORD

while [ `echo $MINIO_PASSWORD | awk '{print length}'` -lt $MIN_PASS_CHAR ]; do 
    read -sp "Enter a password for MinIO (min 8 characters): "$'\n' MINIO_PASSWORD
done

read -p "Do you want to use Knative Serving as Serverless Backend? [y/n] " use_knative

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
echo -e "\n[*] Creating kind cluster"
#Cambiar a eof
kind create cluster --config=$CONFIG_FILEPATH

#Deploy nginx ingress
echo -e "\n[*] Deploying NGINX Ingress ..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/deploy/static/provider/kind/deploy.yaml
checkIngressStatus

#Deploy MinIO
echo -e "\n[*] Deploying MinIO storage provider ..."
helm repo add minio https://charts.min.io
helm install minio minio/minio --namespace minio --set rootUser=minio,rootPassword=$MINIO_PASSWORD,service.type=NodePort,service.nodePort=30300,consoleService.type=NodePort,consoleService.nodePort=30301,mode=standalone,resources.requests.memory=512Mi,environment.MINIO_BROWSER_REDIRECT_URL=http://localhost:30301 --create-namespace

#Deploy NFS server provisioner
echo -e "\n[*] Deploying NFS server provider ..."
helm repo add nfs-ganesha-server-and-external-provisioner https://kubernetes-sigs.github.io/nfs-ganesha-server-and-external-provisioner/
helm install nfs-server-provisioner nfs-ganesha-server-and-external-provisioner/nfs-server-provisioner

#Ask use of knative

if [ `echo $use_knative | tr '[:upper:]' '[:lower:]'` == "y" ]; then 
    deployKnative
fi

echo -e "\n[*] Creating namespaces ..."
# #Create namespaces
kubectl apply -f https://raw.githubusercontent.com/grycap/oscar/master/deploy/yaml/oscar-namespaces.yaml

# #Deploy oscar using helm
echo -e "\n[*] Deploying OSCAR ..."
helm repo add grycap https://grycap.github.io/helm-charts/
helm install --namespace=oscar oscar grycap/oscar --set authPass=$OSCAR_PASSWORD --set service.type=ClusterIP --set ingress.create=true --set volume.storageClassName=nfs --set minIO.endpoint=http://minio.minio:9000 --set minIO.TLSVerify=false --set minIO.accessKey=minio --set minIO.secretKey=$MINIO_PASSWORD

#Wait for cluster creation
checkOSCARDeploy