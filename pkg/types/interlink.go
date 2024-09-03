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

	v1 "k8s.io/api/core/v1"
)

const (
	ContainerSupervisorName = "supervisor-container"
	SupervisorMountPath     = "/data"
	SupervisorArg           = "cp -r /supervisor/* " + SupervisorMountPath
	NameSupervisorVolume    = "supervisor-share-data"
	NodeSelectorKey         = "kubernetes.io/hostname"

	// Annotations for InterLink nodes
	InterLinkDNSPolicy          = "ClusterFirst"
	InterLinkRestartPolicy      = "OnFailure"
	InterLinkTolerationKey      = "virtual-node.interlink/no-schedule"
	InterLinkTolerationOperator = "Exists"
)

var SupervisorCommand = []string{"/bin/sh", "-c"}
var OscarContainerCommand = []string{"echo $EVENT | base64 -d | " + SupervisorMountPath + "/supervisor"}

// SetInterlinkJob Return interlink configuration for kubernetes job and add Interlink variables to podSpec
func SetInterlinkJob(podSpec *v1.PodSpec, service *Service, cfg *Config, eventBytes []byte) ([]string, v1.EnvVar, []string) {
	command := SupervisorCommand
	event := v1.EnvVar{
		Name:  EventVariable,
		Value: base64.StdEncoding.EncodeToString([]byte(eventBytes)),
	}
	args := OscarContainerCommand
	podSpec.NodeSelector = map[string]string{
		NodeSelectorKey: service.InterLinkNodeName,
	}
	podSpec.DNSPolicy = InterLinkDNSPolicy
	podSpec.RestartPolicy = InterLinkRestartPolicy
	podSpec.Tolerations = []v1.Toleration{
		{
			Key:      InterLinkTolerationKey,
			Operator: InterLinkTolerationOperator,
		},
	}

	addInitContainer(podSpec, cfg)
	return command, event, args
}

// SetInterlinkService Add InterLink configuration to podSpec
func SetInterlinkService(podSpec *v1.PodSpec) {
	podSpec.Containers[0].ImagePullPolicy = "Always"
	shareDataVolumeMount := v1.VolumeMount{
		Name:      NameSupervisorVolume,
		MountPath: SupervisorMountPath,
	}

	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, shareDataVolumeMount)

	shareDataVolume := v1.Volume{
		Name: NameSupervisorVolume,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	podSpec.Volumes = append(podSpec.Volumes, shareDataVolume)

}

func addInitContainer(podSpec *v1.PodSpec, cfg *Config) {
	initContainer := v1.Container{
		Name:            ContainerSupervisorName,
		Command:         SupervisorCommand,
		Args:            []string{SupervisorArg},
		Image:           cfg.SupervisorKitImage,
		ImagePullPolicy: v1.PullIfNotPresent,
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      NameSupervisorVolume,
				MountPath: SupervisorMountPath,
			},
		},
	}
	podSpec.InitContainers = []v1.Container{initContainer}
}
