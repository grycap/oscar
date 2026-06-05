package utils

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/grycap/oscar/v4/pkg/types"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kueuev1 "sigs.k8s.io/kueue/apis/kueue/v1beta2"
	kueueclientset "sigs.k8s.io/kueue/client-go/clientset/versioned"
	kueueinformers "sigs.k8s.io/kueue/client-go/informers/externalversions"
)

const (
	defaultKueueQueuePrefix      = "oscar-cq"
	defaultKueueLocalQueuePrefix = "oscar-lq"
	defaultKueueAdmissionTimeout = 30 * time.Second
)

var (
	defaultCpuRequest    = resource.MustParse("0.2")
	defaultMemoryRequest = resource.MustParse("256Mi")
)

var KueueLogger = log.New(os.Stdout, "[KUEUE-SERVICE] ", log.Flags())

// EnsureKueueUserQueues makes sure the user ClusterQueue and the service LocalQueue exist with default quotas.
// It is idempotent and will no-op if Kueue is disabled.
func EnsureKueueUserQueues(ctx context.Context, cfg *types.Config, serviceNamespace, owner, serviceName string) error {
	if !cfg.KueueEnable {
		return nil
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("unable to build in-cluster config for kueue: %w", err)
	}

	kueueClient, err := kueueclientset.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("unable to create kueue client: %w", err)
	}

	flavorName := sanitizeKueueName(cfg.KueueDefaultFlavor)
	if err := ensureResourceFlavor(ctx, kueueClient, flavorName); err != nil {
		return fmt.Errorf("ensuring kueue ResourceFlavor: %w", err)
	}

	clusterQueueName := buildClusterQueueName(owner)
	if err := ensureClusterQueue(ctx, kueueClient, cfg, clusterQueueName, flavorName, owner); err != nil {
		return fmt.Errorf("ensuring kueue ClusterQueue: %w", err)
	}

	if err := ensureLocalQueue(ctx, kueueClient, serviceNamespace, serviceName, clusterQueueName, owner); err != nil {
		return fmt.Errorf("ensuring kueue LocalQueue: %w", err)
	}

	return nil
}

// CreateKueueUserQueuesIfDontExist creates the ClusterQueue for the user if it doesn't exist.
// It is idempotent and will no-op if Kueue is disabled.
func CreateKueueUserQueuesIfDontExist(cfg *types.Config, user string) error {
	if !cfg.KueueEnable {
		return nil
	}
	// Set empty Context
	ctx := context.TODO()

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("unable to build in-cluster config for kueue: %v", err)
	}

	kueueClient, err := kueueclientset.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("unable to create kueue client: %v", err)
	}
	// Check if the ClusterQueue for the user already exists, if not create it
	clusterQueueName := buildClusterQueueName(user)
	_, err = kueueClient.KueueV1beta2().ClusterQueues().Get(ctx, clusterQueueName, metav1.GetOptions{})
	if err != nil {
		flavorName := sanitizeKueueName(cfg.KueueDefaultFlavor)
		if err := ensureResourceFlavor(ctx, kueueClient, flavorName); err != nil {
			return fmt.Errorf("unable to ensure kueue ResourceFlavor: %v", err)
		}

		if err := ensureClusterQueue(ctx, kueueClient, cfg, clusterQueueName, flavorName, user); err != nil {
			return fmt.Errorf("unable to ensure kueue ClusterQueue: %v", err)
		}
	}
	return nil
}

