package resources

import (
	"context"
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestConfig() *types.Config {
	return &types.Config{
		ServicesNamespace:                 "oscar-svc",
		Namespace:                         "oscar",
		Name:                              "oscar",
		ServicePort:                       8080,
		IngressServicesCORSAllowedOrigins: "*",
		IngressServicesCORSAllowedMethods: "GET,POST",
		IngressServicesCORSAllowedHeaders: "*",
	}
}

func newExposeService(name string, nodePort int32, setAuth bool) types.Service {
	svc := types.Service{
		Name:   name,
		Image:  "ghcr.io/grycap/test",
		Script: "echo test",
		Token:  "s3cr3t",
		Expose: types.Expose{
			MinScale:      1,
			MaxScale:      3,
			APIPort:       9090,
			CpuThreshold:  55,
			NodePort:      nodePort,
			SetAuth:       setAuth,
			RewriteTarget: false,
		},
	}
	svc.Environment.Vars = map[string]string{}
	svc.Environment.Secrets = map[string]string{}
	return svc
}

func TestCreateExposeWithIngressAndAuth(t *testing.T) {
	cfg := newTestConfig()
	svc := newExposeService("ingress-service", 0, true)
	client := fake.NewSimpleClientset()

	if err := CreateExpose(svc, client, cfg); err != nil {
		t.Fatalf("CreateExpose returned error: %v", err)
	}

	if _, err := client.AppsV1().Deployments(cfg.ServicesNamespace).Get(context.TODO(), getDeploymentName(svc.Name), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected deployment to exist: %v", err)
	}

	if _, err := client.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Get(context.TODO(), getHPAName(svc.Name), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected hpa to exist: %v", err)
	}

	kubeSvc, err := client.CoreV1().Services(cfg.ServicesNamespace).Get(context.TODO(), getServiceName(svc.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service to exist: %v", err)
	}

	if string(kubeSvc.Spec.Type) != typeClusterIP {
		t.Fatalf("expected ClusterIP service, got %s", kubeSvc.Spec.Type)
	}

	if _, err := client.NetworkingV1().Ingresses(cfg.ServicesNamespace).Get(context.TODO(), getIngressName(svc.Name), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected ingress to exist: %v", err)
	}

	if _, err := client.CoreV1().Secrets(cfg.ServicesNamespace).Get(context.TODO(), getSecretName(svc.Name), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected auth secret to exist: %v", err)
	}
}

func TestCreateExposeNodePort(t *testing.T) {
	cfg := newTestConfig()
	svc := newExposeService("nodeport-service", 30080, false)
	client := fake.NewSimpleClientset()

	if err := CreateExpose(svc, client, cfg); err != nil {
		t.Fatalf("CreateExpose returned error: %v", err)
	}

	kubeSvc, err := client.CoreV1().Services(cfg.ServicesNamespace).Get(context.TODO(), getServiceName(svc.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service to exist: %v", err)
	}

	if string(kubeSvc.Spec.Type) != typeNodePort {
		t.Fatalf("expected NodePort service, got %s", kubeSvc.Spec.Type)
	}

	if kubeSvc.Spec.Ports[0].NodePort != svc.Expose.NodePort {
		t.Fatalf("expected nodePort %d, got %d", svc.Expose.NodePort, kubeSvc.Spec.Ports[0].NodePort)
	}

	if existsIngress(svc.Name, cfg.ServicesNamespace, client) {
		t.Fatalf("expected no ingress to be created for NodePort expose")
	}
}

func TestUpdateExposeTransitions(t *testing.T) {
	cfg := newTestConfig()
	client := fake.NewSimpleClientset()

	ingressSvc := newExposeService("transition", 0, true)
	if err := CreateExpose(ingressSvc, client, cfg); err != nil {
		t.Fatalf("failed to create ingress expose: %v", err)
	}

	// Switch to NodePort and disable auth
	nodePortSvc := newExposeService("transition", 30200, false)
	if err := UpdateExpose(nodePortSvc, client, cfg); err != nil {
		t.Fatalf("UpdateExpose (to NodePort) returned error: %v", err)
	}

	if existsIngress(nodePortSvc.Name, cfg.ServicesNamespace, client) {
		t.Fatalf("expected ingress to be removed when switching to NodePort")
	}

	if existsSecret(nodePortSvc.Name, client, cfg) {
		t.Fatalf("expected auth secret to be removed when auth disabled")
	}

	kubeSvc, err := client.CoreV1().Services(cfg.ServicesNamespace).Get(context.TODO(), getServiceName(nodePortSvc.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service to exist: %v", err)
	}
	if string(kubeSvc.Spec.Type) != typeNodePort {
		t.Fatalf("expected NodePort service after update, got %s", kubeSvc.Spec.Type)
	}

	// Switch back to ingress with auth
	ingressSvcAgain := newExposeService("transition", 0, true)
	if err := UpdateExpose(ingressSvcAgain, client, cfg); err != nil {
		t.Fatalf("UpdateExpose (back to ingress) returned error: %v", err)
	}

	if !existsIngress(ingressSvcAgain.Name, cfg.ServicesNamespace, client) {
		t.Fatalf("expected ingress to be recreated")
	}

	if !existsSecret(ingressSvcAgain.Name, client, cfg) {
		t.Fatalf("expected auth secret to be recreated when auth enabled")
	}
}

func TestDeleteExposeRemovesResources(t *testing.T) {
	cfg := newTestConfig()
	client := fake.NewSimpleClientset()

	svc := newExposeService("cleanup", 0, true)
	if err := CreateExpose(svc, client, cfg); err != nil {
		t.Fatalf("failed to create expose: %v", err)
	}

	if err := DeleteExpose(svc.Name, client, cfg); err != nil {
		t.Fatalf("DeleteExpose returned error: %v", err)
	}

	if _, err := client.AppsV1().Deployments(cfg.ServicesNamespace).Get(context.TODO(), getDeploymentName(svc.Name), metav1.GetOptions{}); err == nil {
		t.Fatalf("expected deployment to be removed")
	}

	if _, err := client.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Get(context.TODO(), getHPAName(svc.Name), metav1.GetOptions{}); err == nil {
		t.Fatalf("expected hpa to be removed")
	}

	if _, err := client.CoreV1().Services(cfg.ServicesNamespace).Get(context.TODO(), getServiceName(svc.Name), metav1.GetOptions{}); err == nil {
		t.Fatalf("expected service to be removed")
	}

	if existsIngress(svc.Name, cfg.ServicesNamespace, client) {
		t.Fatalf("expected ingress to be removed")
	}

	if existsSecret(svc.Name, client, cfg) {
		t.Fatalf("expected secret to be removed")
	}
}

func TestListExpose(t *testing.T) {
	cfg := newTestConfig()
	client := fake.NewSimpleClientset()
	svc := newExposeService("list", 0, false)
	if err := CreateExpose(svc, client, cfg); err != nil {
		t.Fatalf("failed to create expose: %v", err)
	}
	if err := ListExpose(client, cfg); err != nil {
		t.Fatalf("ListExpose returned error: %v", err)
	}
}

func TestUpdateIngressSecretTransitions(t *testing.T) {
	cfg := newTestConfig()
	client := fake.NewSimpleClientset()
	svc := newExposeService("update-ingress", 0, true)

	// initial resources so Update succeeds
	if _, err := client.NetworkingV1().Ingresses(cfg.ServicesNamespace).Create(context.TODO(), getIngressSpec(svc, cfg), metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to seed ingress: %v", err)
	}

	if err := updateIngress(svc, client, cfg); err != nil {
		t.Fatalf("updateIngress returned error: %v", err)
	}
	if !existsSecret(svc.Name, client, cfg) {
		t.Fatalf("expected secret to be created when auth enabled")
	}

	// switch off auth and ensure secret gets removed on update
	svc.Expose.SetAuth = false
	if err := updateIngress(svc, client, cfg); err != nil {
		t.Fatalf("updateIngress (remove auth) returned error: %v", err)
	}
	if existsSecret(svc.Name, client, cfg) {
		t.Fatalf("expected secret to be deleted when auth disabled")
	}

	// enable auth again and ensure updateSecret path is exercised
	if _, err := client.CoreV1().Secrets(cfg.ServicesNamespace).Create(context.TODO(), getSecretSpec(svc, cfg), metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to seed secret for update path: %v", err)
	}
	svc.Expose.SetAuth = true
	if err := updateIngress(svc, client, cfg); err != nil {
		t.Fatalf("updateIngress (re-enable auth) returned error: %v", err)
	}
	if !existsSecret(svc.Name, client, cfg) {
		t.Fatalf("expected secret to exist after re-enabling auth")
	}
}
