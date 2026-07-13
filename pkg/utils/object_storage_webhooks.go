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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/grycap/oscar/v4/pkg/types"
)

// RegisterObjectStorageWebhook creates the per-service notification target in
// the configured object storage. RustFS exposes a native, plain-text admin API
// for dynamic configuration; MinIO uses the encrypted madmin API and restart.
func RegisterObjectStorageWebhook(ctx context.Context, cfg *types.Config, name, token string) error {
	if strings.EqualFold(strings.TrimSpace(cfg.ObjectStorageType), ObjectStorageRustFS) {
		rustFS, err := makeRustFSIAM(cfg)
		if err != nil {
			return err
		}
		if err := ensureRustFSNotificationsEnabled(ctx, rustFS); err != nil {
			return err
		}
		endpoint := fmt.Sprintf("http://%s.%s:%d/job/%s", cfg.Name, cfg.Namespace, cfg.ServicePort, name)
		payload, err := json.Marshal(struct {
			KeyValues []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"key_values"`
		}{KeyValues: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "endpoint", Value: endpoint},
			{Key: "auth_token", Value: token},
			{Key: "queue_dir", Value: ""},
		}})
		if err != nil {
			return fmt.Errorf("encoding RustFS webhook target: %w", err)
		}
		path := "/rustfs/admin/v3/target/notify_webhook/" + name
		return rustFS.requestPayload(ctx, http.MethodPut, path, nil, payload, "application/json")
	}

	client, err := MakeMinIOAdminClient(cfg)
	if err != nil {
		return err
	}
	if err := client.RegisterWebhook(name, token); err != nil {
		return err
	}
	return client.RestartServer()
}

func ensureRustFSNotificationsEnabled(ctx context.Context, rustFS *rustFSIAM) error {
	response, err := rustFS.requestPayloadResponse(ctx, http.MethodGet, "/rustfs/admin/v3/module-switches", nil, nil, "")
	if err != nil {
		return fmt.Errorf("reading RustFS notification module state: %w", err)
	}
	var switches struct {
		NotifyEnabled bool `json:"notify_enabled"`
		AuditEnabled  bool `json:"audit_enabled"`
	}
	if err := json.Unmarshal(response, &switches); err != nil {
		return fmt.Errorf("decoding RustFS notification module state: %w", err)
	}
	if switches.NotifyEnabled {
		return nil
	}
	payload, err := json.Marshal(struct {
		NotifyEnabled bool `json:"notify_enabled"`
		AuditEnabled  bool `json:"audit_enabled"`
	}{NotifyEnabled: true, AuditEnabled: switches.AuditEnabled})
	if err != nil {
		return fmt.Errorf("encoding RustFS notification module state: %w", err)
	}
	if err := rustFS.requestPayload(ctx, http.MethodPut, "/rustfs/admin/v3/module-switches", nil, payload, "application/json"); err != nil {
		return fmt.Errorf("enabling RustFS notification module: %w", err)
	}
	return nil
}

// RemoveObjectStorageWebhook removes a per-service notification target.
func RemoveObjectStorageWebhook(ctx context.Context, cfg *types.Config, name string) error {
	if strings.EqualFold(strings.TrimSpace(cfg.ObjectStorageType), ObjectStorageRustFS) {
		rustFS, err := makeRustFSIAM(cfg)
		if err != nil {
			return err
		}
		path := "/rustfs/admin/v3/target/notify_webhook/" + name + "/reset"
		return rustFS.requestPayload(ctx, http.MethodDelete, path, nil, nil, "")
	}

	client, err := MakeMinIOAdminClient(cfg)
	if err != nil {
		return err
	}
	if err := client.RemoveWebhook(name); err != nil {
		return err
	}
	return client.RestartServer()
}

func makeRustFSIAM(cfg *types.Config) (*rustFSIAM, error) {
	iam, err := MakeObjectStorageIAM(cfg)
	if err != nil {
		return nil, err
	}
	rustFS, ok := iam.(*rustFSIAM)
	if !ok {
		return nil, fmt.Errorf("expected RustFS IAM adapter, got %T", iam)
	}
	return rustFS, nil
}
