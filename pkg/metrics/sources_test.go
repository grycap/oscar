package metrics

import (
	"testing"
)

func TestJoinErrors(t *testing.T) {
	tests := []struct {
		name     string
		errs    []error
		expected string
	}{
		{"Nil and error", []error{nil, nil}, ""},
		{"One error", []error{errNoop{}}, "test error"},
		{"Multiple errors", []error{errNoop{"a"}, errNoop{"b"}}, "a; b"},
		{"Nil in middle", []error{errNoop{"a"}, nil, errNoop{"b"}}, "a; b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinErrors(tt.errs...)
			if result != tt.expected {
				t.Errorf("joinErrors() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseServiceFromPath(t *testing.T) {
	tests := []struct {
		name       string
		path      string
		serviceID string
		reqType   RequestType
	}{
		{"Job path", "/job/myservice", "myservice", RequestAsync},
		{"Run path", "/run/myservice", "myservice", RequestSync},
		{"Other path", "/other/path", "", ""},
		{"Empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceID, reqType := parseServiceFromPath(tt.path)
			if serviceID != tt.serviceID {
				t.Errorf("parseServiceFromPath() serviceID = %q, want %q", serviceID, tt.serviceID)
			}
			if reqType != tt.reqType {
				t.Errorf("parseServiceFromPath() reqType = %q, want %q", reqType, tt.reqType)
			}
		})
	}
}

func TestParseExposedServiceFromPath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		serviceID  string
		ok         bool
	}{
		{"Valid path", "/system/services/svc/exposed", "svc", true},
		{"Invalid path", "/system/services/svc", "", false},
		{"Wrong format", "/other/path", "", false},
		{"Empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceID, ok := parseExposedServiceFromPath(tt.path)
			if ok != tt.ok {
				t.Errorf("parseExposedServiceFromPath() ok = %v, want %v", ok, tt.ok)
			}
			if serviceID != tt.serviceID {
				t.Errorf("parseExposedServiceFromPath() serviceID = %q, want %q", serviceID, tt.serviceID)
			}
		})
	}
}

func TestCountryFromLokiLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels  map[string]string
		country string
	}{
		{"With country code", map[string]string{"geoip_country_code": "US"}, "US"},
		{"With country name", map[string]string{"geoip_country_name": "Spain"}, "Spain"},
		{"Nil", nil, ""},
		{"Empty", map[string]string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countryFromLokiLabels(tt.labels)
			if result != tt.country {
				t.Errorf("countryFromLokiLabels() = %q, want %q", result, tt.country)
			}
		})
	}
}

func TestOkStatus(t *testing.T) {
	status := okStatus("test", "note")
	if status.Name != "test" {
		t.Errorf("okStatus() Name = %q, want %q", status.Name, "test")
	}
	if status.Status != "ok" {
		t.Errorf("okStatus() Status = %q, want %q", status.Status, "ok")
	}
	if status.Notes != "note" {
		t.Errorf("okStatus() Notes = %q, want %q", status.Notes, "note")
	}
}

func TestMissingStatus(t *testing.T) {
	status := missingStatus("test", errNoop{})
	if status.Name != "test" {
		t.Errorf("missingStatus() Name = %q, want %q", status.Name, "test")
	}
	if status.Status != "missing" {
		t.Errorf("missingStatus() Status = %q, want %q", status.Status, "missing")
	}
}

type errNoop struct {
	msg string
}

func (e errNoop) Error() string {
	if e.msg == "" {
		return "test error"
	}
	return e.msg
}