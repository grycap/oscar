/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestJobInfoJSONSerialization(t *testing.T) {
	// Test complete JobInfo
	now := metav1.Now()
	jobInfo := JobInfo{
		Status:       "running",
		CreationTime: &now,
		StartTime:    &now,
		FinishTime:   &now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(jobInfo)
	if err != nil {
		t.Fatalf("Failed to marshal JobInfo: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled JobInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal JobInfo: %v", err)
	}

	if unmarshaled.Status != jobInfo.Status {
		t.Errorf("Expected status %s, got %s", jobInfo.Status, unmarshaled.Status)
	}

	if unmarshaled.CreationTime == nil {
		t.Error("Expected CreationTime to be set")
	}

	if unmarshaled.StartTime == nil {
		t.Error("Expected StartTime to be set")
	}

	if unmarshaled.FinishTime == nil {
		t.Error("Expected FinishTime to be set")
	}
}

func TestJobInfoPartialFields(t *testing.T) {
	// Test JobInfo with only required fields
	jobInfo := JobInfo{
		Status: "completed",
	}

	data, err := json.Marshal(jobInfo)
	if err != nil {
		t.Fatalf("Failed to marshal partial JobInfo: %v", err)
	}

	var unmarshaled JobInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal partial JobInfo: %v", err)
	}

	if unmarshaled.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", unmarshaled.Status)
	}

	// Optional fields should be nil
	if unmarshaled.CreationTime != nil {
		t.Error("Expected CreationTime to be nil")
	}

	if unmarshaled.StartTime != nil {
		t.Error("Expected StartTime to be nil")
	}

	if unmarshaled.FinishTime != nil {
		t.Error("Expected FinishTime to be nil")
	}
}

func TestJobsResponseJSONSerialization(t *testing.T) {
	now := metav1.Now()
	remaining := int64(5)

	jobs := map[string]*JobInfo{
		"job1": {
			Status:       "running",
			CreationTime: &now,
			StartTime:    &now,
		},
		"job2": {
			Status:     "completed",
			FinishTime: &now,
		},
	}

	jobsResponse := JobsResponse{
		Jobs:         jobs,
		NextPage:     "next-page-token",
		RemainingJob: &remaining,
	}

	// Test JSON marshaling
	data, err := json.Marshal(jobsResponse)
	if err != nil {
		t.Fatalf("Failed to marshal JobsResponse: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled JobsResponse
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal JobsResponse: %v", err)
	}

	// Check jobs map
	if len(unmarshaled.Jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(unmarshaled.Jobs))
	}

	job1, exists := unmarshaled.Jobs["job1"]
	if !exists {
		t.Error("Expected job1 to exist in jobs map")
	}

	if job1.Status != "running" {
		t.Errorf("Expected job1 status 'running', got '%s'", job1.Status)
	}

	job2, exists := unmarshaled.Jobs["job2"]
	if !exists {
		t.Error("Expected job2 to exist in jobs map")
	}

	if job2.Status != "completed" {
		t.Errorf("Expected job2 status 'completed', got '%s'", job2.Status)
	}

	// Check optional fields
	if unmarshaled.NextPage != "next-page-token" {
		t.Errorf("Expected NextPage 'next-page-token', got '%s'", unmarshaled.NextPage)
	}

	if unmarshaled.RemainingJob == nil {
		t.Error("Expected RemainingJob to be set")
	} else if *unmarshaled.RemainingJob != 5 {
		t.Errorf("Expected RemainingJob 5, got %d", *unmarshaled.RemainingJob)
	}
}

