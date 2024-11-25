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
	rcloneContainerName  = "rclone-container"
	rcloneContainerImage = "rclone/rclone"
	minioCommand         = `mkdir -p $MNT_POINT/$MINIO_BUCKET
rclone config create minio s3  provider=Minio access_key_id=$AWS_ACCESS_KEY_ID secret_access_key=$AWS_SECRET_ACCESS_KEY endpoint=$MINIO_ENDPOINT acl=public-read-write
rclone mount minio:/$MINIO_BUCKET $MNT_POINT/$MINIO_BUCKET `
	webdavCommand = `mkdir -p $MNT_POINT/$WEBDAV_FOLDER
rclone config create dcache webdav url=$WEBDAV_HOSTNAME vendor=other user=$WEBDAV_LOGIN pass=$WEBDAV_PASSWORD
rclone mount dcache:$WEBDAV_FOLDER $MNT_POINT/$WEBDAV_FOLDER --vfs-cache-mode full `
	communCommand = `--dir-cache-time 10s --allow-other --allow-non-empty --umask 0007 --uid 1000 --gid 100 --allow-other  --no-checksum &
pid=$!
while true; do
	if [ -f /tmpfolder/finish-file ]; then
		kill $pid
		exit 0
	fi
	sleep 5
done`
	rcloneFolderMount    = "/mnt"
	rcloneVolumeName     = "shared-data"
	ephemeralVolumeName  = "ephemeral-data"
	ephemeralVolumeMount = "/tmpfolder"
)

// SetMount Creates the sidecar container that mounts the source volume onto the pod volume
func SetMount(podSpec *v1.PodSpec, service Service, cfg *Config) {
	podSpec.Containers = append(podSpec.Containers, sidecarPodSpec(service))
	termination := int64(5)
	podSpec.TerminationGracePeriodSeconds = &termination
	addVolume(podSpec)
}

func addVolume(podSpec *v1.PodSpec) {
	hostToContainer := v1.MountPropagationHostToContainer
	volumeMountShare := v1.VolumeMount{
		Name:             rcloneVolumeName,
		MountPath:        rcloneFolderMount,
		MountPropagation: &hostToContainer,
	}
	volumeshare := v1.Volume{
		Name: rcloneVolumeName,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	ephemeralvolumeMountShare := v1.VolumeMount{
		Name:             ephemeralVolumeName,
		MountPath:        ephemeralVolumeMount,
		MountPropagation: &hostToContainer,
	}
	ephemeralvolumeshare := v1.Volume{
		Name: ephemeralVolumeName,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volumeMountShare)
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, ephemeralvolumeMountShare)
	podSpec.Volumes = append(podSpec.Volumes, volumeshare)
	podSpec.Volumes = append(podSpec.Volumes, ephemeralvolumeshare)
}

func sidecarPodSpec(service Service) v1.Container {
	bidirectional := v1.MountPropagationBidirectional
	var ptr *bool // Uninitialized pointer
	value := true
	ptr = &value
	container := v1.Container{
		Name:    rcloneContainerName,
		Image:   rcloneContainerImage,
		Command: []string{"/bin/sh"},
		//Args:    []string{"-c", rcloneStartCommand},
		Ports: []v1.ContainerPort{
			{
				ContainerPort: 9000,
			},
		},
		SecurityContext: &v1.SecurityContext{Privileged: ptr},
		Env: []v1.EnvVar{
			{
				Name:  "MNT_POINT",
				Value: rcloneFolderMount,
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:             rcloneVolumeName,
				MountPath:        rcloneFolderMount,
				MountPropagation: &bidirectional,
			},
			{
				Name:             ephemeralVolumeName,
				MountPath:        ephemeralVolumeMount,
				MountPropagation: &bidirectional,
			},
		},
	}

	provider := strings.Split(service.Mount.Provider, ".")
	if provider[0] == MinIOName {
		MinIOEnvVars := setMinIOEnvVars(service, provider[1])
		container.Env = append(container.Env, MinIOEnvVars...)
		container.Args = []string{"-c", minioCommand + communCommand}
	}
	if provider[0] == WebDavName {
		WebDavEnvVars := setWebDavEnvVars(service, provider[1])
		container.Env = append(container.Env, WebDavEnvVars...)
		container.Args = []string{"-c", webdavCommand + communCommand}
	}
	return container

}

func setMinIOEnvVars(service Service, providerId string) []v1.EnvVar {
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

func setWebDavEnvVars(service Service, providerId string) []v1.EnvVar {
	//service.Mount.Provider
	credentials := []v1.EnvVar{
		{
			Name:  "WEBDAV_FOLDER",
			Value: service.Mount.Path,
		},
		{
			Name:  "WEBDAV_LOGIN",
			Value: service.StorageProviders.WebDav[providerId].Login,
		},
		{
			Name:  "WEBDAV_PASSWORD",
			Value: service.StorageProviders.WebDav[providerId].Password,
		},
		{
			Name:  "WEBDAV_HOSTNAME",
			Value: "https://" + service.StorageProviders.WebDav[providerId].Hostname,
		},
	}
	return credentials
}
