package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	testclient "k8s.io/client-go/kubernetes/fake"

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
	body := strings.NewReader(``)
	serviceName := "testName"
	req, _ := http.NewRequest("POST", "/job/services"+serviceName, body)
	req.Header.Set("Authorization", "Bearer AbCdEf123456")
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
