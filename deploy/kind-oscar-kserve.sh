#!/bin/bash
CHECK="\xE2\x9C\x94"
GREEN="\e[32m"
RED="\e[31m"
ORANGE="\e[167m"
END_COLOR="\033[0m"

CONFIG_FILEPATH=$(mktemp -t oscar-kind-config.XXXXXX)
MINIO_HELM_NAME="minio"
NFS_HELM_NAME="nfs-server-provisioner"
OSCAR_HELM_NAME="oscar"

ARCH=`uname -m`
SO=`uname -a | awk '{print $1}' | tr '[:upper:]' '[:lower:]'`

#Generate simple random passwords for OSCAR and MinIO
OSCAR_PASSWORD=`date +%s | sha256sum | base64 | head -c 8`
sleep 1
MINIO_PASSWORD=`date +%s | sha256sum | base64 | head -c 8` 

RANDOM_SUFFIX=`echo $OSCAR_PASSWORD | cut -c1-3 | tr '[:upper:]' '[:lower:]'`
if [ -z "$RANDOM_SUFFIX" ]; then
    RANDOM_SUFFIX=`LC_CTYPE=C tr -dc 'a-z0-9' </dev/urandom | head -c 3`
fi
CLUSTER_NAME="oscar-kserve-$RANDOM_SUFFIX"
KIND_CONTEXT="kind-$CLUSTER_NAME"
DEFAULT_HTTP_PORT=80
DEFAULT_HTTPS_PORT=443
DEFAULT_MINIO_API_PORT=30300
DEFAULT_MINIO_CONSOLE_PORT=30301
DEFAULT_REGISTRY_PORT=5001
OSCAR_IMAGE_BRANCH="master"
OSCAR_BRANCH_SET="false"
OSCAR_HELM_IMAGE_OVERRIDES=""
OSCAR_POST_DEPLOYMENT_IMAGE=""
OSCAR_TARGET_REPLICAS=1
SKIP_PROMPTS="false"
ENABLE_OIDC="false"
APPLY_LIGHT_SYSTEM_PATCH="false"
OSCAR_SERVERLESS_BACKEND="Knative"
KNATIVE_OPERATOR_VERSION="v1.18.0"
KNATIVE_SERVING_VERSION="1.18.0"
OIDC_ISSUERS_DEFAULT="https://keycloak.grycap.net/realms/grycap"
OIDC_GROUPS_DEFAULT="/oscar-staff, /oscar-test"

usage(){
    cat <<EOF
Usage: $(basename "$0") [options]

Options:
  -y, --yes      Skip all interactive prompts and use defaults.
  --devel        Deploy using the OSCAR devel branch without interactive prompts.
  --branch NAME  Deploy using the specified OSCAR git branch (default: master).
  --oidc         Enable OIDC support for OSCAR (default: disabled).
  --light-system Apply a low-resource-friendly KServe patch for LLM model downloads.
  -h, --help     Show this help message and exit.
EOF
}

