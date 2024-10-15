package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

	K8sObjects := []runtime.Object{
		&batchv1.Job{
			Status: batchv1.JobStatus{
				StartTime: &metav1.Time{},
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
										StartedAt: metav1.Time{},
									},
								},
							},
						},
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
	r.GET("/system/logs/:serviceName", MakeJobsInfoHandler(back, kubeClientset, "namespace"))

	w := httptest.NewRecorder()
	serviceName := "test"
	req, _ := http.NewRequest("GET", "/system/logs/"+serviceName, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		fmt.Println(w.Body)
		t.Errorf("expecting code %d, got %d", http.StatusOK, w.Code)
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

	r := gin.Default()
	r.DELETE("/system/logs/:serviceName", MakeDeleteJobsHandler(back, kubeClientset, "namespace"))

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
	r.GET("/system/logs/:serviceName/:jobName", MakeGetLogsHandler(back, kubeClientset, "namespace"))

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
	r.DELETE("/system/logs/:serviceName/:jobName", MakeDeleteJobHandler(back, kubeClientset, "namespace"))

	w := httptest.NewRecorder()
	serviceName := "test"
	jobName := "job"
	req, _ := http.NewRequest("DELETE", "/system/logs/"+serviceName+"/"+jobName, nil)
	req.Header.Set("Authorization", "Bearer AbCdEf123456")
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