func ensureResourceFlavor(ctx context.Context, kueueClient *kueueclientset.Clientset, flavorName string) error {
	_, err := kueueClient.KueueV1beta2().ResourceFlavors().Get(ctx, flavorName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	flavor := &kueuev1.ResourceFlavor{
		ObjectMeta: metav1.ObjectMeta{
			Name: flavorName,
			Labels: map[string]string{
				types.KueueOwnerLabel: defaultKueueQueuePrefix,
			},
		},
		Spec: kueuev1.ResourceFlavorSpec{
			NodeLabels: map[string]string{},
		},
	}

	_, err = kueueClient.KueueV1beta2().ResourceFlavors().Create(ctx, flavor, metav1.CreateOptions{})
	return err
}

func ensureClusterQueue(ctx context.Context, kueueClient *kueueclientset.Clientset, cfg *types.Config, cqName, flavorName, owner string) error {
	cpuQuota, err := resource.ParseQuantity(cfg.KueueDefaultCPU)
	if err != nil {
		return fmt.Errorf("invalid Kueue default CPU quota %q: %w", cfg.KueueDefaultCPU, err)
	}
	memoryQuota, err := resource.ParseQuantity(cfg.KueueDefaultMemory)
	if err != nil {
		return fmt.Errorf("invalid Kueue default memory quota %q: %w", cfg.KueueDefaultMemory, err)
	}
	// Parse the default ephemeral storage quota from config for per-user limits.
	ephemeralStorageQuota, err := resource.ParseQuantity(cfg.KueueDefaultEphemeralStorage)
	if err != nil {
		return fmt.Errorf("invalid Kueue default ephemeral storage quota %q: %w", cfg.KueueDefaultEphemeralStorage, err)
	}

	gpuQuota := resource.MustParse("0")
	if cfg.GPUAvailable {
		gpuQuota, err = resource.ParseQuantity(cfg.KueueDefaultGPU)
		if err != nil {
			return fmt.Errorf("invalid Kueue default GPU quota %q: %w", cfg.KueueDefaultGPU, err)
		}
	}

	cq := &kueuev1.ClusterQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name: cqName,
			Labels: map[string]string{
				types.KueueOwnerLabel: sanitizeKueueName(owner),
			},
		},
		Spec: kueuev1.ClusterQueueSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					namespaceOwnerHashLabel: ownerHash(owner),
				},
			},
			ResourceGroups: []kueuev1.ResourceGroup{
				{
					CoveredResources: []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory, v1.ResourceName("nvidia.com/gpu"), v1.ResourceEphemeralStorage},
					Flavors: []kueuev1.FlavorQuotas{
						{
							Name: kueuev1.ResourceFlavorReference(flavorName),
							Resources: []kueuev1.ResourceQuota{
								{
									Name:         v1.ResourceCPU,
									NominalQuota: cpuQuota,
								},
								{
									Name:         v1.ResourceMemory,
									NominalQuota: memoryQuota,
								},
								{
									Name:         v1.ResourceEphemeralStorage,
									NominalQuota: ephemeralStorageQuota,
								},
								{
									Name:         v1.ResourceName("nvidia.com/gpu"),
									NominalQuota: gpuQuota,
								},
							},
						},
					},
				},
			},
		},
	}

	current, err := kueueClient.KueueV1beta2().ClusterQueues().Get(ctx, cqName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = kueueClient.KueueV1beta2().ClusterQueues().Create(ctx, cq, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	// Reconcile quotas if missing/zero so queues are usable without manual patching.
	if current.Spec.NamespaceSelector == nil ||
		len(current.Spec.ResourceGroups) == 0 ||
		len(current.Spec.ResourceGroups[0].Flavors) == 0 ||
		len(current.Spec.ResourceGroups[0].Flavors[0].Resources) == 0 ||
		!reflect.DeepEqual(current.Spec.NamespaceSelector, cq.Spec.NamespaceSelector) {
		current.Spec.ResourceGroups = cq.Spec.ResourceGroups
		current.Spec.NamespaceSelector = cq.Spec.NamespaceSelector
		_, err = kueueClient.KueueV1beta2().ClusterQueues().Update(ctx, current, metav1.UpdateOptions{})
	}
	return err
}

func ensureLocalQueue(ctx context.Context, kueueClient *kueueclientset.Clientset, namespace, serviceName, clusterQueueName, owner string) error {
	lqName := BuildLocalQueueName(serviceName)
	lq, err := kueueClient.KueueV1beta2().LocalQueues(namespace).Get(ctx, lqName, metav1.GetOptions{})
	if err == nil {
		if string(lq.Spec.ClusterQueue) != clusterQueueName {
			lq.Spec.ClusterQueue = kueuev1.ClusterQueueReference(clusterQueueName)
			if lq.Labels == nil {
				lq.Labels = map[string]string{}
			}
			lq.Labels[types.KueueOwnerLabel] = sanitizeKueueName(owner)
			_, err = kueueClient.KueueV1beta2().LocalQueues(namespace).Update(ctx, lq, metav1.UpdateOptions{})
		}
		return err
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	localQueue := &kueuev1.LocalQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lqName,
			Namespace: namespace,
			Labels: map[string]string{
				types.KueueOwnerLabel: sanitizeKueueName(owner),
			},
		},
		Spec: kueuev1.LocalQueueSpec{
			ClusterQueue: kueuev1.ClusterQueueReference(clusterQueueName),
		},
	}
	_, err = kueueClient.KueueV1beta2().LocalQueues(namespace).Create(ctx, localQueue, metav1.CreateOptions{})
	return err
}