showInfo(){
    echo "[*] This script will install a Kubernetes cluster using Kind along with all the required OSCAR services (if not installed): "
    echo -e "\n- MinIO"
    echo -e "- Helm"
    echo -e "- Kubectl"
    echo -e "- KServe\n"

    if [ "$SKIP_PROMPTS" == "true" ]; then
        echo "[*] --devel flag detected. Continuing without interactive confirmation."
        return
    fi

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

portInUse(){
    local port=$1
    if command -v lsof &> /dev/null; then
        if lsof -PiTCP -sTCP:LISTEN -n -P 2>/dev/null | grep -q ":${port} " ; then
            return 0
        fi
    fi
    if command -v netstat &> /dev/null; then
        if netstat -an 2>/dev/null | grep -E "[:\.]${port}[[:space:]].*LISTEN" >/dev/null; then
            return 0
        fi
    fi
    if command -v docker &> /dev/null; then
        if docker ps --format '{{.Ports}}' 2>/dev/null | tr ',' '\n' | grep -E "(:|::)${port}->" >/dev/null; then
            return 0
        fi
    fi
    if command -v nc &> /dev/null; then
        if nc -z localhost "$port" >/dev/null 2>&1; then
            return 0
        fi
        if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
            return 0
        fi
    elif command -v python3 &> /dev/null; then
        python3 - "$port" <<'PY'
import socket, sys
port = int(sys.argv[1])
for family, host in ((socket.AF_INET, "127.0.0.1"), (socket.AF_INET6, "::1")):
    try:
        with socket.socket(family, socket.SOCK_STREAM) as s:
            s.settimeout(0.2)
            if s.connect_ex((host, port)) == 0:
                sys.exit(0)
    except OSError:
        continue
sys.exit(1)
PY
        if [ $? -eq 0 ]; then
            return 0
        fi
    fi
    if [ "$port" -ge 1024 ]; then
        if command -v python3 &> /dev/null; then
            if ! python3 - "$port" <<'PY'
import errno
import socket
import sys

port = int(sys.argv[1])

def can_bind(family, addr):
    try:
        s = socket.socket(family, socket.SOCK_STREAM)
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        s.bind((addr, port))
        s.close()
        return True
    except OSError as exc:
        if exc.errno in (getattr(errno, "EADDRINUSE", 98), getattr(errno, "EACCES", 13)):
            return False
        if exc.errno in (
            getattr(errno, "EAFNOSUPPORT", 97),
            getattr(errno, "EOPNOTSUPP", 95),
            getattr(errno, "EPROTONOSUPPORT", 93),
            getattr(errno, "EADDRNOTAVAIL", 99),
            getattr(errno, "EINVAL", 22),
        ):
            return True
        return False

if not can_bind(socket.AF_INET, "0.0.0.0"):
    sys.exit(1)

if not can_bind(socket.AF_INET6, "::"):
    sys.exit(1)

sys.exit(0)
PY
            then
                return 0
            fi
        elif command -v python &> /dev/null; then
            if ! python - "$port" <<'PY'
import errno
import socket
import sys

port = int(sys.argv[1])

def can_bind(family, addr):
    try:
        s = socket.socket(family, socket.SOCK_STREAM)
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        s.bind((addr, port))
        s.close()
        return True
    except socket.error as exc:
        if exc.errno in (getattr(errno, "EADDRINUSE", 98), getattr(errno, "EACCES", 13)):
            return False
        if exc.errno in (
            getattr(errno, "EAFNOSUPPORT", 97),
            getattr(errno, "EOPNOTSUPP", 95),
            getattr(errno, "EPROTONOSUPPORT", 93),
            getattr(errno, "EADDRNOTAVAIL", 99),
            getattr(errno, "EINVAL", 22),
        ):
            return True
        return False

if not can_bind(socket.AF_INET, "0.0.0.0"):
    sys.exit(1)

if not can_bind(socket.AF_INET6, "::"):
    sys.exit(1)

sys.exit(0)
PY
            then
                return 0
            fi
        fi
    fi
    return 1
}

findAvailablePort(){
    local default_port=$1
    shift
    local candidates=("$default_port" "$@")
    for candidate in "${candidates[@]}"; do
        if ! portInUse "$candidate"; then
            echo "$candidate"
            return 0
        fi
    done
    echo ""
    return 1
}

findAvailablePortExclude(){
    local default_port=$1
    local excluded_port=$2
    shift 2
    local candidates=("$default_port" "$@")
    for candidate in "${candidates[@]}"; do
        if [ "$candidate" == "$excluded_port" ]; then
            continue
        fi
        if ! portInUse "$candidate"; then
            echo "$candidate"
            return 0
        fi
    done
    echo ""
    return 1
}

#Check if Docker is installed
checkDocker(){
    if  ! command -v docker &> /dev/null; then
        echo -e "$RED[!]$END_COLOR Docker installation not found. Install Docker to run this test."
        echo -e "Stopping execution ..."
        exit
    else
        echo -e "$GREEN$CHECK$END_COLOR Docker installation found"

        rep=$(docker info)
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
    creation_timeout=120
    readiness_timeout=600
    start=$(date +%s)
    echo -e "\n[*] Waiting for OSCAR pods to be scheduled ..."
    while true; do
        pod_info=$(kubectl get pods -n oscar -l app=oscar --no-headers 2>/dev/null)
        if [ -n "$pod_info" ]; then
            pod_count=$(echo "$pod_info" | wc -l | tr -d ' ')
            echo -e "\n[*] Detected $pod_count OSCAR pod(s). Waiting for them to become ready (timeout ${readiness_timeout}s) ..."
            break
        fi
        actual=$(date +%s)
        if [ $((actual - start)) -gt $creation_timeout ]; then
            echo -e "\n$RED[!]$END_COLOR Error: OSCAR pods were not created after ${creation_timeout}s."
            kubectl get pods -n oscar
            exit 1
        fi
        sleep 5
    done

    if ! kubectl wait --namespace oscar --for=condition=Ready pod -l app=oscar --timeout="${readiness_timeout}s"; then
        echo -e "\n$RED[!]$END_COLOR Error: OSCAR pods did not become ready after ${readiness_timeout}s."
        kubectl get pods -n oscar
        failing_pods=$(kubectl get pods -n oscar -l app=oscar --no-headers | awk '{
            split($2, ready, "/");
            if (ready[1] != ready[2] || $3 != "Running") {
                print "- " $1 " (ready=" $2 ", status=" $3 ")"
            }
        }')
        if [ -n "$failing_pods" ]; then
            echo -e "\n[*] Pods still unstable:"
            echo "$failing_pods"
        fi
        echo -e "\n[*] Recent OSCAR namespace events:"
        kubectl get events -n oscar --sort-by=.metadata.creationTimestamp | tail -n 20
        exit 1
    fi
    echo -e "\n[$GREEN$CHECK$END_COLOR] OSCAR platform deployed correctly"
    if [ "$HOST_HTTPS_PORT" == "$DEFAULT_HTTPS_PORT" ]; then
        oscar_url="https://localhost"
    else
        oscar_url="https://localhost:$HOST_HTTPS_PORT"
    fi
    echo -e "\n > You can now acces to the OSCAR web interface through $oscar_url with the following credentials: "
    echo "  - username: oscar"
    echo "  - password: $OSCAR_PASSWORD"
    minio_api_url="http://localhost:$HOST_MINIO_API_PORT"
    minio_console_url="http://localhost:$HOST_MINIO_CONSOLE_PORT"
    echo -e "\n > You can now access MinIO object storage through $minio_api_url and the console through $minio_console_url with the following credentials: "
    echo "  - username: minio"
    echo "  - password: $MINIO_PASSWORD"
    echo -e "\n[*] Note: To delete the cluster type 'kind delete cluster --name=$CLUSTER_NAME'\n"
}

deployKServe(){
    local script_dir
    local kserve_root
    local kserve_repo_url
    local kserve_parent_dir

    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    kserve_root="${KSERVE_ROOT:-${script_dir}/../../kserve}"
    kserve_repo_url="${KSERVE_REPO_URL:-https://github.com/kserve/kserve.git}"
    kserve_parent_dir="$(dirname "$kserve_root")"

    if [ ! -d "$kserve_root" ]; then
        echo -e "\n[*] KServe repository not found at: $kserve_root"
        echo -e "[*] Cloning KServe repository from: $kserve_repo_url"
        if ! command -v git &> /dev/null; then
            echo -e "$RED[!]$END_COLOR git is required to clone KServe. Install git or set KSERVE_ROOT to an existing repository."
            exit 1
        fi
        mkdir -p "$kserve_parent_dir"
        if ! git clone "$kserve_repo_url" "$kserve_root"; then
            echo -e "$RED[!]$END_COLOR Failed to clone KServe repository from $kserve_repo_url"
            exit 1
        fi
        echo -e "[$GREEN$CHECK$END_COLOR] KServe repository cloned to $kserve_root"
    fi

    echo -e "\n[*] Deploying KServe from: $kserve_root"
    (
        cd "$kserve_root" || exit 1

        if [ ! -x "./hack/setup/quick-install/kserve-standard-mode-dependency-install.sh" ]; then
            echo -e "$RED[!]$END_COLOR Missing executable script: ./hack/setup/quick-install/kserve-standard-mode-dependency-install.sh"
            exit 1
        fi
        if [ ! -x "./hack/setup/quick-install/llmisvc-dependency-install.sh" ]; then
            echo -e "$RED[!]$END_COLOR Missing executable script: ./hack/setup/quick-install/llmisvc-dependency-install.sh"
            exit 1
        fi
        if [ ! -x "./hack/setup/infra/manage.kserve-kustomize.sh" ]; then
            echo -e "$RED[!]$END_COLOR Missing executable script: ./hack/setup/infra/manage.kserve-kustomize.sh"
            exit 1
        fi

        ./hack/setup/quick-install/kserve-standard-mode-dependency-install.sh
        ./hack/setup/quick-install/llmisvc-dependency-install.sh
        DEPLOYMENT_MODE=Standard INSTALL_RUNTIMES=true ENABLE_LLMISVC=true INSTALL_LLMISVC_CONFIGS=true hack/setup/infra/manage.kserve-kustomize.sh
    )

    if [ $? -ne 0 ]; then
        echo -e "$RED[!]$END_COLOR KServe deployment failed"
        exit 1
    fi

    # Workaround: ensure llmisvc controller RBAC includes CRD permissions
    if ! kubectl get clusterrole kserve-llmisvc-manager-role -o yaml 2>/dev/null | grep -q "customresourcedefinitions"; then
        echo -e "[*] Applying llmisvc RBAC fix (missing CRD permissions) ..."
        if [ -f "${kserve_root}/config/rbac/llmisvc/role.yaml" ]; then
            kubectl apply -f "${kserve_root}/config/rbac/llmisvc/role.yaml"
            if kubectl -n kserve get deployment llmisvc-controller-manager >/dev/null 2>&1; then
                echo -e "[*] Restarting llmisvc-controller-manager after RBAC fix ..."
                kubectl -n kserve rollout restart deployment/llmisvc-controller-manager
                kubectl -n kserve rollout status deployment/llmisvc-controller-manager --timeout=300s
            fi
        else
            echo -e "$RED[!]$END_COLOR Missing KServe RBAC file: ${kserve_root}/config/rbac/llmisvc/role.yaml"
            exit 1
        fi
    fi

    echo -e "\n[*] Verifying installed KServe runtimes ..."
    kubectl get clusterservingruntime
    runtime_count=$(kubectl get clusterservingruntime --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [ "${runtime_count}" -eq 0 ]; then
        echo -e "$RED[!]$END_COLOR No ClusterServingRuntime resources were installed"
        exit 1
    fi
    echo -e "[$GREEN$CHECK$END_COLOR] Found ${runtime_count} ClusterServingRuntime resources"

    echo -e "[$GREEN$CHECK$END_COLOR] KServe deployed successfully"

}

applyLightSystemPatch(){
    echo -e "\n[*] Applying light-system KServe patch ..."
    if ! kubectl -n kserve patch configmap inferenceservice-config --type merge --patch "$(cat <<'EOF'
data:
  storageInitializer: |
    {
        "image" : "kserve/storage-initializer:latest",
        "memoryRequest": "1Gi",
        "memoryLimit": "4Gi",
        "cpuRequest": "100m",
        "cpuLimit": "1",
        "caBundleConfigMapName": "",
        "caBundleVolumeMountPath": "/etc/ssl/custom-certs",
        "enableModelcar": true,
        "cpuModelcar": "10m",
        "memoryModelcar": "15Mi",
        "uidModelcar": 1010
    }
EOF
)"; then
        echo -e "$RED[!]$END_COLOR Failed to patch kserve/inferenceservice-config"
        exit 1
    fi

    if kubectl -n kserve get deployment llmisvc-controller-manager >/dev/null 2>&1; then
        echo -e "[*] Restarting llmisvc-controller-manager to reload KServe config ..."
        kubectl -n kserve rollout restart deployment/llmisvc-controller-manager
        kubectl -n kserve rollout status deployment/llmisvc-controller-manager --timeout=300s
    fi

    if kubectl -n lws-system get deployment lws-controller-manager >/dev/null 2>&1; then
        echo -e "[*] Scaling lws-controller-manager to 1 replica for light-system mode ..."
        kubectl -n lws-system scale deployment/lws-controller-manager --replicas=1
        kubectl -n lws-system rollout status deployment/lws-controller-manager --timeout=300s
    fi

    echo -e "[$GREEN$CHECK$END_COLOR] Light-system patch applied"
}

deployKnativeServing(){
    echo -e "\n[*] Deploying Knative Serving for OSCAR backend ..."
    if ! kubectl apply -f "https://github.com/knative/operator/releases/download/knative-${KNATIVE_OPERATOR_VERSION}/operator.yaml"; then
        echo -e "$RED[!]$END_COLOR Failed to install Knative operator (version ${KNATIVE_OPERATOR_VERSION})"
        exit 1
    fi

    if ! kubectl apply -f - <<EOF
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
  version: "${KNATIVE_SERVING_VERSION}"
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
    domain:
      example.com: ""
EOF
    then
        echo -e "$RED[!]$END_COLOR Failed to create KnativeServing custom resource (version ${KNATIVE_SERVING_VERSION})"
        exit 1
    fi

    if ! kubectl wait --for=condition=Ready knativeserving/knative-serving -n knative-serving --timeout=900s; then
        echo -e "$RED[!]$END_COLOR Knative Serving did not become ready"
        kubectl get pods -n knative-serving
        exit 1
    fi

    if ! kubectl get ns knative-serving >/dev/null 2>&1; then
        echo -e "$RED[!]$END_COLOR Knative namespace knative-serving not found after installation"
        exit 1
    fi
    if ! kubectl get crd services.serving.knative.dev >/dev/null 2>&1; then
        echo -e "$RED[!]$END_COLOR Knative Serving CRD services.serving.knative.dev not found after installation"
        exit 1
    fi

    echo -e "[$GREEN$CHECK$END_COLOR] Knative Serving deployed successfully"
}

createKindCluster(){
    echo -e "\n[*] Creating kind cluster"
    kind create cluster --image kindest/node:v1.33.1 --config=$CONFIG_FILEPATH --name="$CLUSTER_NAME"

    if ! kubectl cluster-info --context "$KIND_CONTEXT" &> /dev/null; then
        echo -e "$RED[*]$END_COLOR Kind cluster not found."
        echo "Stopping execution ...."
        if [ -f $CONFIG_FILEPATH ]; then 
            rm $CONFIG_FILEPATH
        fi
        exit
    fi
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        -y|--yes)
            SKIP_PROMPTS="true"
            shift
            ;;
        --devel)
            SKIP_PROMPTS="true"
            OSCAR_IMAGE_BRANCH="devel"
            shift
            ;;
        --branch)
            shift
            if [ -z "$1" ]; then
                echo -e "$RED[!]$END_COLOR Missing value for --branch"
                usage
                exit 1
            fi
            OSCAR_IMAGE_BRANCH="$1"
            OSCAR_BRANCH_SET="true"
            shift
            ;;
        --oidc)
            ENABLE_OIDC="true"
            shift
            ;;
        --knative-backend|--kubernetes-backend)
            echo -e "$ORANGE[*]$END_COLOR Backend flags are ignored. OSCAR is always deployed with Knative backend."
            shift
            ;;
        --light-system)
            APPLY_LIGHT_SYSTEM_PATCH="true"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo -e "$RED[!]$END_COLOR Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

