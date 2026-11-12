package utils

import (
	"fmt"
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	kubernetesQPS       float32 = 120 // 120 requests per second
	kubernetesBurst             = 180 // 180 requests in a burst
	kubernetesUserAgent         = "oscar-manager"
)

var (
	kubernetesConfigOnce sync.Once
	kubernetesConfig     *rest.Config
	kubernetesConfigErr  error
)

func configureKubernetesConfig(config *rest.Config) *rest.Config {
	config = rest.CopyConfig(config)
	config.QPS = kubernetesQPS
	config.Burst = kubernetesBurst
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(
		kubernetesQPS,
		kubernetesBurst,
	)
	config.UserAgent = kubernetesUserAgent
	return config
}

// KubernetesConfig returns a copy of the process-wide Kubernetes client config.
func KubernetesConfig() (*rest.Config, error) {
	kubernetesConfigOnce.Do(func() {
		config, err := rest.InClusterConfig()
		if err != nil {
			kubernetesConfigErr = fmt.Errorf("loading Kubernetes config: %w", err)
			return
		}
		kubernetesConfig = configureKubernetesConfig(config)
	})
	if kubernetesConfigErr != nil {
		return nil, kubernetesConfigErr
	}

	return rest.CopyConfig(kubernetesConfig), nil
}

func NewClients() (
	rest.Config,
	kubernetes.Interface,
	dynamic.Interface,
	error,
) {
	config, err := KubernetesConfig()
	if err != nil {
		return rest.Config{}, nil, nil, err
	}

	// Create the Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return rest.Config{}, nil, nil, fmt.Errorf(
			"creating Kubernetes clientset: %w", err,
		)
	}

	// Create the dynamic client for interacting with custom resources
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return rest.Config{}, nil, nil, fmt.Errorf(
			"creating dynamic Kubernetes client: %w", err,
		)
	}

	return *config, clientset, dynamicClient, nil
}
