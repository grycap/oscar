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
	Rclone_containerName  = "rclone-container"
	rclone_containerImage = "rclone/rclone"
	rclone_commandImage   = `mkdir -p $MNT_POINT/$MINIO_BUCKET
rclone config create minio s3  provider=Minio access_key_id=$AWS_ACCESS_KEY_ID secret_access_key=$AWS_SECRET_ACCESS_KEY endpoint=$MINIO_ENDPOINT acl=public-read-write
rclone mount minio:/$MINIO_BUCKET $MNT_POINT/$MINIO_BUCKET --dir-cache-time 10s --allow-other --allow-non-empty --umask 0007 --uid 1000 --gid 100 --allow-other  --no-checksum &
pid=$!
while true; do
	if [ -f /tmpfolder/finish-file ]; then
		kill $pid
		exit 0
	fi
	sleep 5
done`
	rclone_folder_mount    = "/mnt"
	rclone_volume_name     = "shared-data"
	ephemeral_volume_name  = "ephemeral-data"
	ephemeral_volume_mount = "/tmpfolder"
)

func SetMount(podSpec *v1.PodSpec, service Service, cfg *Config) {
	podSpec.Containers = append(podSpec.Containers, secondPodSpec(service, cfg))
	termination := int64(5)
	podSpec.TerminationGracePeriodSeconds = &termination
	addVolume(podSpec, service, cfg)
}

func addVolume(podSpec *v1.PodSpec, service Service, cfg *Config) {
	hostToContainer := v1.MountPropagationHostToContainer
	volumeMountShare := v1.VolumeMount{
		Name:             rclone_volume_name,
		MountPath:        rclone_folder_mount,
		MountPropagation: &hostToContainer,
	}
	volumeshare := v1.Volume{
		Name: rclone_volume_name,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	ephemeralvolumeMountShare := v1.VolumeMount{
		Name:             ephemeral_volume_name,
		MountPath:        ephemeral_volume_mount,
		MountPropagation: &hostToContainer,
	}
	ephemeralvolumeshare := v1.Volume{
		Name: ephemeral_volume_name,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volumeMountShare)
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, ephemeralvolumeMountShare)
	podSpec.Volumes = append(podSpec.Volumes, volumeshare)
	podSpec.Volumes = append(podSpec.Volumes, ephemeralvolumeshare)
}

func secondPodSpec(service Service, cfg *Config) v1.Container {
	bidirectional := v1.MountPropagationBidirectional
	var ptr *bool // Uninitialized pointer
	value := true
	ptr = &value
	container := v1.Container{
		Name:    Rclone_containerName,
		Image:   rclone_containerImage,
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", rclone_commandImage},
		Ports: []v1.ContainerPort{
			{
				Name:          "",
				ContainerPort: 9000,
			},
		},
		SecurityContext: &v1.SecurityContext{Privileged: ptr},
		Env: []v1.EnvVar{
			{
				Name:  "MNT_POINT",
				Value: rclone_folder_mount,
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:             rclone_volume_name,
				MountPath:        rclone_folder_mount,
				MountPropagation: &bidirectional,
			},
			{
				Name:             ephemeral_volume_name,
				MountPath:        ephemeral_volume_mount,
				MountPropagation: &bidirectional,
			},
		},
	}

	provider := strings.Split(service.Mount.Provider, ".")
	if provider[0] == MinIOName {
		credentialsValue := setCredentialsMinIO(service, cfg, provider[1])
		for index := 0; index < len(credentialsValue); index++ {
			container.Env = append(container.Env, credentialsValue[index])
		}
	}
	return container

}

func setCredentialsMinIO(service Service, cfg *Config, providerId string) []v1.EnvVar {
	//service.Mount.Provider
	credentials := []v1.EnvVar{
		{
			Name:  "MINIO_BUCKET",
			Value: service.Mount.Path,
		},
		{
			Name:  "AWS_ACCESS_KEY_ID",
			Value: service.StorageProviders.MinIO[providerId].AccessKey,
		},
		{
			Name:  "AWS_SECRET_ACCESS_KEY",
			Value: service.StorageProviders.MinIO[providerId].SecretKey,
		},
		{
			Name:  "MINIO_ENDPOINT",
			Value: service.StorageProviders.MinIO[providerId].Endpoint,
		},
	}
	return credentials
}