showInfo

echo -e "\n[*] Checking prerequisites ..."
checkDocker
checkKubectl
checkHelm
checkKind

echo -e "\n"
local_reg="y"
oscar_branch_input=""
use_oidc="n"
if [ "$SKIP_PROMPTS" == "true" ]; then
    echo "[*] Running in non-interactive mode: local registry and OSCAR devel branch enabled."
else
    read -p "Do you want suport for local docker images? [y/n] " local_reg </dev/tty
    if [ "$OSCAR_BRANCH_SET" != "true" ]; then
        read -p "Enter OSCAR git branch to deploy (default: $OSCAR_IMAGE_BRANCH) " oscar_branch_input </dev/tty
    fi
    if [ "$ENABLE_OIDC" != "true" ]; then
        echo -e "\n[*] OIDC defaults to be applied if enabled:"
        echo -e "  - OIDC_ENABLE=true"
        echo -e "  - OIDC_ISSUERS=$OIDC_ISSUERS_DEFAULT"
        echo -e "  - OIDC_GROUPS=$OIDC_GROUPS_DEFAULT"
        read -p "Do you want to enable OIDC authentication support with these defaults? [y/n] " use_oidc </dev/tty
    fi
fi

if [ -n "$oscar_branch_input" ]; then
    OSCAR_IMAGE_BRANCH="$oscar_branch_input"
