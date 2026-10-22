package backends

import (
	"errors"
	"strings"
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCheckAdditionalConfigImageRepositories(t *testing.T) {
	cfg := &types.Config{Namespace: "oscar", AdditionalConfigPath: "config.yaml"}
	tests := []struct {
		name    string
		policy  string
		image   string
		allowed bool
	}{
		{"owner image without tag", `["ghcr.io/grycap"]`, "ghcr.io/grycap/imagemagick", true},
		{"owner image with tag", `["ghcr.io/grycap"]`, "ghcr.io/grycap/imagemagick:latest", true},
		{"owner image with digest", `["ghcr.io/grycap"]`, "ghcr.io/grycap/imagemagick@sha256:" + strings.Repeat("a", 64), true},
		{"different owner", `["ghcr.io/grycap"]`, "ghcr.io/grycap-malicious/payload:latest", false},
		{"embedded path on foreign registry", `["ghcr.io/grycap"]`, "evil.example/ghcr.io/grycap/payload:latest", false},
		{"different registry", `["ghcr.io/grycap"]`, "evil.example/grycap/payload", false},
		{"docker hub", `["ghcr.io/grycap"]`, "bgeedb/bgee_r", false},
		{"multiple allowed owners", `["ghcr.io/grycap","registry.example.org/team"]`, "registry.example.org/team/app:1", true},
		{"specific repository", `["ghcr.io/grycap/imagemagick"]`, "ghcr.io/grycap/imagemagick:latest", true},
		{"specific repository does not match suffix", `["ghcr.io/grycap/imagemagick"]`, "ghcr.io/grycap/imagemagick-other:latest", false},
		{"specific repository allows descendants", `["ghcr.io/grycap"]`, "ghcr.io/grycap/team/app:latest", true},
		{"empty policy is unrestricted", `[]`, "bgeedb/bgee_r", true},
		{"malformed image cannot pass", `["ghcr.io/grycap"]`, "ghcr.io/grycap/BadImage", false},
		{"malformed configured repository cannot match", `["grycap"]`, "ghcr.io/grycap/imagemagick", false},
		{"empty configured repository cannot match", `[""]`, "ghcr.io/grycap/imagemagick", false},
		{"invalid entry after matching entry cannot bypass", `["ghcr.io/grycap",""]`, "ghcr.io/grycap/imagemagick", false},
		{"invalid policy JSON", `["ghcr.io/grycap"`, "ghcr.io/grycap/imagemagick", false},
		{"null policy is not an empty allowlist", `null`, "ghcr.io/grycap/imagemagick", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: cfg.AdditionalConfigPath, Namespace: cfg.Namespace},
				Data:       map[string]string{types.AIR: tt.policy},
			})
			// Service ConfigMap arguments differ from the additional policy ConfigMap.
			err := checkAdditionalConfig(ConfigMapNameOSCAR, "oscar-svc", types.Service{Image: tt.image}, cfg, client)
			if (err == nil) != tt.allowed {
				t.Fatalf("image %q with policy %s: got error %v, want allowed=%v", tt.image, tt.policy, err, tt.allowed)
			}
		})
	}
}

func TestCheckAdditionalConfigReadErrors(t *testing.T) {
	cfg := &types.Config{Namespace: "oscar", AdditionalConfigPath: "config.yaml"}
	client := fake.NewSimpleClientset()
	// No ConfigMap means that the optional policy has not been configured.
	if err := checkAdditionalConfig(ConfigMapNameOSCAR, "oscar-svc", types.Service{Image: "any/image"}, cfg, client); err != nil {
		t.Fatalf("absent policy should not restrict images: %v", err)
	}

	client.PrependReactor("get", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("API temporarily unavailable")
	})
	if err := checkAdditionalConfig(ConfigMapNameOSCAR, "oscar-svc", types.Service{Image: "any/image"}, cfg, client); err == nil {
		t.Fatal("ConfigMap read failure must not disable image policy")
	}
}
