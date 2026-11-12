package utils

import (
	"testing"

	"k8s.io/client-go/rest"
)

func TestKubernetesConfigDefaults(t *testing.T) {
	original := &rest.Config{
		QPS:       5,
		Burst:     10,
		UserAgent: "test-agent",
	}
	config := configureKubernetesConfig(original)

	if config.QPS != kubernetesQPS {
		t.Errorf("QPS = %v, want %v", config.QPS, kubernetesQPS)
	}
	if config.Burst != kubernetesBurst {
		t.Errorf("Burst = %v, want %v", config.Burst, kubernetesBurst)
	}
	if config.UserAgent != kubernetesUserAgent {
		t.Errorf("UserAgent = %q, want %q", config.UserAgent, kubernetesUserAgent)
	}
	if config.RateLimiter == nil {
		t.Error("RateLimiter is nil, want a configured rate limiter")
	}
	if original.QPS != 5 || original.Burst != 10 || original.UserAgent != "test-agent" {
		t.Errorf("configureKubernetesConfig mutated its input: %+v", original)
	}
}
