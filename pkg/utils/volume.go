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

package utils

import (
	"fmt"
	"path"
	"strings"

	"github.com/grycap/oscar/v3/pkg/types"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
)

func ValidateManagedVolumeCreateRequest(req *types.ManagedVolumeCreateRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Size = strings.TrimSpace(req.Size)
	if errs := validationName(req.Name); errs != nil {
		return errs
	}
	if errs := validateSize(req.Size); errs != nil {
		return errs
	}
	return nil
}

func validationName(name string) error {
	if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("volume.name must satisfy Kubernetes DNS-1123 naming rules")
	}
	return nil
}

func validateSize(size string) error {
	qty, err := resource.ParseQuantity(size)
	if err != nil || qty.Sign() <= 0 {
		return fmt.Errorf("volume.size must be a valid positive quantity")
	}
	return nil

}

func ValidateVolumeConfig(serviceName string, volume *types.ServiceVolumeConfig) error {
	if volume == nil {
		return nil
	}
	volume.Name = strings.TrimSpace(volume.Name)
	volume.Size = strings.TrimSpace(volume.Size)
	volume.MountPath = strings.TrimSpace(volume.MountPath)
	volume.LifecyclePolicy = strings.TrimSpace(volume.LifecyclePolicy)

	if volume.MountPath == "" {
		return fmt.Errorf("volume.mount_path is required")
	}

	if volume.Size == "" && volume.Name == "" {
		return fmt.Errorf("volume.name is required when volume.size is not set")
	}

	if volume.Name != "" {
		if errs := validationName(volume.Name); errs != nil {
			return errs
		}
	}

	if volume.Size != "" {
		if errs := validateSize(volume.Size); errs != nil {
			return errs
		}
		if volume.Name == "" {
			volume.Name = serviceName
		}
		switch volume.LifecyclePolicy {
		case "":
			volume.LifecyclePolicy = types.VolumeLifecycleDelete
		case types.VolumeLifecycleDelete, types.VolumeLifecycleRetain:
		default:
			return fmt.Errorf("volume.lifecycle_policy must be either %q or %q", types.VolumeLifecycleDelete, types.VolumeLifecycleRetain)
		}
	} else if volume.LifecyclePolicy != "" {
		return fmt.Errorf("volume.lifecycle_policy is only valid when volume.size is set")
	}

	if !path.IsAbs(volume.MountPath) {
		return fmt.Errorf("volume.mount_path must be an absolute path")
	}
	if volume.MountPath == types.ConfigPath || volume.MountPath == types.VolumePath ||
		strings.HasPrefix(volume.MountPath, types.ConfigPath+"/") || strings.HasPrefix(volume.MountPath, types.VolumePath+"/") {
		return fmt.Errorf("volume.mount_path cannot overlap OSCAR reserved paths")
	}
	return nil
}

func SameVolumeConfig(a, b *types.ServiceVolumeConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Name == b.Name && a.Size == b.Size && a.MountPath == b.MountPath && a.LifecyclePolicy == b.LifecyclePolicy
}
