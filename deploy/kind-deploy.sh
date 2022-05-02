#!/bin/bash
CHECK="\e[32m\xE2\x9C\x94\e[0m"
RED="\e[31m"
ORANGE="\e[167m"
END_COLOR="\e[0m"

CONFIG_FILEPATH="/tmp/config.yaml"
KNATIVE_FILEPATH="/tmp/knative.yaml"
MINIO_HELM_NAME="minio"
NFS_HELM_NAME="nfs-server-provisioner"
OSCAR_HELM_NAME="oscar"
MIN_PASS_CHAR=8
OSCAR_PASSWORD=`< /dev/urandom tr -dc _A-Z-a-z-0-9 | head -c${1:-8}`
MINIO_PASSWORD=`< /dev/urandom tr -dc _A-Z-a-z-0-9 | head -c${1:-8}` 

SO=`uname -a | awk '{print $1}' | tr '[:upper:]' '[:lower:]'`

showInfo(){
    echo "[*] This script will install a Kubernetes cluster using Kind along with all the required OSCAR services (if not installed): "
    echo -e "\n- MinIO"
    echo -e "- Helm"
    echo -e "- Kubectl\n"
    read -p "No additional changes to your system will be performed. Would you like to continue? Y/n? [y/n] " res

    if [ $(echo $res | tr '[:upper:]' '[:lower:]') == 'n' ]; then 
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
        echo -e "$CHECK Docker installation found"
        if [ $SO == "darwin" ]; then
            docker_status=`/etc/init.d/docker status | awk '/Active:/ {print $0}' | awk '{print $2}'`
        else
            docker_status=`systemctl status docker.service | awk '/Active:/ {print $0}' | awk '{print $2}'`
        fi
        if [ $docker_status != "active" ]; then
            echo -e "[!] Error: Docker daemon is not working!"
            exit
        fi
        #check docker not sudo
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
        echo -e "$CHECK kubectl client found"
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
        echo -e "$CHECK Helm installation found"
    fi
}

#Check if kind is installed
checkKind(){
    if  ! command -v kind &> /dev/null; then
        echo -e "$ORANGE[*]$END_COLOR Kind installation not found."
        #Installation here
        #Forced to accept insecure cert
        curl -k -Lo ./kind https://kind.sigs.k8s.io/dl/v0.12.0/kind-$SO-amd64
        chmod +x ./kind

        if `whoami` 2>/dev/null != "root"; then
            sudo mv ./kind /usr/local/bin/kind
        else
            mv ./kind /usr/local/bin/kind
        fi
    else
        echo -e "$CHECK kind installation found"
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
    echo -e "\n[$CHECK] ingress-controller pod running correctly"
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
    echo -e "\n[$CHECK] OSCAR platform deployed correctly"
    echo -e "\n > You can now acces to the OSCAR web interface through https://localhost with the following credentials: "
    echo "  - username: oscar"
    echo "  - password: $OSCAR_PASSWORD"
    echo -e "\n > You can now access to MinIO console through https://localhost:30300 with the following credentials: "
    echo "  - username: minio"
    echo "  - password: $MINIO_PASSWORD"
    echo -e "\n[*] Note: To delete the cluster type 'kind delete cluster'\n"
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

showInfo

echo -e "\n[*] Checking prerequisites ..."
checkDocker
checkKubectl
checkHelm
checkKind

echo -e "\n"
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
kind create cluster --config=$CONFIG_FILEPATH

if [ ! `kubectl cluster-info --context kind-kind` &> /dev/null ]; then
    echo -e "$RED[*]$END_COLOR Kind cluster not found."
    echo "Stopping execution ...."
    exit
fi
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

#Deploy Knative Serving
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