// DeleteKueueLocalQueue deletes the LocalQueue associated with a service. It does not delete the ClusterQueue.
func DeleteKueueLocalQueue(ctx context.Context, cfg *types.Config, namespace, serviceName string) error {
	if !cfg.KueueEnable {
		return nil
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("unable to build in-cluster config for kueue: %w", err)
	}

	kueueClient, err := kueueclientset.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("unable to create kueue client: %w", err)
	}

	err = kueueClient.KueueV1beta2().LocalQueues(namespace).Delete(ctx, BuildLocalQueueName(serviceName), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func buildClusterQueueName(owner string) string {
	return sanitizeKueueName(fmt.Sprintf("%s-%s", defaultKueueQueuePrefix, owner))
}

// BuildClusterQueueName returns the canonical ClusterQueue name for a given owner.
func BuildClusterQueueName(owner string) string {
	return buildClusterQueueName(owner)
}

// SanitizeKueueName exposes the internal sanitizer for consumers that must
// reference Kueue objects (e.g., when annotating Jobs with a LocalQueue name).
func SanitizeKueueName(value string) string {
	return sanitizeKueueName(value)
}

// BuildLocalQueueName builds the canonical LocalQueue name for a service.
func BuildLocalQueueName(serviceName string) string {
	return sanitizeKueueName(fmt.Sprintf("%s-%s", defaultKueueLocalQueuePrefix, serviceName))
}

func sanitizeKueueName(value string) string {
	value = strings.ToLower(value)

	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	clean := strings.Trim(b.String(), "-")
	if len(clean) > validation.DNS1123LabelMaxLength {
		clean = clean[:validation.DNS1123LabelMaxLength]
		clean = strings.TrimRight(clean, "-")
	}
	if clean == "" {
		return defaultKueueQueuePrefix
	}
	return clean
}

///------------Deployment--------------------------

func CreateWorkload(service types.Service, namespace string, cfg *types.Config, templateFunction func(types.Service, string, *types.Config) v1.PodTemplateSpec) bool {
	// TO DO
	workloadSpec := getWorkloadSpec(service, namespace, cfg, templateFunction)

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		KueueLogger.Printf("error building in-cluster config for kueue: %v", err)
		return false
	}
	kueueClient, err := kueueclientset.NewForConfig(restCfg)

	_, err2 := kueueClient.KueueV1beta2().Workloads(namespace).Create(context.TODO(), workloadSpec, metav1.CreateOptions{})
	if err2 != nil {
		KueueLogger.Printf("error creating workload for exposed service '%s': %v", service.Name, err2)
		return false
	}
	return true
}

func UpdateWorkload(service types.Service, namespace string, cfg *types.Config, templateFunction func(types.Service, string, *types.Config) v1.PodTemplateSpec) {
	DeleteWorkload(service.Name, namespace, cfg)
	CreateWorkload(service, namespace, cfg, templateFunction)
}

