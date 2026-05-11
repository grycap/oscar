package utils

import v1 "k8s.io/api/core/v1"

// IsControlPlaneNode returns true if the node is labeled as control-plane/master.
func IsControlPlaneNode(node v1.Node) bool {
	if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
		return true
	}
	if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
		return true
	}
	return false
}

// SelectEligibleNodes prefers worker nodes, falling back to control-plane nodes
// when no workers are available.
func SelectEligibleNodes(nodes []v1.Node) []v1.Node {
	workerNodes := make([]v1.Node, 0, len(nodes))
	for _, node := range nodes {
		if IsControlPlaneNode(node) {
			continue
		}
		workerNodes = append(workerNodes, node)
	}
	if len(workerNodes) > 0 {
		return workerNodes
	}
	return nodes
}