fi

if [ `echo $use_oidc | tr '[:upper:]' '[:lower:]'` == "y" ]; then
    ENABLE_OIDC="true"
fi
if [ "$OSCAR_IMAGE_BRANCH" == "devel" ]; then
    OSCAR_HELM_IMAGE_OVERRIDES="--set replicas=0"
    OSCAR_POST_DEPLOYMENT_IMAGE="ghcr.io/grycap/oscar:devel"
fi

HTTP_PORT_FALLBACKS=(8080 8081 8082 8880 9080 10080)
HTTPS_PORT_FALLBACKS=(444 8443 9443 10443)
MINIO_API_PORT_FALLBACKS=(30302 30304 30306 31300 32000 32500)
MINIO_CONSOLE_PORT_FALLBACKS=(30303 30305 30307 31301 32001 32501)
HOST_HTTP_PORT=$(findAvailablePort "$DEFAULT_HTTP_PORT" "${HTTP_PORT_FALLBACKS[@]}")
HOST_HTTPS_PORT=$(findAvailablePort "$DEFAULT_HTTPS_PORT" "${HTTPS_PORT_FALLBACKS[@]}")
HOST_MINIO_API_PORT=$(findAvailablePort "$DEFAULT_MINIO_API_PORT" "${MINIO_API_PORT_FALLBACKS[@]}")
HOST_MINIO_CONSOLE_PORT=$(findAvailablePortExclude "$DEFAULT_MINIO_CONSOLE_PORT" "$HOST_MINIO_API_PORT" "${MINIO_CONSOLE_PORT_FALLBACKS[@]}")

