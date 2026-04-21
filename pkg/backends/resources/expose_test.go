package resources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

type kueueMock struct {
	KUBERNETES_SERVICE_HOST string
	KUBERNETES_SERVICE_PORT string
}

func newKueueMock() *kueueMock {
	return &kueueMock{
		KUBERNETES_SERVICE_HOST: "localhost",
		KUBERNETES_SERVICE_PORT: "8080",
	}
}
func (k *kueueMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"Status":"success"}`))
	return
}
func newTestConfig() *types.Config {
	return &types.Config{
		ServicesNamespace:                 "oscar-svc",
		Namespace:                         "oscar",
		Name:                              "oscar",
		ServicePort:                       8080,
		IngressServicesCORSAllowedOrigins: "*",
		IngressServicesCORSAllowedMethods: "GET,POST",
		IngressServicesCORSAllowedHeaders: "*",
		KueueEnable:                       false,
	}
}

func newExposeService(name string, nodePort int32, setAuth bool) types.Service {
	svc := types.Service{
		Name:   name,
		Image:  "ghcr.io/grycap/test",
		Script: "echo test",
		Token:  "s3cr3t",
		Owner:  types.DefaultOwner,
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

func useFakeGatewayClient(t *testing.T) {
	t.Helper()

	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	gatewayClientsetProvider = func() (dynamic.Interface, error) {
		return client, nil
	}

	t.Cleanup(func() {
		gatewayClientsetProvider = getGatewayClientset
	})
}

func TestCreateExposeWithIngressAndAuth(t *testing.T) {
	mock := newKueueMock()
	server := httptest.NewServer(mock)
	defer server.Close()
	cfg := newTestConfig()
	svc := newExposeService("ingress-service", 0, true)
	svc.Namespace = cfg.ServicesNamespace
	client := fake.NewSimpleClientset()

	if err := CreateExpose(svc, svc.Namespace, client, cfg); err != nil {
		t.Fatalf("CreateExpose returned error: %v", err)
	}

	if _, err := client.AppsV1().Deployments(svc.Namespace).Get(context.TODO(), getDeploymentName(svc.Name), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected deployment to exist: %v", err)
	}

	if _, err := client.AutoscalingV1().HorizontalPodAutoscalers(svc.Namespace).Get(context.TODO(), getHPAName(svc.Name), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected hpa to exist: %v", err)
	}

	kubeSvc, err := client.CoreV1().Services(svc.Namespace).Get(context.TODO(), getServiceName(svc.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service to exist: %v", err)
	}

	if string(kubeSvc.Spec.Type) != typeClusterIP {
		t.Fatalf("expected ClusterIP service, got %s", kubeSvc.Spec.Type)
	}

	if _, err := client.NetworkingV1().Ingresses(svc.Namespace).Get(context.TODO(), getIngressName(svc.Name), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected ingress to exist: %v", err)
	}

	if _, err := client.CoreV1().Secrets(svc.Namespace).Get(context.TODO(), getSecretName(svc.Name), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected auth secret to exist: %v", err)
	}
}

func TestCreateExposeNodePort(t *testing.T) {
	cfg := newTestConfig()
	svc := newExposeService("nodeport-service", 30080, false)
	svc.Namespace = cfg.ServicesNamespace
	client := fake.NewSimpleClientset()

	if err := CreateExpose(svc, svc.Namespace, client, cfg); err != nil {
		t.Fatalf("CreateExpose returned error: %v", err)
	}

	kubeSvc, err := client.CoreV1().Services(svc.Namespace).Get(context.TODO(), getServiceName(svc.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service to exist: %v", err)
	}

	if string(kubeSvc.Spec.Type) != typeNodePort {
		t.Fatalf("expected NodePort service, got %s", kubeSvc.Spec.Type)
	}

	if kubeSvc.Spec.Ports[0].NodePort != svc.Expose.NodePort {
		t.Fatalf("expected nodePort %d, got %d", svc.Expose.NodePort, kubeSvc.Spec.Ports[0].NodePort)
	}

	if existsIngress(svc.Name, svc.Namespace, client) {
		t.Fatalf("expected no ingress to be created for NodePort expose")
	}
}

func TestCreateExposeHTTPRouteWithAuth(t *testing.T) {
	useFakeGatewayClient(t)

	cfg := newTestConfig()
	cfg.ExposedServicesRouteKind = "httproute"
	cfg.HTTPRouteGatewayName = "public-gateway"
	cfg.HTTPRouteGatewayNamespace = "gateway-system"
	cfg.IngressHost = "example.org"

	svc := newExposeService("httproute-service", 0, true)
	svc.Namespace = cfg.ServicesNamespace
	client := fake.NewSimpleClientset()

	if err := CreateExpose(svc, svc.Namespace, client, cfg); err != nil {
		t.Fatalf("CreateExpose returned error: %v", err)
	}

	if !existsHTTPRoute(svc.Name, svc.Namespace) {
		t.Fatalf("expected httproute to exist")
	}

	if existsIngress(svc.Name, svc.Namespace, client) {
		t.Fatalf("expected no ingress to be created when route kind is httproute")
	}

	if !existsTraefikCORSMiddleware(svc.Name, svc.Namespace) {
		t.Fatalf("expected traefik CORS middleware to exist")
	}

	if !existsTraefikAuthMiddleware(svc.Name, svc.Namespace) {
		t.Fatalf("expected traefik auth middleware to exist")
	}

	if !existsTraefikAuthSecret(svc.Name, svc.Namespace, client) {
		t.Fatalf("expected traefik auth secret to exist")
	}

	if existsSecret(svc.Name, svc.Namespace, client, cfg) {
		t.Fatalf("expected ingress auth secret to not exist for httproute mode")
	}
}

func TestCreateExposeHTTPRouteWithoutAuth(t *testing.T) {
	useFakeGatewayClient(t)

	cfg := newTestConfig()
	cfg.ExposedServicesRouteKind = "httproute"
	cfg.HTTPRouteGatewayName = "public-gateway"

	svc := newExposeService("httproute-no-auth", 0, false)
	svc.Namespace = cfg.ServicesNamespace
	client := fake.NewSimpleClientset()

	if err := CreateExpose(svc, svc.Namespace, client, cfg); err != nil {
		t.Fatalf("CreateExpose returned error: %v", err)
	}

	if !existsHTTPRoute(svc.Name, svc.Namespace) {
		t.Fatalf("expected httproute to exist")
	}

	if !existsTraefikCORSMiddleware(svc.Name, svc.Namespace) {
		t.Fatalf("expected traefik CORS middleware to exist")
	}

	if existsTraefikAuthMiddleware(svc.Name, svc.Namespace) {
		t.Fatalf("expected no traefik auth middleware when auth is disabled")
	}

	if existsTraefikAuthSecret(svc.Name, svc.Namespace, client) {
		t.Fatalf("expected no traefik auth secret when auth is disabled")
	}
}

func TestUpdateExposeTransitions(t *testing.T) {
	cfg := newTestConfig()
	client := fake.NewSimpleClientset()

	ingressSvc := newExposeService("transition", 0, true)
	ingressSvc.Namespace = cfg.ServicesNamespace
	if err := CreateExpose(ingressSvc, ingressSvc.Namespace, client, cfg); err != nil {
		t.Fatalf("failed to create ingress expose: %v", err)
	}

	// Switch to NodePort and disable auth
	nodePortSvc := newExposeService("transition", 30200, false)
	nodePortSvc.Namespace = cfg.ServicesNamespace
	if err := UpdateExpose(nodePortSvc, nodePortSvc.Namespace, client, cfg); err != nil {
		t.Fatalf("UpdateExpose (to NodePort) returned error: %v", err)
	}

	if existsIngress(nodePortSvc.Name, nodePortSvc.Namespace, client) {
		t.Fatalf("expected ingress to be removed when switching to NodePort")
	}

	if existsSecret(nodePortSvc.Name, nodePortSvc.Namespace, client, cfg) {
		t.Fatalf("expected auth secret to be removed when auth disabled")
	}

	kubeSvc, err := client.CoreV1().Services(nodePortSvc.Namespace).Get(context.TODO(), getServiceName(nodePortSvc.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service to exist: %v", err)
	}
	if string(kubeSvc.Spec.Type) != typeNodePort {
		t.Fatalf("expected NodePort service after update, got %s", kubeSvc.Spec.Type)
	}

	// Switch back to ingress with auth
	ingressSvcAgain := newExposeService("transition", 0, true)
	ingressSvcAgain.Namespace = cfg.ServicesNamespace
	if err := UpdateExpose(ingressSvcAgain, ingressSvcAgain.Namespace, client, cfg); err != nil {
		t.Fatalf("UpdateExpose (back to ingress) returned error: %v", err)
	}

	if !existsIngress(ingressSvcAgain.Name, ingressSvcAgain.Namespace, client) {
		t.Fatalf("expected ingress to be recreated")
	}

	if !existsSecret(ingressSvcAgain.Name, ingressSvcAgain.Namespace, client, cfg) {
		t.Fatalf("expected auth secret to be recreated when auth enabled")
	}
}

func TestDeleteExposeRemovesResources(t *testing.T) {
	cfg := newTestConfig()
	client := fake.NewSimpleClientset()

	svc := newExposeService("cleanup", 0, true)
	svc.Namespace = cfg.ServicesNamespace
	if err := CreateExpose(svc, svc.Namespace, client, cfg); err != nil {
		t.Fatalf("failed to create expose: %v", err)
	}

	if err := DeleteExpose(svc.Name, svc.Namespace, client, cfg); err != nil {
		t.Fatalf("DeleteExpose returned error: %v", err)
	}

	if _, err := client.AppsV1().Deployments(svc.Namespace).Get(context.TODO(), getDeploymentName(svc.Name), metav1.GetOptions{}); err == nil {
		t.Fatalf("expected deployment to be removed")
	}

	if _, err := client.AutoscalingV1().HorizontalPodAutoscalers(svc.Namespace).Get(context.TODO(), getHPAName(svc.Name), metav1.GetOptions{}); err == nil {
		t.Fatalf("expected hpa to be removed")
	}

	if _, err := client.CoreV1().Services(svc.Namespace).Get(context.TODO(), getServiceName(svc.Name), metav1.GetOptions{}); err == nil {
		t.Fatalf("expected service to be removed")
	}

	if existsIngress(svc.Name, svc.Namespace, client) {
		t.Fatalf("expected ingress to be removed")
	}

	if existsSecret(svc.Name, svc.Namespace, client, cfg) {
		t.Fatalf("expected secret to be removed")
	}
}

/*func TestListExpose(t *testing.T) {
	cfg := newTestConfig()
	client := fake.NewSimpleClientset()
	svc := newExposeService("list", 0, false)
	svc.Namespace = cfg.ServicesNamespace
	if err := CreateExpose(svc, svc.Namespace, client, cfg); err != nil {
		t.Fatalf("failed to create expose: %v", err)
	}
	if err := ListExpose(client, cfg); err != nil {
		t.Fatalf("ListExpose returned error: %v", err)
	}
}*/

func TestUpdateIngressSecretTransitions(t *testing.T) {
	cfg := newTestConfig()
	client := fake.NewSimpleClientset()
	svc := newExposeService("update-ingress", 0, true)

	// initial resources so Update succeeds
	if _, err := client.NetworkingV1().Ingresses(cfg.ServicesNamespace).Create(context.TODO(), getIngressSpec(svc, cfg.ServicesNamespace, cfg), metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to seed ingress: %v", err)
	}

	if err := updateIngress(svc, cfg.ServicesNamespace, client, cfg); err != nil {
		t.Fatalf("updateIngress returned error: %v", err)
	}
	if !existsSecret(svc.Name, cfg.ServicesNamespace, client, cfg) {
		t.Fatalf("expected secret to be created when auth enabled")
	}

	// switch off auth and ensure secret gets removed on update
	svc.Expose.SetAuth = false
	if err := updateIngress(svc, cfg.ServicesNamespace, client, cfg); err != nil {
		t.Fatalf("updateIngress (remove auth) returned error: %v", err)
	}
	if existsSecret(svc.Name, cfg.ServicesNamespace, client, cfg) {
		t.Fatalf("expected secret to be deleted when auth disabled")
	}

	// enable auth again and ensure updateSecret path is exercised
	if _, err := client.CoreV1().Secrets(cfg.ServicesNamespace).Create(context.TODO(), getSecretSpec(svc, cfg.ServicesNamespace, cfg), metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to seed secret for update path: %v", err)
	}
	svc.Expose.SetAuth = true
	if err := updateIngress(svc, cfg.ServicesNamespace, client, cfg); err != nil {
		t.Fatalf("updateIngress (re-enable auth) returned error: %v", err)
	}
	if !existsSecret(svc.Name, cfg.ServicesNamespace, client, cfg) {
		t.Fatalf("expected secret to exist after re-enabling auth")
	}
}

func TestRouteKindSelection(t *testing.T) {
	cfgIngress := newTestConfig()
	cfgIngress.ExposedServicesRouteKind = "ingress"
	if getRouteKind(cfgIngress) == routeKindHTTPRoute {
		t.Fatalf("expected ingress route kind")
	}

	cfgHTTPRoute := newTestConfig()
	cfgHTTPRoute.ExposedServicesRouteKind = "httproute"
	if getRouteKind(cfgHTTPRoute) != routeKindHTTPRoute {
		t.Fatalf("expected httproute route kind")
	}
}

func TestGetHTTPRouteSpec(t *testing.T) {
	cfg := newTestConfig()
	cfg.ExposedServicesRouteKind = "httproute"
	cfg.IngressHost = "example.org"
	cfg.HTTPRouteGatewayName = "public-gateway"
	cfg.HTTPRouteGatewayNamespace = "gateway-system"

	svc := newExposeService("http-route-service", 0, false)
	httpRoute := getHTTPRouteSpec(svc, cfg.ServicesNamespace, cfg)

	if httpRoute.GetName() != getHTTPRouteName(svc.Name) {
		t.Fatalf("expected httproute name %s, got %s", getHTTPRouteName(svc.Name), httpRoute.GetName())
	}

	parentRefs, found, err := unstructured.NestedSlice(httpRoute.Object, "spec", "parentRefs")
	if err != nil || !found {
		t.Fatalf("expected parentRefs in httproute spec")
	}

	if len(parentRefs) != 1 {
		t.Fatalf("expected one parentRef, got %d", len(parentRefs))
	}

	parentRef, ok := parentRefs[0].(map[string]any)
	if !ok {
		t.Fatalf("expected parentRef to be a map")
	}

	if parentRef["name"] != cfg.HTTPRouteGatewayName {
		t.Fatalf("expected gateway name %s, got %v", cfg.HTTPRouteGatewayName, parentRef["name"])
	}

	if parentRef["namespace"] != cfg.HTTPRouteGatewayNamespace {
		t.Fatalf("expected gateway namespace %s", cfg.HTTPRouteGatewayNamespace)
	}

	hostnames, found, err := unstructured.NestedSlice(httpRoute.Object, "spec", "hostnames")
	if err != nil || !found {
		t.Fatalf("expected hostnames in httproute spec")
	}

	if len(hostnames) != 1 || hostnames[0] != cfg.IngressHost {
		t.Fatalf("expected host %s", cfg.IngressHost)
	}

	rules, found, err := unstructured.NestedSlice(httpRoute.Object, "spec", "rules")
	if err != nil || !found {
		t.Fatalf("expected rules in httproute spec")
	}

	if len(rules) != 1 {
		t.Fatalf("expected one rule, got %d", len(rules))
	}

	rule, ok := rules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected rule to be a map")
	}

	backendRefs, ok := rule["backendRefs"].([]any)
	if !ok || len(backendRefs) != 1 {
		t.Fatalf("expected one backendRef")
	}

	backendRef, ok := backendRefs[0].(map[string]any)
	if !ok {
		t.Fatalf("expected backendRef to be a map")
	}

	if backendRef["name"] != getServiceName(svc.Name) {
		t.Fatalf("expected backend service %s, got %v", getServiceName(svc.Name), backendRef["name"])
	}

	filters, ok := rule["filters"].([]any)
	if !ok || len(filters) != 2 {
		t.Fatalf("expected rewrite and extensionRef filters for non rewrite_target mode")
	}

	filter, ok := filters[0].(map[string]any)
	if !ok {
		t.Fatalf("expected filter to be a map")
	}

	if filter["type"] != "URLRewrite" {
		t.Fatalf("expected URLRewrite filter, got %v", filter["type"])
	}

	filter, ok = filters[1].(map[string]any)
	if !ok {
		t.Fatalf("expected extensionRef filter to be a map")
	}

	if filter["type"] != "ExtensionRef" {
		t.Fatalf("expected ExtensionRef filter, got %v", filter["type"])
	}

	extensionRef, ok := filter["extensionRef"].(map[string]any)
	if !ok {
		t.Fatalf("expected extensionRef content")
	}

	if extensionRef["group"] != "traefik.io" || extensionRef["kind"] != "Middleware" || extensionRef["name"] != getTraefikCORSMiddlewareName(svc.Name) {
		t.Fatalf("expected traefik middleware reference, got %v", extensionRef)
	}
}

func TestValidateHTTPRouteConfig(t *testing.T) {
	svc := newExposeService("validation", 0, false)
	cfg := newTestConfig()
	cfg.ExposedServicesRouteKind = "httproute"

	if err := validateHTTPRouteConfig(svc, cfg); err == nil {
		t.Fatalf("expected error when HTTPROUTE_GATEWAY_NAME is empty")
	}

	cfg.HTTPRouteGatewayName = "public-gateway"
	svc.Expose.SetAuth = true
	if err := validateHTTPRouteConfig(svc, cfg); err != nil {
		t.Fatalf("expected set_auth to be valid with httproute, got: %v", err)
	}

	svc.Expose.SetAuth = false
	if err := validateHTTPRouteConfig(svc, cfg); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestGetHTTPRouteSpecWithAuth(t *testing.T) {
	cfg := newTestConfig()
	cfg.ExposedServicesRouteKind = "httproute"
	cfg.HTTPRouteGatewayName = "public-gateway"

	svc := newExposeService("http-route-auth", 0, true)
	httpRoute := getHTTPRouteSpec(svc, cfg.ServicesNamespace, cfg)

	rules, found, err := unstructured.NestedSlice(httpRoute.Object, "spec", "rules")
	if err != nil || !found || len(rules) != 1 {
		t.Fatalf("expected one rule in httproute spec")
	}

	rule, ok := rules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected rule to be a map")
	}

	filters, ok := rule["filters"].([]any)
	if !ok || len(filters) != 3 {
		t.Fatalf("expected URLRewrite + CORS + Auth filters, got %v", rule["filters"])
	}

	authFilter, ok := filters[2].(map[string]any)
	if !ok {
		t.Fatalf("expected auth filter map")
	}
	if authFilter["type"] != "ExtensionRef" {
		t.Fatalf("expected auth filter type ExtensionRef, got %v", authFilter["type"])
	}

	extensionRef, ok := authFilter["extensionRef"].(map[string]any)
	if !ok {
		t.Fatalf("expected auth extensionRef content")
	}
	if extensionRef["name"] != getTraefikAuthMiddlewareName(svc.Name) {
		t.Fatalf("expected traefik auth middleware %s, got %v", getTraefikAuthMiddlewareName(svc.Name), extensionRef["name"])
	}
}

func TestGetTraefikCORSMiddlewareSpec(t *testing.T) {
	cfg := newTestConfig()
	cfg.IngressServicesCORSAllowedOrigins = "https://one.example, https://two.example"
	cfg.IngressServicesCORSAllowedMethods = "GET, POST"
	cfg.IngressServicesCORSAllowedHeaders = "Authorization, Content-Type"

	svc := newExposeService("cors-svc", 0, false)
	middleware := getTraefikCORSMiddlewareSpec(svc, cfg.ServicesNamespace, cfg)

	if middleware.GetName() != getTraefikCORSMiddlewareName(svc.Name) {
		t.Fatalf("expected middleware name %s, got %s", getTraefikCORSMiddlewareName(svc.Name), middleware.GetName())
	}

	origins, found, err := unstructured.NestedStringSlice(middleware.Object, "spec", "headers", "accessControlAllowOriginList")
	if err != nil || !found {
		originValues, foundAny, errAny := unstructured.NestedSlice(middleware.Object, "spec", "headers", "accessControlAllowOriginList")
		if errAny != nil || !foundAny {
			t.Fatalf("expected accessControlAllowOriginList in middleware")
		}
		origins = make([]string, 0, len(originValues))
		for _, v := range originValues {
			s, ok := v.(string)
			if !ok {
				t.Fatalf("expected origin value as string, got %T", v)
			}
			origins = append(origins, s)
		}
	}
	if len(origins) != 2 || origins[0] != "https://one.example" || origins[1] != "https://two.example" {
		t.Fatalf("unexpected origins list: %v", origins)
	}

	methods, found, err := unstructured.NestedStringSlice(middleware.Object, "spec", "headers", "accessControlAllowMethods")
	if err != nil || !found {
		methodValues, foundAny, errAny := unstructured.NestedSlice(middleware.Object, "spec", "headers", "accessControlAllowMethods")
		if errAny != nil || !foundAny {
			t.Fatalf("expected accessControlAllowMethods in middleware")
		}
		methods = make([]string, 0, len(methodValues))
		for _, v := range methodValues {
			s, ok := v.(string)
			if !ok {
				t.Fatalf("expected method value as string, got %T", v)
			}
			methods = append(methods, s)
		}
	}
	if len(methods) != 2 || methods[0] != "GET" || methods[1] != "POST" {
		t.Fatalf("unexpected methods list: %v", methods)
	}

	headers, found, err := unstructured.NestedStringSlice(middleware.Object, "spec", "headers", "accessControlAllowHeaders")
	if err != nil || !found {
		headerValues, foundAny, errAny := unstructured.NestedSlice(middleware.Object, "spec", "headers", "accessControlAllowHeaders")
		if errAny != nil || !foundAny {
			t.Fatalf("expected accessControlAllowHeaders in middleware")
		}
		headers = make([]string, 0, len(headerValues))
		for _, v := range headerValues {
			s, ok := v.(string)
			if !ok {
				t.Fatalf("expected header value as string, got %T", v)
			}
			headers = append(headers, s)
		}
	}
	if len(headers) != 2 || headers[0] != "Authorization" || headers[1] != "Content-Type" {
		t.Fatalf("unexpected headers list: %v", headers)
	}
}

func TestGetTraefikAuthMiddlewareSpec(t *testing.T) {
	cfg := newTestConfig()
	svc := newExposeService("auth-svc", 0, true)
	middleware := getTraefikAuthMiddlewareSpec(svc, cfg.ServicesNamespace)

	if middleware.GetName() != getTraefikAuthMiddlewareName(svc.Name) {
		t.Fatalf("expected middleware name %s, got %s", getTraefikAuthMiddlewareName(svc.Name), middleware.GetName())
	}

	secretName, found, err := unstructured.NestedString(middleware.Object, "spec", "basicAuth", "secret")
	if err != nil || !found {
		t.Fatalf("expected basicAuth.secret in middleware")
	}

	if secretName != getTraefikAuthSecretName(svc.Name) {
		t.Fatalf("expected basicAuth secret %s, got %s", getTraefikAuthSecretName(svc.Name), secretName)
	}
}

func TestGetTraefikAuthSecretSpec(t *testing.T) {
	cfg := newTestConfig()
	svc := newExposeService("auth-secret-svc", 0, true)
	secret := getTraefikAuthSecretSpec(svc, cfg.ServicesNamespace)

	if secret.Name != getTraefikAuthSecretName(svc.Name) {
		t.Fatalf("expected secret name %s, got %s", getTraefikAuthSecretName(svc.Name), secret.Name)
	}

	users, ok := secret.StringData["users"]
	if !ok || users == "" {
		t.Fatalf("expected users entry in traefik auth secret")
	}
}