func DeleteWorkload(name string, namespace string, cfg *types.Config) bool {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		KueueLogger.Printf("error building in-cluster config for kueue: %v", err)
		return false
	}
	kueueClient, err := kueueclientset.NewForConfig(restCfg)

	err2 := kueueClient.KueueV1beta2().Workloads(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err2 != nil {
		KueueLogger.Printf("error deleting workload for exposed service '%s': %v", name, err2)
		return false
	}

	return true
}

func getWorkloadSpec(service types.Service, namespace string, cfg *types.Config, template func(types.Service, string, *types.Config) v1.PodTemplateSpec) *kueuev1.Workload {
	// TO DO
	// Create a new workload spec based on the service and config
	boolActive := true
	workload := &kueuev1.Workload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: namespace,
		},
		Spec: kueuev1.WorkloadSpec{
			Active:    &boolActive,
			QueueName: kueuev1.LocalQueueName(BuildLocalQueueName(service.Name)),
			PodSets: []kueuev1.PodSet{
				{
					Name:     "default",
					Count:    service.Expose.MinScale,
					Template: template(service, namespace, cfg),
				},
			},
		},
	}
	if len(service.CPU) > 0 && len(service.Memory) > 0 {
		cpu, err := resource.ParseQuantity(service.CPU)
		if err != nil {
			return nil
		}

		memory, err := resource.ParseQuantity(service.Memory)
		if err != nil {
			return nil
		}

		workload.Spec.PodSets[0].Template.Spec.Resources = &v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    cpu,
				v1.ResourceMemory: memory,
			},
		}
	}

	return workload
}

func getResourceOnlyWorkloadSpec(service *types.Service, cfg *types.Config, namespace, workloadName, localQueueName string) (*kueuev1.Workload, error) {
	serviceRequests, err := getServiceResourceRequests(service, cfg)
	if err != nil {
		return nil, err
	}
	var serviceReplicas int32 = 1

	if len(service.Expose.APIPort) > 0 && service.Expose.APIPort[0] != 0 && service.Expose.MinScale > 1 {
		serviceReplicas = service.Expose.MinScale
	} else if service.Synchronous.MinScale > 1 {
		if service.Synchronous.MinScale > math.MaxInt32 {
			return nil, fmt.Errorf("synchronous min_scale %d exceeds int32 range", service.Synchronous.MinScale)
		}
		serviceReplicas = int32(service.Synchronous.MinScale)
	}

	podSets := []kueuev1.PodSet{
		buildResourceCheckPodSet("oscar-service", serviceReplicas, serviceRequests),
	}

	kserveRequests, kserveReplicas, hasKservePodSet, err := getKserveResourceRequests(service, cfg)
	if err != nil {
		return nil, err
	}
	if hasKservePodSet {
		podSets = append(podSets, buildResourceCheckPodSet("kserve-service", kserveReplicas, kserveRequests))
	}

	boolActive := true
	workload := &kueuev1.Workload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workloadName,
			Namespace: namespace,
		},
		Spec: kueuev1.WorkloadSpec{
			Active:    &boolActive,
			QueueName: kueuev1.LocalQueueName(localQueueName),
			PodSets:   podSets,
		},
	}

	return workload, nil
}

func buildResourceCheckPodSet(name string, replicas int32, requests v1.ResourceList) kueuev1.PodSet {
	return kueuev1.PodSet{
		Name:  kueuev1.PodSetReference(name),
		Count: replicas,
		Template: v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "resource-check",
						Resources: v1.ResourceRequirements{
							Requests: requests,
							Limits:   requests,
						},
					},
				},
			},
		},
	}
}