if [ -z "$HOST_HTTP_PORT" ]; then
    echo -e "$RED[!]$END_COLOR Error: Unable to find a free port for HTTP ingress"
    exit 1
fi

if [ -z "$HOST_HTTPS_PORT" ]; then
    echo -e "$RED[!]$END_COLOR Error: Unable to find a free port for HTTPS ingress"
    exit 1
fi

if [ -z "$HOST_MINIO_API_PORT" ]; then
    echo -e "$RED[!]$END_COLOR Error: Unable to find a free port for MinIO API"
    exit 1
fi

if [ -z "$HOST_MINIO_CONSOLE_PORT" ]; then
    echo -e "$RED[!]$END_COLOR Error: Unable to find a free port for MinIO console"
    exit 1
fi

if [ "$HOST_HTTP_PORT" != "$DEFAULT_HTTP_PORT" ]; then
    echo -e "$ORANGE[*]$END_COLOR Port 80 is busy. Using $HOST_HTTP_PORT for ingress HTTP instead."
fi

if [ "$HOST_HTTPS_PORT" != "$DEFAULT_HTTPS_PORT" ]; then
    echo -e "$ORANGE[*]$END_COLOR Port 443 is busy. Using $HOST_HTTPS_PORT for ingress HTTPS instead."
fi

