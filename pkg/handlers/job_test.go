package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	testclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		Owner:  "uid",
		Token:  "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf",
		CPU:    "100m",
		Memory: "128Mi",
	}}
	cfg := types.Config{}
	qb := &types.QuotaBackend{KubeClientset: back.GetKubeClientset()}

	r := gin.Default()
	r.POST("/job/:serviceName", MakeJobHandler(&cfg, *qb, back, nil))

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"Records": [{"requestParameters": {"principalId": "uid", "sourceIPAddress": "ip"}}]}`)
	serviceName := "testName"
	req, _ := http.NewRequest("POST", "/job/"+serviceName, body)
	req.Header.Set("Authorization", "Bearer 11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expecting code %d, got %d", http.StatusCreated, w.Code)
	}

	kubeClient := back.GetKubeClientset().(*testclient.Clientset)
	actions := kubeClient.Actions()

	var jobCreate k8stesting.CreateAction
	for _, a := range actions {
		if a.GetVerb() == "create" && a.GetResource().Resource == "jobs" {
			jobCreate = a.(k8stesting.CreateAction)
			break
		}
	}
	if jobCreate == nil {
		for _, a := range actions {
			t.Logf("action: %s %s/%s", a.GetVerb(), a.GetResource().Group, a.GetResource().Resource)
		}
		t.Fatalf("expected a create job action among %d actions", len(actions))
	}
	job, ok := jobCreate.GetObject().(*batchv1.Job)
	if !ok {
		t.Fatalf("expected job object, got %T", jobCreate.GetObject())
	}
	if job.Spec.Template.Spec.EnableServiceLinks == nil {
		t.Fatal("expected job pod spec to set EnableServiceLinks")
	}
	if *job.Spec.Template.Spec.EnableServiceLinks {
		t.Fatal("expected job pod spec to disable service links")
	}
}

func TestJobEventPrincipalCannotSelectForeignSecret(t *testing.T) {
	const token = "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf"
	for _, tc := range []struct {
		name, credential string
		status           int
		secret           string
	}{
		{"service token, forged principal", token, http.StatusForbidden, ""},
		{"service token, owner principal", token, http.StatusCreated, "minio"},
		{"storage webhook, event principal", webhookToken("testName", token, "server-secret"), http.StatusCreated, auth.FormatUID("victim")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			back := backends.MakeFakeBackend()
			back.Services = []*types.Service{{Name: "testName", Token: token, Owner: "owner", Namespace: "owner-ns", CPU: "100m", Memory: "128Mi"}}
			client := back.GetKubeClientset().(*testclient.Clientset)
			_, err := client.CoreV1().Secrets(auth.ServicesNamespace).Create(context.Background(), &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: auth.FormatUID("victim"), Namespace: auth.ServicesNamespace},
				Data:       map[string][]byte{"accessKey": []byte("victim-credential")},
			}, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CoreV1().Secrets(auth.ServicesNamespace).Create(context.Background(), &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "minio", Namespace: auth.ServicesNamespace},
			}, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			client.ClearActions()
			principal := "victim"
			if tc.secret == "minio" {
				principal = "owner"
			}
			r := gin.New()
			cfg := &types.Config{ServicesNamespace: "owner-ns", MinIOProvider: &types.MinIOProvider{SecretKey: "server-secret"}}
			r.POST("/job/:serviceName", MakeJobHandler(cfg, types.QuotaBackend{KubeClientset: client}, back, nil))
			req := httptest.NewRequest(http.MethodPost, "/job/testName", strings.NewReader(`{"Records":[{"requestParameters":{"principalId":"`+principal+`","sourceIPAddress":"ip"}}]}`))
			req.Header.Set("Authorization", "Bearer "+tc.credential)
			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)
			if resp.Code != tc.status {
				t.Fatalf("expected %d, got %d: %s", tc.status, resp.Code, resp.Body.String())
			}
			foundJob := false
			for _, action := range client.Actions() {
				if action.GetVerb() == "create" && action.GetResource().Resource == "jobs" {
					foundJob = true
					job := action.(k8stesting.CreateAction).GetObject().(*batchv1.Job)
					for _, volume := range job.Spec.Template.Spec.Volumes {
						if volume.Name == MinIOSecretVolumeName && volume.Secret.SecretName != tc.secret {
							t.Fatalf("mounted %q, wanted %q", volume.Secret.SecretName, tc.secret)
						}
					}
				}
				if tc.secret == "" && action.GetVerb() == "create" && action.GetResource().Resource == "secrets" {
					t.Fatal("forged principal copied a secret")
				}
			}
			if foundJob != (tc.status == http.StatusCreated) {
				t.Fatalf("job created: %v", foundJob)
			}
			if tc.secret == auth.FormatUID("victim") {
				copied, err := client.CoreV1().Secrets("owner-ns").Get(context.Background(), tc.secret, metav1.GetOptions{})
				if err != nil || string(copied.Data["accessKey"]) != "victim-credential" {
					t.Fatalf("expected webhook to copy the event principal's credentials: %v", err)
				}
			}
		})
	}
}
