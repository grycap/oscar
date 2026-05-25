package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	autosv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestMakeStopExposedServiceHandlerScalesDeploymentToZero(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = exposedLifecycleTestService()

	replicas := int32(3)
	minReplicas := int32(1)
	kubeClientset := testclient.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-dpl",
				Namespace: "ns",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: types.ContainerName}},
					},
				},
			},
		},
		&autosv1.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-hpa",
				Namespace: "ns",
			},
			Spec: autosv1.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autosv1.CrossVersionObjectReference{
					Kind: "Deployment",
					Name: "svc-dpl",
				},
				MinReplicas: &minReplicas,
				MaxReplicas: 5,
			},
		},
	)

	r := gin.Default()
	r.POST("/system/services/:serviceName/stop", MakeStopExposedServiceHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/services/svc/stop", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State != types.DeploymentStateStopped {
		t.Fatalf("expected stopped state, got %s", response.State)
	}

	deployment, err := kubeClientset.AppsV1().Deployments("ns").Get(t.Context(), "svc-dpl", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 0 {
		t.Fatalf("expected replicas 0, got %v", deployment.Spec.Replicas)
	}
	if deployment.Annotations[lifecyclePreviousReplicasAnnotation] != "3" {
		t.Fatalf("expected previous replicas annotation to be 3, got %q", deployment.Annotations[lifecyclePreviousReplicasAnnotation])
	}
	if _, err := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers("ns").Get(t.Context(), "svc-hpa", metav1.GetOptions{}); err == nil {
		t.Fatal("expected HPA to be deleted")
	}
}

func TestMakeStartExposedServiceHandlerRestoresReplicasAndHPA(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = exposedLifecycleTestService()

	replicas := int32(0)
	kubeClientset := testclient.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dpl",
			Namespace: "ns",
			Annotations: map[string]string{
				lifecyclePreviousReplicasAnnotation: "3",
				lifecycleStoppedAnnotation:          "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: types.ContainerName}},
				},
			},
		},
	})

	r := gin.Default()
	r.POST("/system/services/:serviceName/start", MakeStartExposedServiceHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/services/svc/start", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	deployment, err := kubeClientset.AppsV1().Deployments("ns").Get(t.Context(), "svc-dpl", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 {
		t.Fatalf("expected replicas 3, got %v", deployment.Spec.Replicas)
	}
	if deployment.Annotations[lifecycleStoppedAnnotation] != "false" {
		t.Fatalf("expected stopped annotation false, got %q", deployment.Annotations[lifecycleStoppedAnnotation])
	}
	hpa, err := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers("ns").Get(t.Context(), "svc-hpa", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected HPA to be recreated: %v", err)
	}
	if hpa.Spec.MaxReplicas != 5 {
		t.Fatalf("expected max replicas 5, got %d", hpa.Spec.MaxReplicas)
	}
	secret, err := kubeClientset.CoreV1().Secrets("ns").Get(t.Context(), "svc-auth-expose", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected auth secret to be recreated: %v", err)
	}
	if secret.StringData["auth"] == "" && len(secret.Data["auth"]) == 0 {
		t.Fatal("expected auth secret to contain basic auth data")
	}
}

func TestMakeRestartExposedServiceHandlerAnnotatesPodTemplate(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = exposedLifecycleTestService()

	replicas := int32(1)
	kubeClientset := testclient.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dpl",
			Namespace: "ns",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	})

	r := gin.Default()
	r.POST("/system/services/:serviceName/restart", MakeRestartExposedServiceHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/services/svc/restart", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	deployment, err := kubeClientset.AppsV1().Deployments("ns").Get(t.Context(), "svc-dpl", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if deployment.Spec.Template.Annotations[lifecycleRestartedAtAnnotation] == "" {
		t.Fatal("expected restart annotation on pod template")
	}
}

func TestMakeStopExposedServiceHandlerRequiresOwnerForBearer(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = exposedLifecycleTestService()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "other@example.org")
		c.Next()
	})
	r.POST("/system/services/:serviceName/stop", MakeStopExposedServiceHandler(back, testclient.NewSimpleClientset(), &types.Config{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/services/svc/stop", nil)
	req.Header.Set("Authorization", "Bearer token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMakeStopExposedServiceHandlerRejectsNonExposedService(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
		Owner:     "owner@example.org",
	}

	r := gin.Default()
	r.POST("/system/services/:serviceName/stop", MakeStopExposedServiceHandler(back, testclient.NewSimpleClientset(), &types.Config{}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/services/svc/stop", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func exposedLifecycleTestService() *types.Service {
	return &types.Service{
		Name:      "svc",
		Namespace: "ns",
		Owner:     "owner@example.org",
		Token:     "service-token",
		Expose: types.Expose{
			APIPort:      8080,
			MinScale:     1,
			MaxScale:     5,
			CpuThreshold: 80,
			SetAuth:      true,
		},
	}
}
