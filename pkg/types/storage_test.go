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
	"fmt"
	"net/http"
	"testing"
)

func TestGetMinIOClient(t *testing.T) {
	scenarios := []struct {
		name          string
		minioProvider MinIOProvider
	}{
		{
			"no verify tls",
			MinIOProvider{
				Endpoint:  "https://test.endpoint",
				Verify:    false,
				AccessKey: "testaccesskey",
				SecretKey: "testsecretkey",
				Region:    "us-east-1",
			},
		},
		{
			"verify tls",
			MinIOProvider{
				Endpoint:  "https://test.endpoint",
				Verify:    true,
				AccessKey: "testaccesskey",
				SecretKey: "testsecretkey",
				Region:    "us-east-1",
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			client := s.minioProvider.GetS3Client()

			if client.Endpoint != s.minioProvider.Endpoint {
				t.Errorf("expected MinIO endpoint: %s, got: %s", s.minioProvider.Endpoint, client.Endpoint)
			}

			credentials, _ := client.Config.Credentials.Get()
			if credentials.AccessKeyID != s.minioProvider.AccessKey {
				t.Errorf("expected MinIO AccessKey: %s, got: %s", s.minioProvider.AccessKey, credentials.AccessKeyID)
			}

			if credentials.SecretAccessKey != s.minioProvider.SecretKey {
				t.Errorf("expected MinIO SecretKey: %s, got: %s", s.minioProvider.SecretKey, credentials.SecretAccessKey)
			}

			if *client.Config.Region != s.minioProvider.Region {
				t.Errorf("expected MinIO region: %s, got: %s", s.minioProvider.Region, *client.Config.Region)
			}

			if !s.minioProvider.Verify {
				transport := client.Config.HTTPClient.Transport.(*http.Transport)
				if !transport.TLSClientConfig.InsecureSkipVerify {
					t.Error("InsecureSkipVerify option not set")
				}
			}
		})
	}
}

func TestGetS3Client(t *testing.T) {
	s3Provider := S3Provider{
		AccessKey: "testaccesskey",
		SecretKey: "testsecretkey",
		Region:    "us-east-1",
	}

	client := s3Provider.GetS3Client()

	credentials, _ := client.Config.Credentials.Get()
	if credentials.AccessKeyID != s3Provider.AccessKey {
		t.Errorf("expected S3 AccessKey: %s, got: %s", s3Provider.AccessKey, credentials.AccessKeyID)
	}

	if credentials.SecretAccessKey != s3Provider.SecretKey {
		t.Errorf("expected S3 SecretKey: %s, got: %s", s3Provider.SecretKey, credentials.SecretAccessKey)
	}

	if *client.Config.Region != s3Provider.Region {
		t.Errorf("expected S3 region: %s, got: %s", s3Provider.Region, *client.Config.Region)
	}
}

func TestGetCDMIClient(t *testing.T) {
	onedataProvider := OnedataProvider{
		OneproviderHost: "test.host",
		Token:           "testtoken",
		Space:           "testspace",
	}

	client := onedataProvider.GetCDMIClient()

	expectedProviderHost := fmt.Sprintf("https://%s/cdmi", onedataProvider.OneproviderHost)
	if client.Endpoint.String() != expectedProviderHost {
		t.Errorf("expected Oneprovider host: %s, got: %s", onedataProvider.OneproviderHost, client.Endpoint)
	}
}
