package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
	knapis "knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	serving "knative.dev/serving/pkg/apis/serving"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
)

type fakeRuntimeServiceBackend struct {
	*backends.FakeBackend
	runtimeService *knv1.Service
	runtimeErr     error
}

func (f *fakeRuntimeServiceBackend) GetRuntimeService(namespace, name string) (*knv1.Service, error) {
	return f.runtimeService, f.runtimeErr
}

func TestMakeGetDeploymentStatusHandlerReadyExposed(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name: "svc",
		Expose: types.Expose{
			APIPort: 8080,
		},
		Namespace: "ns",
	}

	replicas := int32(2)
	now := metav1.NewTime(time.Now().UTC())
	kubeClientset := testclient.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dlp",
			Namespace: "ns",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          2,
			AvailableReplicas: 2,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:           appsv1.DeploymentAvailable,
					Status:         corev1.ConditionTrue,
					Reason:         "MinimumReplicasAvailable",
					LastUpdateTime: now,
				},
			},
		},
	})

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.State != types.DeploymentStateReady {
		t.Fatalf("expected ready state, got %s", response.State)
	}
	if response.ActiveInstances != 2 {
		t.Fatalf("expected 2 active instances, got %d", response.ActiveInstances)
	}
	if response.ResourceKind != types.DeploymentResourceKindExposedService {
		t.Fatalf("expected exposed_service resource kind, got %s", response.ResourceKind)
	}
}

func TestMakeGetDeploymentStatusHandlerUnavailable(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
	}

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, testclient.NewSimpleClientset(), &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.State != types.DeploymentStateUnavailable {
		t.Fatalf("expected unavailable state, got %s", response.State)
	}
}

func TestMakeGetDeploymentStatusHandlerInvalidServiceName(t *testing.T) {
	back := backends.MakeFakeBackend()

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, testclient.NewSimpleClientset(), &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc}/deployment", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMakeGetDeploymentStatusHandlerUnavailableForPodBackedRuntime(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
	}

	kubeClientset := testclient.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-job-pod",
			Namespace: "ns",
			Labels: map[string]string{
				types.ServiceLabel: "svc",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	})

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State != types.DeploymentStateUnavailable {
		t.Fatalf("expected unavailable state, got %s", response.State)
	}
	if response.ActiveInstances != 0 || response.AffectedInstances != 0 {
		t.Fatalf("expected zero instance counters for unavailable runtime, got active=%d affected=%d", response.ActiveInstances, response.AffectedInstances)
	}
}

func TestMakeGetDeploymentStatusHandlerDegraded(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name: "svc",
		Expose: types.Expose{
			APIPort: 8080,
		},
		Namespace: "ns",
	}

	replicas := int32(2)
	now := metav1.NewTime(time.Now().UTC())
	kubeClientset := testclient.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dlp",
			Namespace: "ns",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          2,
			AvailableReplicas: 1,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:           appsv1.DeploymentAvailable,
					Status:         corev1.ConditionTrue,
					Reason:         "MinimumReplicasAvailable",
					LastUpdateTime: now,
				},
			},
		},
	})

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State != types.DeploymentStateDegraded {
		t.Fatalf("expected degraded state, got %s", response.State)
	}
	if response.AffectedInstances != 1 {
		t.Fatalf("expected 1 affected instance, got %d", response.AffectedInstances)
	}
	if response.ActiveInstances != 2 {
		t.Fatalf("expected 2 observed active instances, got %d", response.ActiveInstances)
	}
}

func TestMakeGetDeploymentStatusHandlerPending(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name: "svc",
		Expose: types.Expose{
			APIPort: 8080,
		},
		Namespace: "ns",
	}

	replicas := int32(1)
	now := metav1.NewTime(time.Now().UTC())
	kubeClientset := testclient.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dlp",
			Namespace: "ns",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          0,
			AvailableReplicas: 0,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:           appsv1.DeploymentProgressing,
					Status:         corev1.ConditionTrue,
					Reason:         "ReplicaSetUpdated",
					LastUpdateTime: now,
				},
			},
		},
	})

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State != types.DeploymentStatePending {
		t.Fatalf("expected pending state, got %s", response.State)
	}
	if response.ActiveInstances != 0 || response.AffectedInstances != 0 {
		t.Fatalf("expected zero observed instance counters during pending startup, got active=%d affected=%d", response.ActiveInstances, response.AffectedInstances)
	}
}

