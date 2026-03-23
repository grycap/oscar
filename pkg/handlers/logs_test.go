package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestMakeJobsInfoHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	now := time.Now()
	cfg := types.Config{
		JobListingLimit: 70,
	}

	K8sObjects := []runtime.Object{
		&batchv1.Job{
			Status: batchv1.JobStatus{
				StartTime: &metav1.Time{Time: now},
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job",
				Namespace: "namespace",
				Labels: map[string]string{
					types.ServiceLabel: "test",
				},
			},
		},
		&corev1.PodList{
			Items: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: types.ContainerName,
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{
										StartedAt: metav1.Time{Time: now},
									},
								},
							},
						},
						StartTime: &metav1.Time{Time: now},
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod",
						Namespace: "namespace",
						Labels: map[string]string{
							"oscar_service": "test",
							"job-name":      "job"},
					},
				},
			},
		},
	}
	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)

	r := gin.Default()
	r.GET("/system/logs/:serviceName", MakeJobsInfoHandler(back, kubeClientset, &cfg))

	w := httptest.NewRecorder()
	serviceName := "test"
	req, _ := http.NewRequest("GET", "/system/logs/"+serviceName, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Println(w.Body)
		t.Errorf("expecting code %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}

	expected := map[string]interface{}{
		"jobs": map[string]interface{}{
			"job": map[string]interface{}{
				"status":        "Running",
				"creation_time": now.UTC().Format(time.RFC3339),
				"start_time":    now.UTC().Format(time.RFC3339),
			},
		},
	}

	if !reflect.DeepEqual(response, expected) {
		t.Errorf("expecting %v, got %v", expected, response)
	}

	actions := kubeClientset.Actions()
	if len(actions) != 2 {
		t.Errorf("expecting 2 actions, got %d", len(actions))
	}

	if actions[0].GetVerb() != "list" || actions[0].GetResource().Resource != "jobs" {
		t.Errorf("expecting list jobs, got %s %s", actions[0].GetVerb(), actions[0].GetResource().Resource)
	}
	if actions[1].GetVerb() != "list" || actions[1].GetResource().Resource != "pods" {
		t.Errorf("expecting list pods, got %s %s", actions[1].GetVerb(), actions[1].GetResource().Resource)
	}
}

func TestMakeDeleteJobsHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	kubeClientset := testclient.NewSimpleClientset()
	cfg := types.Config{
		JobListingLimit: 70,
	}
	r := gin.Default()
	r.DELETE("/system/logs/:serviceName", MakeDeleteJobsHandler(back, kubeClientset, &cfg))

	w := httptest.NewRecorder()
	serviceName := "test"
	req, _ := http.NewRequest("DELETE", "/system/logs/"+serviceName, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		fmt.Println(w.Body)
		t.Errorf("expecting code %d, got %d", http.StatusNoContent, w.Code)
	}

	actions := kubeClientset.Actions()
	if len(actions) != 1 {
		t.Errorf("expecting 1 actions, got %d", len(actions))
	}

	if actions[0].GetVerb() != "delete-collection" || actions[0].GetResource().Resource != "jobs" {
		t.Errorf("expecting list jobs, got %s %s", actions[0].GetVerb(), actions[0].GetResource().Resource)
	}
}

func TestMakeGetLogsHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	cfg := types.Config{
		JobListingLimit: 70,
	}
	K8sObjects := []runtime.Object{
		&corev1.PodList{
			Items: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod",
						Namespace: "namespace",
						Labels: map[string]string{
							"oscar_service": "test",
							"job-name":      "job"},
					},
				},
			},
		},
	}
	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)

	r := gin.Default()
	r.GET("/system/logs/:serviceName/:jobName", MakeGetLogsHandler(back, kubeClientset, &cfg))

	w := httptest.NewRecorder()
	serviceName := "test"
	jobName := "job"
	req, _ := http.NewRequest("GET", "/system/logs/"+serviceName+"/"+jobName, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Println(w.Body)
		t.Errorf("expecting code %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "fake logs" {
		t.Errorf("expecting 'fake logs', got %s", w.Body.String())
	}

	actions := kubeClientset.Actions()
	if len(actions) != 2 {
		t.Errorf("expecting 2 actions, got %d", len(actions))
	}

	if actions[0].GetVerb() != "list" || actions[0].GetResource().Resource != "pods" {
		t.Errorf("expecting list pods, got %s %s", actions[0].GetVerb(), actions[0].GetResource().Resource)
	}
	if actions[1].GetVerb() != "get" || actions[1].GetResource().Resource != "pods" {
		t.Errorf("expecting get pods, got %s %s", actions[1].GetVerb(), actions[1].GetResource().Resource)
	}
}
func TestMakeDeleteJobHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	cfg := types.Config{
		JobListingLimit:   70,
		ServicesNamespace: "namespace",
	}
	K8sObjects := []runtime.Object{
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job",
				Namespace: "namespace",
				Labels: map[string]string{
					types.ServiceLabel: "test",
				},
			},
		},
	}
	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "some-uid-value")
		c.Next()
	})
	r.DELETE("/system/logs/:serviceName/:jobName", MakeDeleteJobHandler(back, kubeClientset, &cfg))

	w := httptest.NewRecorder()
	serviceName := "test"
	jobName := "job"
	req, _ := http.NewRequest("DELETE", "/system/logs/"+serviceName+"/"+jobName, nil)
	req.Header.Set("Authorization", "Bearer 11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		fmt.Println(w.Body)
		t.Errorf("expecting code %d, got %d", http.StatusNoContent, w.Code)
	}

	actions := kubeClientset.Actions()
	if len(actions) != 2 {
		t.Errorf("expecting 2 actions, got %d", len(actions))
	}

	if actions[0].GetVerb() != "get" || actions[0].GetResource().Resource != "jobs" {
		t.Errorf("expecting get jobs, got %s %s", actions[0].GetVerb(), actions[0].GetResource().Resource)
	}

	if actions[1].GetVerb() != "delete" || actions[1].GetResource().Resource != "jobs" {
		t.Errorf("expecting delete jobs, got %s %s", actions[1].GetVerb(), actions[1].GetResource().Resource)
	}
}