if [ "$HOST_MINIO_API_PORT" != "$DEFAULT_MINIO_API_PORT" ]; then
    echo -e "$ORANGE[*]$END_COLOR Port $DEFAULT_MINIO_API_PORT is busy. Using $HOST_MINIO_API_PORT for MinIO API instead."
fi

if [ "$HOST_MINIO_CONSOLE_PORT" != "$DEFAULT_MINIO_CONSOLE_PORT" ]; then
    echo -e "$ORANGE[*]$END_COLOR Port $DEFAULT_MINIO_CONSOLE_PORT is busy. Using $HOST_MINIO_CONSOLE_PORT for MinIO console instead."
fi

if [ `echo $local_reg | tr '[:upper:]' '[:lower:]'` == "y" ]; then 
    reg_name='local-registry'
    registry_status="created"
    reg_port=$DEFAULT_REGISTRY_PORT

    if docker inspect -f '{{.Name}}' "${reg_name}" &>/dev/null; then
        registry_status="reused"
        if [ "$(docker inspect -f '{{.State.Running}}' "${reg_name}")" != 'true' ]; then
            docker start "${reg_name}" >/dev/null
        fi
        existing_port=$(docker port "${reg_name}" 5000/tcp 2>/dev/null | head -n1 | awk -F':' '{print $NF}')
        if [ -n "$existing_port" ]; then
            reg_port=$existing_port
        fi
        echo -e "$GREEN$CHECK$END_COLOR Reusing existing local registry '${reg_name}' on port ${reg_port}"
    else
        if portInUse "$DEFAULT_REGISTRY_PORT"; then
            echo -e "$RED[!]$END_COLOR Port ${DEFAULT_REGISTRY_PORT} is already in use. Stop the service using it or rerun with an existing local registry."
            exit 1
        fi
        docker run -d --restart=always -p "127.0.0.1:${DEFAULT_REGISTRY_PORT}:5000" --name "${reg_name}" registry:2
        reg_port=$DEFAULT_REGISTRY_PORT
        echo -e "$GREEN$CHECK$END_COLOR Local registry '${reg_name}' created on port ${DEFAULT_REGISTRY_PORT}"
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
    hostPort: ${HOST_HTTP_PORT}
    protocol: TCP
  - containerPort: 443
    hostPort: ${HOST_HTTPS_PORT}
    protocol: TCP
  - containerPort: ${HOST_MINIO_API_PORT}
    hostPort: ${HOST_MINIO_API_PORT}
    protocol: TCP
  - containerPort: ${HOST_MINIO_CONSOLE_PORT}
    hostPort: ${HOST_MINIO_CONSOLE_PORT}
    protocol: TCP
