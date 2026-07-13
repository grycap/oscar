package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	testclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/types"
	batchv1 "k8s.io/api/batch/v1"
)

func TestRewriteRustFSEventSource(t *testing.T) {
	original := []byte(`{"Records":[{"eventSource":"rustfs:s3","s3":{"object":{"key":"input%2Fimage.jpg"}}}],"extra":"preserved"}`)
	rewritten := rewriteRustFSEventSource(original)

	var event map[string]interface{}
	if err := json.Unmarshal(rewritten, &event); err != nil {
		t.Fatalf("decoding rewritten event: %v", err)
	}
	record := event["Records"].([]interface{})[0].(map[string]interface{})
	if record["eventSource"] != "minio:s3" {
		t.Fatalf("expected minio:s3 event source, got %v", record["eventSource"])
	}
	if event["extra"] != "preserved" {
		t.Fatalf("unrelated event data was not preserved")
	}
	object := record["s3"].(map[string]interface{})["object"].(map[string]interface{})
	if object["key"] != "input%2Fimage.jpg" {
		t.Fatalf("object key was unexpectedly changed: %v", object["key"])
	}
}

func TestRewriteRustFSEventSourceLeavesOtherInputsUnchanged(t *testing.T) {
	tests := [][]byte{
		[]byte(`{"Records":[{"eventSource":"minio:s3"}]}`),
		[]byte(`plain synchronous input`),
	}
	for _, input := range tests {
		if got := rewriteRustFSEventSource(input); string(got) != string(input) {
			t.Fatalf("expected input to remain unchanged, got %q", got)
		}
	}
}

func TestMakeJobHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Services = []*types.Service{{
		Name:   "testName",
		Token:  "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf",
		CPU:    "100m",
		Memory: "128Mi",
	}}
	cfg := types.Config{}
	kubeClient := testclient.NewSimpleClientset()

	r := gin.Default()
	r.POST("/job/:serviceName", MakeJobHandler(&cfg, kubeClient, back, nil))

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"Records": [{"requestParameters": {"principalId": "uid", "sourceIPAddress": "ip"}}]}`)
	serviceName := "testName"
	req, _ := http.NewRequest("POST", "/job/"+serviceName, body)
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

	createAction, ok := actions[0].(k8stesting.CreateAction)
	if !ok {
		t.Fatalf("expected create action, got %T", actions[0])
	}
	job, ok := createAction.GetObject().(*batchv1.Job)
	if !ok {
		t.Fatalf("expected job object, got %T", createAction.GetObject())
	}
	if job.Spec.Template.Spec.EnableServiceLinks == nil {
		t.Fatal("expected job pod spec to set EnableServiceLinks")
	}
	if *job.Spec.Template.Spec.EnableServiceLinks {
		t.Fatal("expected job pod spec to disable service links")
	}
}
