package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
)

func TestMakeJobHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	cfg := types.Config{}
	kubeClient := testclient.NewSimpleClientset()

	r := gin.Default()
	r.POST("/job/:serviceName", MakeJobHandler(&cfg, kubeClient, back, nil))

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"Records": [{"requestParameters": {"principalId": "uid", "sourceIPAddress": "ip"}}]}`)
	serviceName := "testName"
	req, _ := http.NewRequest("POST", "/job/services"+serviceName, body)
	req.Header.Set("Authorization", "Bearer 11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		fmt.Println(w.Body)
		t.Errorf("expecting code %d, got %d", http.StatusCreated, w.Code)
	}

	actions := kubeClient.Actions()
	if len(actions) != 1 {
		t.Errorf("Expected 1 action but got %d", len(actions))
	}
	if actions[0].GetVerb() != "create" || actions[0].GetResource().Resource != "jobs" {
		t.Errorf("Expected create job action but got %v", actions[0])
	}
}

func TestMakeJobHandlerPropagateTokenEnvVar(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	serviceToken := "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf"
	back.Service = &types.Service{
		Name:           "propagate-service",
		Image:          "ubuntu",
		Script:         "echo test",
		Owner:          "test-owner",
		PropagateToken: true,
		Token:          serviceToken,
	}

	cfg := types.Config{}
	kubeClient := testclient.NewSimpleClientset()

	r := gin.New()
	r.POST("/job/:serviceName", MakeJobHandler(&cfg, kubeClient, back, nil))

	body := strings.NewReader(`{"Records": [{"requestParameters": {"principalId": "uid", "sourceIPAddress": "ip"}}]}`)
	req, _ := http.NewRequest("POST", "/job/propagate-service", body)
	req.Header.Set("Authorization", "Bearer "+serviceToken)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expecting code %d, got %d", http.StatusCreated, w.Code)
	}

	actions := kubeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("Expected 1 action but got %d", len(actions))
	}

	createAction, ok := actions[0].(k8stesting.CreateAction)
	if !ok {
		t.Fatalf("expected create action but got %T", actions[0])
	}

	job, ok := createAction.GetObject().(*batchv1.Job)
	if !ok {
		t.Fatalf("expected job object but got %T", createAction.GetObject())
	}

	var found bool
	for _, envVar := range job.Spec.Template.Spec.Containers[0].Env {
		if envVar.Name == types.AccessTokenEnvVar {
			found = true
			if envVar.Value != serviceToken {
				t.Fatalf("expected ACCESS_TOKEN value %s, got %s", serviceToken, envVar.Value)
			}
			break
		}
	}

	if !found {
		t.Fatal("expected ACCESS_TOKEN env var to be present in async job pod spec")
	}
}
