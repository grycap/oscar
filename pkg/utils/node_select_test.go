package utils

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsControlPlaneNode(t *testing.T) {
	tests := []struct {
		name     string
		labels  map[string]string
		expected bool
	}{
		{"Control plane label", map[string]string{"node-role.kubernetes.io/control-plane": ""}, true},
		{"Master label", map[string]string{"node-role.kubernetes.io/master": ""}, true},
		{"No labels", nil, false},
		{"Worker only", map[string]string{"node-role.kubernetes.io/worker": ""}, false},
		{"Empty labels", map[string]string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tt.labels,
				},
			}
			result := IsControlPlaneNode(node)
			if result != tt.expected {
				t.Errorf("IsControlPlaneNode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSelectEligibleNodes(t *testing.T) {
	t.Run("All worker nodes", func(t *testing.T) {
		nodes := []v1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"node-role.kubernetes.io/worker": ""}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"node-role.kubernetes.io/worker": ""}}},
		}
		result := SelectEligibleNodes(nodes)
		if len(result) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(result))
		}
	})

	t.Run("Mixed nodes", func(t *testing.T) {
		nodes := []v1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node3"}},
		}
		result := SelectEligibleNodes(nodes)
		if len(result) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(result))
		}
		for _, node := range result {
			if node.Name == "node2" {
				t.Error("Expected control-plane node to be excluded")
			}
		}
	})

	t.Run("Only control plane", func(t *testing.T) {
		nodes := []v1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"node-role.kubernetes.io/master": ""}}},
		}
		result := SelectEligibleNodes(nodes)
		if len(result) != 1 {
			t.Errorf("Expected 1 node, got %d", len(result))
		}
	})

	t.Run("Empty list", func(t *testing.T) {
		nodes := []v1.Node{}
		result := SelectEligibleNodes(nodes)
		if len(result) != 0 {
			t.Errorf("Expected 0 nodes, got %d", len(result))
		}
	})
}