func TestMakeGetDeploymentStatusHandlerFailed(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name: "svc",
		Expose: types.Expose{
			APIPort: 8080,
		},
		Namespace: "ns",
	}

	replicas := int32(1)
	now := metav1.NewTime(time.Now().UTC())
	kubeClientset := testclient.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dlp",
			Namespace: "ns",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          0,
			AvailableReplicas: 0,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:           appsv1.DeploymentReplicaFailure,
					Status:         corev1.ConditionTrue,
					Reason:         "FailedCreate",
					LastUpdateTime: now,
				},
			},
		},
	})

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State != types.DeploymentStateFailed {
		t.Fatalf("expected failed state, got %s", response.State)
	}
	if response.ActiveInstances != 0 || response.AffectedInstances != 0 {
		t.Fatalf("expected zero observed instance counters for failed deployment without active replicas, got active=%d affected=%d", response.ActiveInstances, response.AffectedInstances)
	}
}

func TestMakeGetDeploymentStatusHandlerKnativeUsesKnativePods(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC())
	back := &fakeRuntimeServiceBackend{
		FakeBackend: backends.MakeFakeBackend(),
		runtimeService: &knv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc",
				Namespace: "ns",
			},
			Status: knv1.ServiceStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{
						{
							Type:               knapis.ConditionReady,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: knapis.VolatileTime{Inner: now},
						},
					},
				},
			},
		},
	}
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
	}

	kubeClientset := testclient.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "knative-runtime-pod",
				Namespace: "ns",
				Labels: map[string]string{
					serving.ServiceLabelKey: "svc",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "async-job-pod",
				Namespace: "ns",
				Labels: map[string]string{
					types.ServiceLabel: "svc",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodFailed,
			},
		},
	)

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State != types.DeploymentStateReady {
		t.Fatalf("expected ready state, got %s", response.State)
	}
	if response.ActiveInstances != 1 || response.AffectedInstances != 0 {
		t.Fatalf("expected knative runtime counters only, got active=%d affected=%d", response.ActiveInstances, response.AffectedInstances)
	}
}

func TestMakeGetDeploymentStatusHandlerKnativeReadyDuringScaleUpStaysReady(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC())
	back := &fakeRuntimeServiceBackend{
		FakeBackend: backends.MakeFakeBackend(),
		runtimeService: &knv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc",
				Namespace: "ns",
			},
			Status: knv1.ServiceStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{
						{
							Type:               knapis.ConditionReady,
							Status:             corev1.ConditionTrue,
							Reason:             "Ready",
							Message:            "Service is ready while scaling.",
							LastTransitionTime: knapis.VolatileTime{Inner: now},
						},
					},
				},
			},
		},
	}
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
	}

	kubeClientset := testclient.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ready-pod",
				Namespace: "ns",
				Labels: map[string]string{
					serving.ServiceLabelKey: "svc",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scaling-pod",
				Namespace: "ns",
				Labels: map[string]string{
					serving.ServiceLabelKey: "svc",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		},
	)

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State != types.DeploymentStateReady {
		t.Fatalf("expected ready state during normal knative scale-up, got %s", response.State)
	}
	if response.ActiveInstances != 2 || response.AffectedInstances != 0 {
		t.Fatalf("expected ready knative service not to report affected instances during autoscaling, got active=%d affected=%d", response.ActiveInstances, response.AffectedInstances)
	}
}

func TestMakeGetDeploymentStatusHandlerKnativeUsesSpecificConfigurationFailureReason(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC())
	back := &fakeRuntimeServiceBackend{
		FakeBackend: backends.MakeFakeBackend(),
		runtimeService: &knv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc",
				Namespace: "ns",
			},
			Status: knv1.ServiceStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{
						{
							Type:               knv1.ServiceConditionConfigurationsReady,
							Status:             corev1.ConditionFalse,
							Reason:             "RevisionFailed",
							Message:            "Revision \"svc-00001\" failed with message: Unable to fetch image \"fake.com/fake/ubuntu\": failed to resolve image to digest: GET https://www.fake.com/v2/: unexpected status code 404 Not Found\n<!DOCTYPE html>",
							LastTransitionTime: knapis.VolatileTime{Inner: now},
						},
						{
							Type:               knapis.ConditionReady,
							Status:             corev1.ConditionFalse,
							Reason:             "RevisionMissing",
							Message:            "Configuration \"svc\" does not have any ready Revision.",
							LastTransitionTime: knapis.VolatileTime{Inner: now},
						},
					},
				},
			},
		},
	}
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
	}

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment", MakeGetDeploymentStatusHandler(back, testclient.NewSimpleClientset(), &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment", nil)
	r.ServeHTTP(w, req)

	var response types.ServiceDeploymentStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.State != types.DeploymentStateFailed {
		t.Fatalf("expected failed state, got %s", response.State)
	}
	if strings.Contains(response.Reason, "does not have any ready Revision") {
		t.Fatalf("expected specific configuration failure reason, got %q", response.Reason)
	}
	if !strings.Contains(response.Reason, "Unable to fetch image") {
		t.Fatalf("expected image fetch cause in reason, got %q", response.Reason)
	}
	if strings.Contains(response.Reason, "<!DOCTYPE html>") {
		t.Fatalf("expected sanitized reason without HTML payload, got %q", response.Reason)
	}
}

