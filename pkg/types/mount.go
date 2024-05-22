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
	"strings"

	v1 "k8s.io/api/core/v1"
)

const (
	s3_fs_fuse_containerName  = "pocminiovolumek8s"
	s3_fs_fuse_containerImage = "ghcr.io/esparig/pocminiovolumek8s:mys3fs"
	s3_fs_fuse_commandImage   = "s3fs -d -f ${MINIO_BUCKET} ${MNT_POINT} -o use_path_request_style,no_check_certificate,ssl_verify_hostname=0,allow_other,umask=0007,uid=1000,gid=100,url=http://${MINIO_ENDPOINT}:9000"
	s3_fs_fuse_folder_mount   = "/mnt/data"
	s3_fs_fuse_volume_name    = "shared-data"
	//MINIO_ENDPOINT
	//MINIO_ACCESS_KEY
	//MINIO_SECRET_KEY
)

func SetMount(podSpec *v1.PodSpec, service Service, cfg *Config) {
	podSpec.Containers = append(podSpec.Containers, secondPodSpec(service, cfg))
	addVolume(podSpec)
}

func addVolume(podSpec *v1.PodSpec) {
	hostToContainer := v1.MountPropagationHostToContainer
	volumeMountShare := v1.VolumeMount{
		Name:             s3_fs_fuse_volume_name,
		MountPath:        s3_fs_fuse_folder_mount,
		MountPropagation: &hostToContainer,
	}
	volumeshare := v1.Volume{
		Name: s3_fs_fuse_volume_name,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volumeMountShare)
	podSpec.Volumes = append(podSpec.Volumes, volumeshare)
}

func secondPodSpec(service Service, cfg *Config) v1.Container {
	bidirectional := v1.MountPropagationBidirectional
	//tr := true
	var ptr *bool // Uninitialized pointer
	value := true
	ptr = &value
	container := v1.Container{
		Name:    s3_fs_fuse_containerName,
		Image:   s3_fs_fuse_containerImage,
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", s3_fs_fuse_commandImage},
		Ports: []v1.ContainerPort{
			{
				Name:          "",
				ContainerPort: 9000,
			},
		},
		SecurityContext: &v1.SecurityContext{Privileged: ptr},
		Env: []v1.EnvVar{
			{
				Name:  "MINIO_BUCKET",
				Value: "jupyter-bucket",
			},
			{
				Name:  "MINIO_ENDPOINT",
				Value: "minio.minio.svc.cluster.local",
			},
			{
				Name:  "MNT_POINT",
				Value: s3_fs_fuse_folder_mount,
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:             s3_fs_fuse_volume_name,
				MountPath:        s3_fs_fuse_folder_mount,
				MountPropagation: &bidirectional,
			},
		},
	}

	credentialsValue := setCredentials(service, cfg)
	container.Env = append(container.Env, credentialsValue[0])
	container.Env = append(container.Env, credentialsValue[1])
	return container

}

func setCredentials(service Service, cfg *Config) []v1.EnvVar {
	if service.Owner == "" {
		credentials := []v1.EnvVar{
			{
				Name:  "AWS_ACCESS_KEY_ID",
				Value: cfg.MinIOProvider.AccessKey,
			},
			{
				Name:  "AWS_SECRET_ACCESS_KEY",
				Value: cfg.MinIOProvider.SecretKey,
			},
		}
		return credentials
	} else {
		credentials := []v1.EnvVar{
			{
				Name: "AWS_ACCESS_KEY_ID",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{Name: strings.Trim(service.Owner, "@egi.eu")},
						Key:                  "accessKey",
					},
				},
			},
			{
				Name: "AWS_SECRET_ACCESS_KEY",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{Name: strings.Trim(service.Owner, "@egi.eu")},
						Key:                  "secretKey",
					},
				},
			},
		}
		return credentials
	}
}
