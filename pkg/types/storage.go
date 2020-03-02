// Copyright (C) GRyCAP - I3M - UPV
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import "net/url"

// StorageIOConfig provides the storage input/output configuration for services
type StorageIOConfig struct {
	Provider string   `json:"storage_provider"`
	Path     string   `json:"path"`
	Suffix   []string `json:"suffix,omitempty"`
	Prefix   []string `json:"prefix,omitempty"`
}

// StorageProviders stores the credentials of all supported storage providers
type StorageProviders struct {
	S3      *S3Provider      `json:"s3"`
	MinIO   *MinIOProvider   `json:"minio"`
	Onedata *OnedataProvider `json:"onedata"`
}

// S3Provider stores the credentials of the AWS S3 storage provider
type S3Provider struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
}

// MinIOProvider stores the credentials of the MinIO storage provider
type MinIOProvider struct {
	Endpoint  url.URL `json:"endpoint"`
	Verify    bool    `json:"verify"`
	AccessKey string  `json:"access_key"`
	SecretKey string  `json:"secret_key"`
	Region    string  `json:"region"`
}

// OnedataProvider stores the credentials of the Onedata storage provider
type OnedataProvider struct {
	OneproviderHost url.URL `json:"oneprovider_host"`
	Token           string  `json:"token"`
	Space           string  `json:"space"`
}