func TestMakeGetDeploymentLogsHandlerKnativeRuntime(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC())
	back := &fakeRuntimeServiceBackend{
		FakeBackend: backends.MakeFakeBackend(),
		runtimeService: &knv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc",
				Namespace: "ns",
			},
			Status: knv1.ServiceStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{
						{
							Type:               knapis.ConditionReady,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: knapis.VolatileTime{Inner: now},
						},
					},
				},
			},
		},
	}
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
	}

	kubeObjects := []runtime.Object{
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-pod",
				Namespace: "ns",
				Labels: map[string]string{
					serving.ServiceLabelKey: "svc",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	kubeClientset := testclient.NewSimpleClientset(kubeObjects...)

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment/logs", MakeGetDeploymentLogsHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment/logs?timestamps=true", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response types.DeploymentLogStream
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.Available {
		t.Fatalf("expected logs to be available, got unavailable with message %q", response.Message)
	}
	if len(response.Entries) == 0 {
		t.Fatal("expected at least one log entry")
	}
}

func TestMakeGetDeploymentLogsHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
	}

	kubeObjects := []runtime.Object{
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-pod",
				Namespace: "ns",
				Labels: map[string]string{
					types.ServiceLabel: "svc",
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	kubeClientset := testclient.NewSimpleClientset(kubeObjects...)

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment/logs", MakeGetDeploymentLogsHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment/logs?timestamps=true", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response types.DeploymentLogStream
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Available {
		t.Fatalf("expected non-knative pod-backed runtime to remain unavailable, got entries=%d", len(response.Entries))
	}
}

func TestMakeGetDeploymentLogsHandlerFallback(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name: "svc",
		Expose: types.Expose{
			APIPort: 8080,
		},
		Namespace: "ns",
	}

	kubeClientset := testclient.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-failed-pod",
			Namespace: "ns",
			Labels: map[string]string{
				"app": "oscar-svc-exp-svc",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	})

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment/logs", MakeGetDeploymentLogsHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment/logs", nil)
	r.ServeHTTP(w, req)

	var response types.DeploymentLogStream
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Available {
		t.Fatalf("expected fallback logs to be available, got unavailable with message %q", response.Message)
	}
	if response.Message == "" {
		t.Fatal("expected fallback message to explain last-attempt logs")
	}
}

func TestMakeGetDeploymentLogsHandlerUnauthorized(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:         "svc",
		Namespace:    "ns",
		Owner:        "owner@example.org",
		AllowedUsers: []string{"allowed@example.org"},
	}
	kubeClientset := testclient.NewSimpleClientset()

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "blocked@example.org")
		c.Next()
	})
	r.GET("/system/services/:serviceName/deployment/logs", MakeGetDeploymentLogsHandler(back, kubeClientset, &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment/logs", nil)
	req.Header.Set("Authorization", "Bearer token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMakeGetDeploymentLogsHandlerUnavailable(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "ns",
	}

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment/logs", MakeGetDeploymentLogsHandler(back, testclient.NewSimpleClientset(), &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc/deployment/logs", nil)
	r.ServeHTTP(w, req)

	var response types.DeploymentLogStream
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Available {
		t.Fatal("expected unavailable logs response")
	}
	if response.Message == "" {
		t.Fatal("expected explicit unavailable message")
	}
}

func TestMakeGetDeploymentLogsHandlerInvalidServiceName(t *testing.T) {
	back := backends.MakeFakeBackend()

	r := gin.Default()
	r.GET("/system/services/:serviceName/deployment/logs", MakeGetDeploymentLogsHandler(back, testclient.NewSimpleClientset(), &types.Config{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/services/svc}/deployment/logs", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