func getServiceResourceRequests(service *types.Service, cfg *types.Config) (v1.ResourceList, error) {
	requests := v1.ResourceList{}
	var cpuQty resource.Quantity = defaultCpuRequest
	var memoryQty resource.Quantity = defaultMemoryRequest

	if len(service.CPU) > 0 {
		parsedCPU, err := resource.ParseQuantity(service.CPU)
		if err != nil {
			return nil, fmt.Errorf("invalid service CPU %q: %w", service.CPU, err)
		}
		cpuQty = parsedCPU
	}
	if len(service.Memory) > 0 {
		parsedMemory, err := resource.ParseQuantity(service.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid service memory %q: %w", service.Memory, err)
		}
		memoryQty = parsedMemory
	}

	requests[v1.ResourceCPU] = cpuQty
	requests[v1.ResourceMemory] = memoryQty

	// Include ephemeral storage request in workload resources for Kueue quota enforcement.
	if len(service.EphemeralStorageRequest) > 0 {
		parsedEphemeral, err := resource.ParseQuantity(service.EphemeralStorageRequest)
		if err != nil {
			return nil, fmt.Errorf("invalid service ephemeral storage %q: %w", service.EphemeralStorageRequest, err)
		}
		requests[v1.ResourceEphemeralStorage] = parsedEphemeral
	}

	if service.EnableGPU {
		gpu, err := resource.ParseQuantity("1")
		if err != nil {
			return nil, fmt.Errorf("invalid service GPU quantity: %w", err)
		}
		requests["nvidia.com/gpu"] = gpu
	}

	if service.EnableSGX {
		sgx, err := resource.ParseQuantity("1")
		if err != nil {
			return nil, fmt.Errorf("invalid service SGX quantity: %w", err)
		}
		requests["sgx.intel.com/enclave"] = sgx
	}

	if len(requests) == 0 {
		return nil, fmt.Errorf("service %q has no resource requests to validate", service.Name)
	}

	return requests, nil
}

func getKserveResourceRequests(service *types.Service, cfg *types.Config) (v1.ResourceList, int32, bool, error) {
	isKserveService := IsKserveService(service) && IsKserveSupported(cfg)
	if !isKserveService {
		return nil, 0, false, nil
	}

	requests := v1.ResourceList{}
	cpuQty := defaultKserveCpuRequest
	memoryQty := defaultKserveMemoryRequest

	if len(service.Kserve.CPU) > 0 {
		parsedCPU, err := resource.ParseQuantity(service.Kserve.CPU)
		if err != nil {
			return nil, 0, false, fmt.Errorf("invalid KServe service CPU %q: %w", service.Kserve.CPU, err)
		}
		cpuQty = parsedCPU
	}
	if len(service.Kserve.Memory) > 0 {
		parsedMemory, err := resource.ParseQuantity(service.Kserve.Memory)
		if err != nil {
			return nil, 0, false, fmt.Errorf("invalid KServe service memory %q: %w", service.Kserve.Memory, err)
		}
		memoryQty = parsedMemory
	}

	requests[v1.ResourceCPU] = cpuQty
	requests[v1.ResourceMemory] = memoryQty

	if service.Kserve.EnableGPU {
		gpu, err := resource.ParseQuantity("1")
		if err != nil {
			return nil, 0, false, fmt.Errorf("invalid KServe service GPU quantity: %w", err)
		}
		requests["nvidia.com/gpu"] = gpu
	}

	var kserveMinScale int32 = 1
	if service.Kserve.MinScale > 1 {
		kserveMinScale = service.Kserve.MinScale
	}

	// resoures, replicas, hasKservePodSet, err
	return requests, kserveMinScale, true, nil
}

// checkQueueReferences validates that the specified LocalQueue and ClusterQueue exist and are correctly linked.
func checkQueueReferences(ctx context.Context, kueueClient *kueueclientset.Clientset, namespace, localQueueName, clusterQueueName string) error {
	if strings.TrimSpace(localQueueName) == "" || strings.TrimSpace(clusterQueueName) == "" {
		return fmt.Errorf("localQueueName and clusterQueueName are required")
	}

	lq, err := kueueClient.KueueV1beta2().LocalQueues(namespace).Get(ctx, localQueueName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get LocalQueue %q in namespace %q: %w", localQueueName, namespace, err)
	}

	if string(lq.Spec.ClusterQueue) != clusterQueueName {
		return fmt.Errorf("LocalQueue %q points to ClusterQueue %q, expected %q", localQueueName, lq.Spec.ClusterQueue, clusterQueueName)
	}

	_, err = kueueClient.KueueV1beta2().ClusterQueues().Get(ctx, clusterQueueName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get ClusterQueue %q: %w", clusterQueueName, err)
	}

	return nil
}

