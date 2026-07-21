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

package objectstorage

import (
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
)

func TestMakeObjectStorageIAM(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		assertType  func(ObjectStorageIAM) bool
	}{
		{
			name:        "default is MinIO",
			storageType: "",
			assertType: func(iam ObjectStorageIAM) bool {
				_, ok := iam.(*types.MinIOIAM)
				return ok
			},
		},
		{
			name:        "explicit MinIO",
			storageType: "MINIO",
			assertType: func(iam ObjectStorageIAM) bool {
				_, ok := iam.(*types.MinIOIAM)
				return ok
			},
		},
		{
			name:        "RustFS",
			storageType: "rustfs",
			assertType: func(iam ObjectStorageIAM) bool {
				_, ok := iam.(*types.RustFSIAM)
				return ok
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := objectStorageIAMTestConfig(tt.storageType)
			iam, err := MakeObjectStorageIAM(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.assertType(iam) {
				t.Fatalf("unexpected IAM implementation %T", iam)
			}
		})
	}
}

func TestMakeObjectStorageIAMRejectsUnknownType(t *testing.T) {
	cfg := objectStorageIAMTestConfig("unknown")
	if _, err := MakeObjectStorageIAM(cfg); err == nil {
		t.Fatal("expected unsupported object storage type error")
	}
}

func objectStorageIAMTestConfig(storageType string) *types.Config {
	return &types.Config{
		ObjectStorageType: storageType,
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://object-storage.example.test:9000",
			AccessKey: "admin",
			SecretKey: "secret",
			Region:    "us-east-1",
		},
		Name:        "oscar",
		Namespace:   "oscar",
		ServicePort: 8080,
	}
}
