/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package imagepuller

//TODO check error control

import (
	//"k8s.io/apimachinery/pkg/watch"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/grycap/oscar/v3/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var DaemonSetLoggerInfo = log.New(os.Stdout, "[DAEMONSET-INFO] ", log.Flags())

const letterBytes = "abcdefghijklmnopqrstuvwxyz"
const lengthStr = 5

var podGroup string
var daemonsetName string

var workingNodes int

type PodCounter struct {
	wnCount int
	mutex   sync.Mutex
}

var pc PodCounter
var stopper chan struct{}
var watchPodsFunc = watchPods

// Create daemonset
func CreateDaemonset(cfg *types.Config, service types.Service, namespace string, kubeClientset kubernetes.Interface) error {
	DaemonSetLoggerInfo.Println("Creating daemonset for service:", service.Name)
	//Set needed variables
	err := setWorkingNodes(kubeClientset)
	if err != nil {
		DaemonSetLoggerInfo.Println(err)
		return fmt.Errorf("failed to set working nodes: %s", err.Error())
	}
	podGroup = generatePodGroupName()
	daemonsetName = "image-puller-" + service.Name

	//Get daemonset definition
	daemon := getDaemonset(cfg, service, namespace)

	//Create daemonset
	_, err = kubeClientset.AppsV1().DaemonSets(namespace).Create(context.TODO(), daemon, metav1.CreateOptions{})
	if err != nil {
		DaemonSetLoggerInfo.Println(err)
		return fmt.Errorf("failed to create daemonset: %s", err.Error())
	}

	//Set watcher informer
	watchPodsFunc(kubeClientset, namespace)

	return nil
}

// Get daemonset definition
func getDaemonset(cfg *types.Config, service types.Service, namespace string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonsetName,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"oscar-resource": "daemonset",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"oscar-resource": "daemonset",
						"pod-group":      podGroup,
					},
					Name: "podpuller",
				},
				Spec: corev1.PodSpec{
					Volumes:          []corev1.Volume{},
					ImagePullSecrets: types.SetImagePullSecrets(service.ImagePullSecrets),
					Containers: []corev1.Container{
						{
							Name:    "image-puller",
							Image:   service.Image,
							Command: []string{"/bin/sh", "-c", "sleep 1h"},
						},
					},
				},
			},
		},
	}
}

// Watch pods with a Kubernetes Informer
func watchPods(kubeClientset kubernetes.Interface, namespace string) {
	stopper = make(chan struct{})
	defer close(stopper)

	pc = PodCounter{wnCount: 0}

	var optionsFunc internalinterfaces.TweakListOptionsFunc = func(options *metav1.ListOptions) {
		labelSelector := labels.Set{
			"pod-group": podGroup,
		}.AsSelector()
		options.LabelSelector = labelSelector.String()
	}

	sharedInformerOp := informers.WithTweakListOptions(optionsFunc)

	factory := informers.NewSharedInformerFactoryWithOptions(kubeClientset, 2*time.Second, informers.WithNamespace(namespace), sharedInformerOp)

	podInformer := factory.Core().V1().Pods().Informer()
	factory.Start(stopper)

	//Wait for all the selected resources to be added to the cache
	state := cache.WaitForCacheSync(stopper, podInformer.HasSynced)
	if !state {
		log.Fatalf("Failed to sync informer cache")
	}

	//Add event handler that gets all the pods status
	_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: handleUpdatePodEvent,
	})
	if err != nil {
		DaemonSetLoggerInfo.Println(err)
		log.Fatalf("Failed to add event handler: %s", err.Error())
	}

	<-stopper

	//Delete daemonset when all pods are in state "Running"
	DaemonSetLoggerInfo.Println("Deleting daemonset...")
	err = kubeClientset.AppsV1().DaemonSets(namespace).Delete(context.TODO(), daemonsetName, metav1.DeleteOptions{})
	if err != nil {
		DaemonSetLoggerInfo.Println(err)
		log.Fatalf("Failed to delete daemonset: %s", err.Error())
	} else {
		DaemonSetLoggerInfo.Println("Deleted daemonset")
	}
}

func handleUpdatePodEvent(oldObj interface{}, newObj interface{}) {
	newPod := newObj.(*corev1.Pod)
	if newPod.Status.Phase == corev1.PodRunning {
		pc.mutex.Lock()
		defer pc.mutex.Unlock()
		pc.wnCount++
		//Check the running pods count and stop the informer
		if pc.wnCount >= workingNodes {
			stopper <- struct{}{}
		}
	}
}

func setWorkingNodes(kubeClientset kubernetes.Interface) error {
	nodes, err := kubeClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "!node-role.kubernetes.io/control-plane,!node-role.kubernetes.io/master"})
	if err != nil {
		return fmt.Errorf("error getting node list: %v", err)
	}

	for range nodes.Items {
		workingNodes++
	}
	return nil
}

func generatePodGroupName() string {
	b := make([]byte, lengthStr)
	for i := range b {
		max := big.NewInt(int64(len(letterBytes)))
		randomNumber, _ := rand.Int(rand.Reader, max)
		b[i] = letterBytes[randomNumber.Int64()]
	}
	return "pod-group-" + string(b)
}