EOF
    #Create kind cluster
    createKindCluster

    # connect the registry to the cluster network if not already connected
    if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
    docker network connect "kind" "${reg_name}"
    fi

# Document the local registry
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
    hostPort: ${HOST_HTTP_PORT}
    protocol: TCP
  - containerPort: 443
    hostPort: ${HOST_HTTPS_PORT}
    protocol: TCP
  - containerPort: ${HOST_MINIO_API_PORT}
    hostPort: ${HOST_MINIO_API_PORT}
    protocol: TCP
  - containerPort: ${HOST_MINIO_CONSOLE_PORT}
    hostPort: ${HOST_MINIO_CONSOLE_PORT}
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
if ! helm install minio minio/minio --namespace minio --set rootUser=minio,rootPassword=$MINIO_PASSWORD,service.type=NodePort,service.nodePort=$HOST_MINIO_API_PORT,consoleService.type=NodePort,consoleService.nodePort=$HOST_MINIO_CONSOLE_PORT,mode=standalone,resources.requests.memory=512Mi,environment.MINIO_BROWSER_REDIRECT_URL=http://localhost:$HOST_MINIO_CONSOLE_PORT --create-namespace --version 4.0.7; then
    echo -e "$RED[!]$END_COLOR MinIO deployment failed"
    exit 1
fi

#Deploy NFS server provisioner
echo -e "\n[*] Deploying NFS server provider ..."
helm repo add --force-update nfs-ganesha-server-and-external-provisioner https://kubernetes-sigs.github.io/nfs-ganesha-server-and-external-provisioner/
if [ $ARCH == "arm64" ]; then
    if ! helm install nfs-server-provisioner nfs-ganesha-server-and-external-provisioner/nfs-server-provisioner --set image.repository=ghcr.io/grycap/nfs-provisioner-arm64 --set image.tag=latest; then
        echo -e "$RED[!]$END_COLOR NFS server provisioner deployment failed"
        exit 1
    fi
else
    if ! helm install nfs-server-provisioner nfs-ganesha-server-and-external-provisioner/nfs-server-provisioner --set image.tag=v3.0.1; then
        echo -e "$RED[!]$END_COLOR NFS server provisioner deployment failed"
        exit 1
    fi
fi

#Deploy metrics-server
echo -e "\n[*] Deploying metrics-server ..."
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml && \
kubectl -n kube-system patch deployment metrics-server --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'

#Deploy KServe
deployKServe

if [ "$APPLY_LIGHT_SYSTEM_PATCH" == "true" ]; then
    applyLightSystemPatch
fi

deployKnativeServing

echo -e "\n[*] Creating namespaces ..."
#Create namespaces
kubectl apply -f https://raw.githubusercontent.com/grycap/oscar/master/deploy/yaml/oscar-namespaces.yaml

