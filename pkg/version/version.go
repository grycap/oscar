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

package version

import (
	"runtime"

	"github.com/grycap/oscar/v2/pkg/types"
	"k8s.io/client-go/kubernetes"
)

var (
	// Version release version
	Version string

	// GitCommit SHA of last git commit
	GitCommit string
)

// GetInfo returns version info
func GetInfo(kubeClientset *kubernetes.Clientset, back types.ServerlessBackend) types.Info {
	version := "devel"

	if Version != "" {
		version = Version
	}

	return types.Info{
		Version:               version,
		GitCommit:             GitCommit,
		Architecture:          runtime.GOARCH,
		KubeVersion:           getKubeVersion(kubeClientset),
		ServerlessBackendInfo: back.GetInfo(),
	}

}

func getKubeVersion(kubeClientset *kubernetes.Clientset) string {
	version, err := kubeClientset.Discovery().ServerVersion()
	if err != nil {
		return ""
	}

	return version.String()
}