func TestJobsResponseMinimal(t *testing.T) {
	jobsResponse := JobsResponse{
		Jobs: map[string]*JobInfo{
			"job1": {
				Status: "pending",
			},
		},
	}

	data, err := json.Marshal(jobsResponse)
	if err != nil {
		t.Fatalf("Failed to marshal minimal JobsResponse: %v", err)
	}

	var unmarshaled JobsResponse
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal minimal JobsResponse: %v", err)
	}

	if len(unmarshaled.Jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(unmarshaled.Jobs))
	}

	if unmarshaled.NextPage != "" {
		t.Errorf("Expected empty NextPage, got '%s'", unmarshaled.NextPage)
	}

	if unmarshaled.RemainingJob != nil {
		t.Error("Expected RemainingJob to be nil")
	}
}

func TestJobInfoJSONTags(t *testing.T) {
	// Test that JSON tags work correctly
	jobInfo := JobInfo{
		Status: "running",
	}

	data, err := json.Marshal(jobInfo)
	if err != nil {
		t.Fatalf("Failed to marshal JobInfo: %v", err)
	}

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// Check that JSON field names match the tags
	if _, exists := raw["status"]; !exists {
		t.Error("Expected 'status' field in JSON")
	}

	if _, exists := raw["creation_time"]; exists {
		t.Error("Expected 'creation_time' field to be omitted when nil")
	}

	if _, exists := raw["start_time"]; exists {
		t.Error("Expected 'start_time' field to be omitted when nil")
	}

	if _, exists := raw["finish_time"]; exists {
		t.Error("Expected 'finish_time' field to be omitted when nil")
	}
}

func TestJobsResponseJSONTags(t *testing.T) {
	jobsResponse := JobsResponse{
		Jobs: map[string]*JobInfo{},
	}

	data, err := json.Marshal(jobsResponse)
	if err != nil {
		t.Fatalf("Failed to marshal JobsResponse: %v", err)
	}

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// Check that JSON field names match the tags
	if _, exists := raw["jobs"]; !exists {
		t.Error("Expected 'jobs' field in JSON")
	}

	if _, exists := raw["next_page"]; exists {
		t.Error("Expected 'next_page' field to be omitted when empty")
	}

	if _, exists := raw["remaining_jobs"]; exists {
		t.Error("Expected 'remaining_jobs' field to be omitted when nil")
	}
}

func TestJobInfoTimeFields(t *testing.T) {
	// Test with different time values
	past := time.Now().Add(-1 * time.Hour)
	now := time.Now()
	future := time.Now().Add(1 * time.Hour)

	creationTime := metav1.NewTime(past)
	startTime := metav1.NewTime(now)
	finishTime := metav1.NewTime(future)

	jobInfo := JobInfo{
		Status:       "completed",
		CreationTime: &creationTime,
		StartTime:    &startTime,
		FinishTime:   &finishTime,
	}

	data, err := json.Marshal(jobInfo)
	if err != nil {
		t.Fatalf("Failed to marshal JobInfo with times: %v", err)
	}

	var unmarshaled JobInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal JobInfo with times: %v", err)
	}

	// Check that time fields are set
	if unmarshaled.CreationTime == nil {
		t.Error("Expected CreationTime to be set")
	}

	if unmarshaled.StartTime == nil {
		t.Error("Expected StartTime to be set")
	}

	if unmarshaled.FinishTime == nil {
		t.Error("Expected FinishTime to be set")
	}

	// Check that times are approximately equal (allowing for JSON serialization precision)
	if !unmarshaled.CreationTime.Time.Truncate(time.Second).Equal(past.Truncate(time.Second)) {
		t.Errorf("Expected CreationTime approximately %v, got %v", past, unmarshaled.CreationTime.Time)
	}

	if !unmarshaled.StartTime.Time.Truncate(time.Second).Equal(now.Truncate(time.Second)) {
		t.Errorf("Expected StartTime approximately %v, got %v", now, unmarshaled.StartTime.Time)
	}

	if !unmarshaled.FinishTime.Time.Truncate(time.Second).Equal(future.Truncate(time.Second)) {
		t.Errorf("Expected FinishTime approximately %v, got %v", future, unmarshaled.FinishTime.Time)
	}
}
