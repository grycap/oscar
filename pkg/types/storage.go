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
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/grycap/cdmi-client-go"
)

const (
	// DefaultProvider string identifier for the default StorageProvider
	DefaultProvider = "default"

	// MinIOName string representing the MinIO provider name
	MinIOName = "minio"

	// S3Name string representing the S3 provider name
	S3Name = "s3"

	// OnedataName string representing the Onedata provider name
	OnedataName = "onedata"

	// ProviderSeparator separator character used for split provider's name and identifier
	ProviderSeparator = "."
)

// StorageIOConfig provides the storage input/output configuration for services
type StorageIOConfig struct {
	// Provider reference to the provider's name and identifier specified in StorageProviders
	// The provider's name is separated from the ID by a point (e.g. "minio.myidentifier")
	Provider string   `json:"storage_provider"`
	Path     string   `json:"path"`
	Suffix   []string `json:"suffix,omitempty"`
	Prefix   []string `json:"prefix,omitempty"`
}

// StorageProviders stores the credentials of all supported storage providers
type StorageProviders struct {
	S3      map[string]*S3Provider      `json:"s3,omitempty"`
	MinIO   map[string]*MinIOProvider   `json:"minio,omitempty"`
	Onedata map[string]*OnedataProvider `json:"onedata,omitempty"`
}

// S3Provider stores the credentials of the AWS S3 storage provider
type S3Provider struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
}

// MinIOProvider stores the credentials of the MinIO storage provider
type MinIOProvider struct {
	Endpoint  string `json:"endpoint"`
	Verify    bool   `json:"verify"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
}

// OnedataProvider stores the credentials of the Onedata storage provider
type OnedataProvider struct {
	OneproviderHost string `json:"oneprovider_host"`
	Token           string `json:"token"`
	Space           string `json:"space"`
}

// GetS3Client creates a new S3 Client from a S3Provider
func (s3Provider S3Provider) GetS3Client() *s3.S3 {
	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(s3Provider.AccessKey, s3Provider.SecretKey, ""),
		Region:      aws.String(s3Provider.Region),
	}

	s3Session := session.New(s3Config)

	return s3.New(s3Session)
}

// GetS3Client creates a new S3 Client from a MinIOProvider
func (minIOProvider MinIOProvider) GetS3Client() *s3.S3 {
	s3MinIOConfig := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(minIOProvider.AccessKey, minIOProvider.SecretKey, ""),
		Endpoint:         aws.String(minIOProvider.Endpoint),
		Region:           aws.String(minIOProvider.Region),
		S3ForcePathStyle: aws.Bool(true),
	}

	// Disable tls verification in client transport if Verify == false
	if !minIOProvider.Verify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		s3MinIOConfig.HTTPClient = &http.Client{Transport: tr}
	}

	minIOSession := session.New(s3MinIOConfig)

	return s3.New(minIOSession)
}

// GetCDMIClient creates a new CDMI Client from a OnedataProvider
func (onedataProvider OnedataProvider) GetCDMIClient() *cdmi.Client {
	opHost := strings.TrimRight(onedataProvider.OneproviderHost, "/ ")
	// OneproviderHost must contain the "/cdmi" path for creating the CDMI client
	opHostCDMI, _ := url.Parse(fmt.Sprintf("https://%s/cdmi", opHost))

	return cdmi.New(opHostCDMI, onedataProvider.Token, true)
}