#Deploy oscar using helm
echo -e "\n[*] Deploying OSCAR ..."
helm repo add --force-update grycap https://grycap.github.io/helm-charts/
helm install --namespace=oscar oscar grycap/oscar --set authPass=$OSCAR_PASSWORD --set service.type=ClusterIP --set ingress.create=true --set volume.storageClassName=nfs --set minIO.endpoint=http://minio.minio:9000 --set minIO.TLSVerify=false --set minIO.accessKey=minio --set minIO.secretKey=$MINIO_PASSWORD --set serverlessBackend=$OSCAR_SERVERLESS_BACKEND --set resourceManager.enable=true $OSCAR_HELM_IMAGE_OVERRIDES

if [ -n "$OSCAR_POST_DEPLOYMENT_IMAGE" ]; then
    echo -e "\n[*] Switching OSCAR deployment to use $OSCAR_POST_DEPLOYMENT_IMAGE ..."
    if ! kubectl -n oscar set image deployment/oscar oscar="$OSCAR_POST_DEPLOYMENT_IMAGE"; then
        echo -e "$RED[!]$END_COLOR Failed to switch OSCAR deployment to $OSCAR_POST_DEPLOYMENT_IMAGE"
        exit 1
    fi
    echo -e "\n[*] Scaling OSCAR deployment to $OSCAR_TARGET_REPLICAS replica(s) ..."
    if ! kubectl -n oscar scale deployment/oscar --replicas="$OSCAR_TARGET_REPLICAS"; then
        echo -e "$RED[!]$END_COLOR Failed to scale OSCAR deployment"
        exit 1
    fi
fi

if [ "$ENABLE_OIDC" == "true" ]; then
    echo -e "\n[*] Enabling OIDC support in OSCAR deployment ..."
    if ! kubectl -n oscar set env deployment/oscar \
        OIDC_ENABLE="true" \
        OIDC_ISSUERS="$OIDC_ISSUERS_DEFAULT" \
        OIDC_GROUPS="$OIDC_GROUPS_DEFAULT"; then
        echo -e "$RED[!]$END_COLOR Failed to set OIDC environment variables in OSCAR deployment"
        exit 1
    fi
fi

#Wait for OSCAR deployment
checkOSCARDeploy
 
echo -e "\n[*] Deployment details:"
echo "  - Kind cluster name: $CLUSTER_NAME"
echo "  - Kind context: $KIND_CONTEXT"
if [ "$HOST_HTTP_PORT" == "$DEFAULT_HTTP_PORT" ]; then
    oscar_http_url="http://localhost"
else
    oscar_http_url="http://localhost:$HOST_HTTP_PORT"
fi
if [ "$HOST_HTTPS_PORT" == "$DEFAULT_HTTPS_PORT" ]; then
    oscar_https_url="https://localhost"
else
    oscar_https_url="https://localhost:$HOST_HTTPS_PORT"
fi
minio_api_url="http://localhost:$HOST_MINIO_API_PORT"
minio_console_url="http://localhost:$HOST_MINIO_CONSOLE_PORT"
echo "  - OSCAR HTTP port: $HOST_HTTP_PORT ($oscar_http_url)"
echo "  - OSCAR HTTPS port: $HOST_HTTPS_PORT ($oscar_https_url)"
echo "  - MinIO API NodePort/host port: $HOST_MINIO_API_PORT ($minio_api_url)"
echo "  - MinIO console NodePort/host port: $HOST_MINIO_CONSOLE_PORT ($minio_console_url)"
echo "  - OSCAR image branch: $OSCAR_IMAGE_BRANCH"
echo "  - OSCAR credentials: username='oscar', password='$OSCAR_PASSWORD'"
echo "  - MinIO credentials: username='minio', password='$MINIO_PASSWORD'"
echo "  - OSCAR Serverless backend: Knative"
echo "  - KServe deployed in Standard mode"
if [ `echo $local_reg | tr '[:upper:]' '[:lower:]'` == "y" ]; then
    echo "  - Local registry: ${reg_name} (port ${reg_port}, ${registry_status})"
else
    echo "  - Local registry: not configured"
fi

echo -e "\n[$GREEN$CHECK$END_COLOR] Deployment completed successfully"

rm $CONFIG_FILEPATH