func TestMakeGetSystemLogsHandlerBasicAuth(t *testing.T) {
	cfg := types.Config{
		Name:      "oscar",
		Namespace: "oscar",
	}
	now := metav1.Time{Time: time.Now()}
	kubeObjects := []runtime.Object{
		&corev1.PodList{
			Items: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "oscar-123",
						Namespace:         "oscar",
						Labels:            map[string]string{"app": "oscar"},
						CreationTimestamp: now,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "oscar"},
						},
					},
				},
			},
		},
	}
	kubeClientset := testclient.NewSimpleClientset(kubeObjects...)

	r := gin.Default()
	r.GET("/system/logs", MakeGetSystemLogsHandler(kubeClientset, &cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/logs", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expecting code %d, got %d (body: %s)", http.StatusOK, w.Code, w.Body.String())
	}
	var response struct {
		Logs []struct {
			Raw string `json:"raw"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(response.Logs) != 0 {
		t.Errorf("expected empty logs, got %v", response.Logs)
	}

	actions := kubeClientset.Actions()
	if len(actions) != 2 {
		t.Errorf("expecting 2 actions, got %d", len(actions))
	}
	if actions[0].GetVerb() != "list" || actions[0].GetResource().Resource != "pods" {
		t.Errorf("expecting list pods, got %s %s", actions[0].GetVerb(), actions[0].GetResource().Resource)
	}
	if actions[1].GetVerb() != "get" || actions[1].GetResource().Resource != "pods" {
		t.Errorf("expecting get pods, got %s %s", actions[1].GetVerb(), actions[1].GetResource().Resource)
	}
}

func TestMakeGetSystemLogsHandlerRejectsOIDC(t *testing.T) {
	cfg := types.Config{
		Name:      "oscar",
		Namespace: "oscar",
	}
	kubeObjects := []runtime.Object{
		&corev1.PodList{
			Items: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oscar-123",
						Namespace: "oscar",
						Labels:    map[string]string{"app": "oscar"},
					},
				},
			},
		},
	}
	kubeClientset := testclient.NewSimpleClientset(kubeObjects...)

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "some-uid-value")
		c.Next()
	})
	r.GET("/system/logs", MakeGetSystemLogsHandler(kubeClientset, &cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/system/logs", nil)
	req.Header.Set("Authorization", "Bearer token")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expecting code %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestParseExecutionLogs(t *testing.T) {
	raw := `
[GIN-EXECUTIONS-LOGGER] 2025/10/28 - 16:53:34 | 200 |  347.805334ms | 172.25.0.1 | POST    /run/simple-test | oscar
[GIN-EXECUTIONS-LOGGER] 2025/10/28 - 16:55:12 | 201 |   14.219292ms | 127.0.0.1 | POST    /job/simple-test | minio
[GIN-EXECUTIONS-LOGGER] 2025/10/28 - 16:55:20 | 200 |   10.000000ms | 127.0.0.1 | GET     /health | oscar
`

	entries := parseExecutionLogs(raw)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	layout := "2006/01/02 - 15:04:05"
	first := entries[0]
	firstExpected, err := time.ParseInLocation(layout, "2025/10/28 - 16:53:34", time.Local)
	if err != nil {
		t.Fatalf("unable to parse expected timestamp: %v", err)
	}
	if first.Timestamp != firstExpected.UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected first timestamp: %s", first.Timestamp)
	}
	if first.Method != "POST" || first.Path != "/run/simple-test" || first.User != "oscar" {
		t.Fatalf("unexpected first entry: %+v", first)
	}
	second := entries[1]
	secondExpected, err := time.ParseInLocation(layout, "2025/10/28 - 16:55:12", time.Local)
	if err != nil {
		t.Fatalf("unable to parse expected timestamp: %v", err)
	}
	if second.Timestamp != secondExpected.UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected second timestamp: %s", second.Timestamp)
	}
	if second.Path != "/job/simple-test" || second.Status != 201 {
		t.Fatalf("unexpected second entry: %+v", second)
	}
}