func buildVerificationWorkloadName(serviceName string) string {
	suffix := fmt.Sprintf("-%d", time.Now().UnixNano())
	base := sanitizeKueueName(fmt.Sprintf("verify-%s", serviceName))

	maxBaseLen := validation.DNS1123LabelMaxLength - len(suffix)
	if maxBaseLen < 1 {
		maxBaseLen = 1
	}

	if len(base) > maxBaseLen {
		base = strings.TrimRight(base[:maxBaseLen], "-")
	}
	if base == "" {
		base = "verify"
	}

	return base + suffix
}

func CheckWorkloadAdmited(service types.Service, namespace string, cfg *types.Config, kubeClientset kubernetes.Interface, templateFunction func(types.Service, string, *types.Config) *apps.Deployment) error {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		KueueLogger.Printf("error building in-cluster config for kueue: %v", err)
		return err
	}

	kueueClient, err := kueueclientset.NewForConfig(restCfg)
	if err != nil {
		KueueLogger.Printf("error building kueue clientset: %v", err)
		return err
	}
	factory := kueueinformers.NewSharedInformerFactory(kueueClient, 0)
	workloadsInformer := factory.Kueue().V1beta2().Workloads().Informer()

	resource, err := workloadsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			_, ok := newObj.(*kueuev1.Workload)
			if !ok {
				KueueLogger.Printf("error: unexpected type in workload informer")
				return
			}
		},
	})
	if err != nil {
		KueueLogger.Printf("error adding event handler to workload informer: %v, %v", err, resource)
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	factory.Start(stopCh)

	if !cache.WaitForCacheSync(stopCh, workloadsInformer.HasSynced) {
		return fmt.Errorf("failed to sync workload informer")
	}
	obj, exists, err := workloadsInformer.GetIndexer().GetByKey(namespace + "/" + service.Name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("workload not found")
	}

	wl := obj.(*kueuev1.Workload)
	admitted := false
	for _, c := range wl.Status.Conditions {
		if c.Type == kueuev1.WorkloadAdmitted &&
			c.Status == metav1.ConditionTrue {
			admitted = true
			break
		}
	}
	if !admitted {
		DeleteWorkload(service.Name, namespace, cfg)
		return fmt.Errorf("workload for exposed service '%s' is NOT admitted", service.Name)
	} else {
		KueueLogger.Printf("workload for exposed service '%s' is admitted", service.Name)
		deployment := templateFunction(service, namespace, cfg) //getDeploymentSpec
		if SecretExists(service.Name, namespace, kubeClientset) {
			fmt.Println("exist")
			deployment.Spec.Template.Spec.Containers[0].EnvFrom = []v1.EnvFromSource{
				{
					SecretRef: &v1.SecretEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: service.Name,
						},
					},
				},
			}
		}
		deployment.Spec.Replicas = &service.Expose.MinScale
		_, err := kubeClientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			KueueLogger.Printf("error updating deployment for exposed service '%s': %v", service.Name, err)
		}
	}

	return nil
}

// ------------Knative--------------------------
func VerifyWorkload(service types.Service, namespace string, cfg *types.Config) bool {
	if service.Expose.MinScale == 0 {
		service.Expose.MinScale = 1
	}
	if !CreateWorkload(service, namespace, cfg, getPodTemplateSpec) {
		return false
	}
	check := onlyCheckWorkloadAdmited(service.Name, defaultKueueAdmissionTimeout)
	delete := DeleteWorkload(service.Name, namespace, cfg)
	return check && delete
}

