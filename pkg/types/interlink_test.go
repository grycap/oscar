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
package types

import (
	"encoding/base64"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestSetInterlinkJob(t *testing.T) {
	podSpec := &v1.PodSpec{}
	service := &Service{InterLinkNodeName: "test-node"}
	cfg := &Config{SupervisorKitImage: "test-image"}
	eventBytes := []byte("test-event")

	command, event, args := SetInterlinkJob(podSpec, service, cfg, eventBytes)

	if len(command) != 2 || command[0] != "/bin/sh" || command[1] != "-c" {
		t.Errorf("Unexpected command: %v", command)
	}

	expectedEventValue := base64.StdEncoding.EncodeToString(eventBytes)
	if event.Name != EventVariable || event.Value != expectedEventValue {
		t.Errorf("Unexpected event: %v", event)
	}

	expectedArgs := "echo $EVENT | base64 -d | " + SupervisorMountPath + "/supervisor"
	if len(args) != 1 || args[0] != expectedArgs {
		t.Errorf("Unexpected args: %v", args)
	}

	if podSpec.NodeSelector[NodeSelectorKey] != service.InterLinkNodeName {
		t.Errorf("Unexpected NodeSelector: %v", podSpec.NodeSelector)
	}

	if podSpec.DNSPolicy != InterLinkDNSPolicy {
		t.Errorf("Unexpected DNSPolicy: %v", podSpec.DNSPolicy)
	}

	if podSpec.RestartPolicy != InterLinkRestartPolicy {
		t.Errorf("Unexpected RestartPolicy: %v", podSpec.RestartPolicy)
	}

	if len(podSpec.Tolerations) != 1 || podSpec.Tolerations[0].Key != InterLinkTolerationKey || podSpec.Tolerations[0].Operator != InterLinkTolerationOperator {
		t.Errorf("Unexpected Tolerations: %v", podSpec.Tolerations)
	}
}

func TestSetInterlinkService(t *testing.T) {
	podSpec := &v1.PodSpec{
		Containers: []v1.Container{
			{},
		},
	}

	SetInterlinkService(podSpec)

	if podSpec.Containers[0].ImagePullPolicy != "Always" {
		t.Errorf("Unexpected ImagePullPolicy: %v", podSpec.Containers[0].ImagePullPolicy)
	}

	if len(podSpec.Containers[0].VolumeMounts) != 1 || podSpec.Containers[0].VolumeMounts[0].Name != NameSupervisorVolume || podSpec.Containers[0].VolumeMounts[0].MountPath != SupervisorMountPath {
		t.Errorf("Unexpected VolumeMounts: %v", podSpec.Containers[0].VolumeMounts)
	}

	if len(podSpec.Volumes) != 1 || podSpec.Volumes[0].Name != NameSupervisorVolume || podSpec.Volumes[0].VolumeSource.EmptyDir == nil {
		t.Errorf("Unexpected Volumes: %v", podSpec.Volumes)
	}
}

func TestAddInitContainer(t *testing.T) {
	podSpec := &v1.PodSpec{}
	cfg := &Config{SupervisorKitImage: "test-image"}

	addInitContainer(podSpec, cfg)

	if len(podSpec.InitContainers) != 1 {
		t.Fatalf("Expected 1 init container, got %d", len(podSpec.InitContainers))
	}

	initContainer := podSpec.InitContainers[0]
	if initContainer.Name != ContainerSupervisorName {
		t.Errorf("Unexpected init container name: %v", initContainer.Name)
	}

	if len(initContainer.Command) != 2 || initContainer.Command[0] != "/bin/sh" || initContainer.Command[1] != "-c" {
		t.Errorf("Unexpected init container command: %v", initContainer.Command)
	}

	if len(initContainer.Args) != 1 || initContainer.Args[0] != SupervisorArg {
		t.Errorf("Unexpected init container args: %v", initContainer.Args)
	}

	if initContainer.Image != cfg.SupervisorKitImage {
		t.Errorf("Unexpected init container image: %v", initContainer.Image)
	}

	if initContainer.ImagePullPolicy != v1.PullIfNotPresent {
		t.Errorf("Unexpected init container image pull policy: %v", initContainer.ImagePullPolicy)
	}

	if len(initContainer.VolumeMounts) != 1 || initContainer.VolumeMounts[0].Name != NameSupervisorVolume || initContainer.VolumeMounts[0].MountPath != SupervisorMountPath {
		t.Errorf("Unexpected init container volume mounts: %v", initContainer.VolumeMounts)
	}
}