// VerifyWorkloadByResources validates a temporary workload using only service resources
// and explicit LocalQueue/ClusterQueue references.
func VerifyWorkloadByResources(service types.Service, cfg *types.Config) bool {
	if cfg == nil {
		KueueLogger.Printf("invalid nil config while verifying workload for service '%s'", service.Name)
		return false
	}
	if !cfg.KueueEnable {
		return true
	}
	if service.Expose.MinScale == 0 {
		service.Expose.MinScale = 1
	}

	localQueueName := BuildLocalQueueName(service.Name)
	clusterQueueName := BuildClusterQueueName(service.Owner)
	namespace := service.Namespace

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		KueueLogger.Printf("error building in-cluster config for kueue: %v", err)
		return false
	}

	kueueClient, err := kueueclientset.NewForConfig(restCfg)
	if err != nil {
		KueueLogger.Printf("error building kueue clientset: %v", err)
		return false
	}

	if err := checkQueueReferences(context.TODO(), kueueClient, namespace, localQueueName, clusterQueueName); err != nil {
		KueueLogger.Printf("invalid queue references while verifying workload for service '%s': %v", service.Name, err)
		return false
	}

	workloadName := buildVerificationWorkloadName(service.Name)
	workloadSpec, err := getResourceOnlyWorkloadSpec(&service, cfg, namespace, workloadName, localQueueName)
	if err != nil {
		KueueLogger.Printf("error building resource-only workload for service '%s': %v", service.Name, err)
		return false
	}

	_, err = kueueClient.KueueV1beta2().Workloads(namespace).Create(context.TODO(), workloadSpec, metav1.CreateOptions{})
	if err != nil {
		KueueLogger.Printf("error creating resource-only workload for service '%s': %v", service.Name, err)
		return false
	}

	check := onlyCheckWorkloadAdmited(workloadName, 4*time.Second)
	delete := DeleteWorkload(workloadName, namespace, cfg)
	return check && delete
}

func getPodTemplateSpec(service types.Service, namespace string, cfg *types.Config) v1.PodTemplateSpec {
	resources, err := types.CreateResources(&service)
	if err != nil {
		KueueLogger.Printf("error creating resources for exposed service '%s': %v", service.Name, err)
	}
	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": service.Name,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:      service.Name,
					Image:     service.Image,
					Resources: resources,
				},
			},
		},
	}
}

func onlyCheckWorkloadAdmited(serviceName string, timeout time.Duration) bool {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		KueueLogger.Printf("error building in-cluster config for kueue: %v", err)
		return false
	}
	kueueClient, err := kueueclientset.NewForConfig(restCfg)
	if err != nil {
		KueueLogger.Printf("error building kueue clientset: %v", err)
		return false
	}
	factory := kueueinformers.NewSharedInformerFactory(kueueClient, 0)
	workloadsInformer := factory.Kueue().V1beta2().Workloads().Informer()
	admissionChan := make(chan bool, 1)
	stopCh := make(chan struct{})
	defer close(stopCh)

	resource, err := workloadsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			newWL := newObj.(*kueuev1.Workload)
			if newWL.Name != serviceName {
				return
			}
			if workloadIsAdmitted(newWL) {
				KueueLogger.Printf("Workload %s admitted", serviceName)
				select {
				case admissionChan <- true:
				default:
				}
				return
			}
		},
	})
	if err != nil {
		KueueLogger.Printf("error adding event handler to workload informer: %v, %v", err, resource)
	}

	factory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, workloadsInformer.HasSynced) {
		KueueLogger.Printf("timed out syncing workload informer for service '%s'", serviceName)
		return false
	}

	for _, obj := range workloadsInformer.GetStore().List() {
		if wl, ok := obj.(*kueuev1.Workload); ok && wl.Name == serviceName && workloadIsAdmitted(wl) {
			KueueLogger.Printf("Workload %s admitted", serviceName)
			return true
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case admitted := <-admissionChan:
		return admitted
	case <-timer.C:
		KueueLogger.Printf("timed out waiting for Kueue admission for service '%s'", serviceName)
		return false
	}
}

func workloadIsAdmitted(wl *kueuev1.Workload) bool {
	for _, cond := range wl.Status.Conditions {
		if cond.Type == kueuev1.WorkloadAdmitted && cond.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}
