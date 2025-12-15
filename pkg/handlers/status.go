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

package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"

	"strings"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

//  ROOT STRUCTURE

// Enhanced struct to store both display strings and int64 values
type NodeInfoWithAllocatable struct {
	NodeDetail        types.NodeDetail
	CPUAllocatable    int64
	MemoryAllocatable int64
	GPUAllocatable    int64
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// MakeStatusHandler godoc
// @Summary Get status
// @Description Retrieve cluster, OSCAR deployment and MinIO status data.
// @Tags status
// @Produce json
// @Success 200 {object} types.StatusInfo
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/status [get]
func MakeStatusHandler(cfg *types.Config, kubeClientset kubernetes.Interface, metricsClientset versioned.MetricsV1beta1Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		clusterInfo := types.StatusInfo{}
		var isAdmin bool = false
		if len(strings.Split(authHeader, "Bearer")) > 1 {
			uid, err := auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}
			if slices.Contains(cfg.UsersAdmin, uid) {
				isAdmin = true
			}
		} else {
			// If there is no Bearer token, we assume it is an admin by default (original behavior)
			isAdmin = true
		}

		// Initialize ClusterInfo
		clusterInfo.Cluster.Nodes = make([]types.NodeDetail, 0)
		clusterInfo.Cluster.Metrics.GPU.TotalGPU = 0

		nodeInfoMap, err := getNodesInfo(kubeClientset, &clusterInfo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error getting node info: %v", err)})
			return
		}

		err = getMetricsInfo(kubeClientset, metricsClientset, nodeInfoMap, &clusterInfo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error getting metrics info: %v", err)})
			return
		}
		err = getDeploymentInfo(kubeClientset, cfg, &clusterInfo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error getting OSCAR deployment: %v", err)})
			return
		}

		if isAdmin {
			err = getJobsInfo(cfg, kubeClientset, &clusterInfo)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error getting OSCAR jobs: %v", err)})
				return
			}

			err = getMinioInfo(cfg, &clusterInfo)
			if err != nil {
				// Logs the error but proceeds, as MinIO might not be essential for the status response to function
				log.Printf("Error getting MinIO info (might be normal if not configured or client fails): %v", err)
			}
		}

		c.JSON(http.StatusOK, clusterInfo)
	}
}

// Helper function to check if a node is an interLink node
func checkIfInterLinkNode(node v1.Node) bool {
	// Check for the specific interLink label
	if nodeType, exists := node.Labels["virtual-node.interlink/type"]; exists && nodeType == "virtual-kubelet" {
		return true
	}
	return false
}

// Helper function to check if a node is a control plane node
func isControlPlaneNode(node v1.Node) bool {
	// Check for control-plane role label (Kubernetes 1.20+)
	if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
		return true
	}

	// Check for master role label (older Kubernetes versions)
	if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
		return true
	}

	return false
}

func getNodesInfo(kubeClientset kubernetes.Interface, clusterInfo *types.StatusInfo) (map[string]*NodeInfoWithAllocatable, error) {
	nodes, err := kubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeInfoMap := make(map[string]*NodeInfoWithAllocatable)
	var totalGPUs int64 = 0

	for _, node := range nodes.Items {
		if isControlPlaneNode(node) {
			continue
		}

		nodeName := node.Name

		// Allocatable Resources
		cpu_alloc := node.Status.Allocatable.Cpu().MilliValue()
		memory_alloc := node.Status.Allocatable.Memory().Value()
		var gpu_alloc int64 = 0

		if gpuQty, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
			gpuVal, _ := gpuQty.AsInt64()
			gpu_alloc = gpuVal
			totalGPUs += gpuVal
		}

		// 2. Status
		nodeStatus := "Unknown"
		var conditions []types.NodeConditionSimple
		for _, cond := range node.Status.Conditions {
			isReady := (cond.Type == v1.NodeReady)

			// Map k8s status (True/False/Unknown) to bool (true/false)
			conditionStatus := cond.Status == v1.ConditionTrue

			conditions = append(conditions, types.NodeConditionSimple{
				Type:   string(cond.Type),
				Status: conditionStatus,
			})

			if isReady {
				if cond.Status == v1.ConditionTrue {
					nodeStatus = "Ready"
				} else {
					nodeStatus = "NotReady"
				}
			}
		}

		// Create NodeDetail and NodeInfoWithAllocatable
		nodeInfoMap[nodeName] = &NodeInfoWithAllocatable{
			NodeDetail: types.NodeDetail{
				Name: nodeName,
				CPU: types.NodeResource{
					CapacityCores: cpu_alloc, // Use CapacityCores
					UsageCores:    0,         // Will be updated in getMetricsInfo
				},
				Memory: types.NodeResource{
					CapacityBytes: memory_alloc, // Use CapacityBytes
					UsageBytes:    0,            // Will be updated in getMetricsInfo
				},
				GPU:         gpu_alloc,
				IsInterlink: checkIfInterLinkNode(node),
				Status:      nodeStatus,
				Conditions:  conditions,
			},
			CPUAllocatable:    cpu_alloc,
			MemoryAllocatable: memory_alloc,
			GPUAllocatable:    gpu_alloc,
		}
	}

	// Update Cluster GPU metrics
	clusterInfo.Cluster.Metrics.GPU.TotalGPU = totalGPUs

	return nodeInfoMap, nil
}

// getMetricsInfo

func getMetricsInfo(kubeClientset kubernetes.Interface, metricsClientset versioned.MetricsV1beta1Interface, nodeInfoMap map[string]*NodeInfoWithAllocatable, clusterInfo *types.StatusInfo) error {
	nodeMetricsList, err := metricsClientset.NodeMetricses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting metrics nodes: %v\n", err)
		return err
	}

	var cpu_free_total int64 = 0
	var cpu_max_free int64 = 0
	var memory_free_total int64 = 0
	var memory_max_free int64 = 0
	var number_nodes int64 = 0

	var nodeDetailList []types.NodeDetail
	for _, metrics := range nodeMetricsList.Items {
		nodeName := metrics.Name
		if nodeInfo, exists := nodeInfoMap[nodeName]; exists {
			number_nodes++

			// Get current usage

			cpu_quantity := metrics.Usage["cpu"]
			cpu_usage_milli := cpu_quantity.MilliValue()
			memory_quantity := metrics.Usage["memory"]
			memory_usage_bytes := memory_quantity.Value()

			// Allocatable capacity
			cpu_alloc := nodeInfo.CPUAllocatable
			memory_alloc := nodeInfo.MemoryAllocatable

			//Calculate free resources
			cpu_node_free := cpu_alloc - cpu_usage_milli
			memory_node_free := memory_alloc - memory_usage_bytes

			// Update NodeDetail with usage metrics (Use UsageCores and UsageBytes)
			nodeInfo.NodeDetail.CPU.UsageCores = cpu_usage_milli
			nodeInfo.NodeDetail.Memory.UsageBytes = memory_usage_bytes

			// Update Cluster totals
			cpu_free_total += cpu_node_free
			if cpu_max_free < cpu_node_free {
				cpu_max_free = cpu_node_free
			}

			memory_free_total += memory_node_free
			if memory_max_free < memory_node_free {
				memory_max_free = memory_node_free
			}

			// Add to the final list
			nodeDetailList = append(nodeDetailList, nodeInfo.NodeDetail)
		}
	}

	// Populate ClusterInfo (Use TotalFreeCores and MaxFreeOnNodeCores)
	clusterInfo.Cluster.NodesCount = number_nodes
	clusterInfo.Cluster.Nodes = nodeDetailList

	clusterInfo.Cluster.Metrics.CPU.TotalFreeCores = cpu_free_total
	clusterInfo.Cluster.Metrics.CPU.MaxFreeOnNodeCores = cpu_max_free
	clusterInfo.Cluster.Metrics.Memory.TotalFreeBytes = memory_free_total
	clusterInfo.Cluster.Metrics.Memory.MaxFreeOnNodeBytes = memory_max_free

	return nil
}

// getDeploymentInfo

func getDeploymentInfo(kubeClientset kubernetes.Interface, cfg *types.Config, clusterInfo *types.StatusInfo) (err error) {
	// Get OSCAR deployment status
	deploymentsClient := kubeClientset.AppsV1().Deployments(cfg.Namespace)
	deployment, err := deploymentsClient.Get(context.Background(), cfg.Namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deploymentReady := deployment.Status.ReadyReplicas == *deployment.Spec.Replicas

	/// Map to the new OscarDeployment structure
	deploymentInfo := types.OscarDeployment{
		Replicas:          *deployment.Spec.Replicas,
		ReadyReplicas:     deployment.Status.ReadyReplicas,
		AvailableReplicas: deployment.Status.AvailableReplicas,
		Strategy:          string(deployment.Spec.Strategy.Type),
		Labels:            deployment.Labels,
		CreationTimestamp: deployment.CreationTimestamp,
	}

	clusterInfo.Oscar = types.OscarInfo{
		DeploymentName: deployment.Name,
		Ready:          deploymentReady,
		Deployment:     deploymentInfo,
		OIDC: types.OIDCInfo{
			Enabled: cfg.OIDCEnable,
			Issuers: cfg.OIDCValidIssuers,
			Groups:  cfg.OIDCGroups,
		},
		// JobsCount and Pods are initialized to zero, they will be filled in getJobsInfo
		JobsCount: 0,
		Pods: types.PodStates{
			Total:  0,
			States: make(map[string]int),
		},
	}
	return nil
}

func getMinioInfo(cfg *types.Config, clusterInfo *types.StatusInfo) (err error) {
	// S3 client to list all buckets in the cluster
	s3Client := cfg.MinIOProvider.GetS3Client()
	bucketList, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return err
	}

	bucketCount := len(bucketList.Buckets)
	var totalObjectCount int = 0
	// Calculate the total object count
	for _, b := range bucketList.Buckets {
		bucketName := *b.Name
		input := &s3.ListObjectsInput{
			Bucket: aws.String(bucketName),
		}
		objectCh, _ := s3Client.ListObjects(input)
		for obj := range objectCh.Contents {
			// Only count objects that are not directories (directories have size 0 and usually end with a slash).
			if int(*objectCh.Contents[obj].Size) > 0 {
				totalObjectCount++
			}

		}
	}

	clusterInfo.MinIO = types.MinioInfo{
		BucketsCount: bucketCount,
		TotalObjects: totalObjectCount,
	}
	return nil
}

// getJobsInfo

func getJobsInfo(cfg *types.Config, kubeClientset kubernetes.Interface, clusterInfo *types.StatusInfo) (err error) {
	jobs, err := kubeClientset.BatchV1().Jobs(cfg.ServicesNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Sum all job statuses to get a simple total
	totalJobs := 0
	for _, job := range jobs.Items {
		totalJobs += int(job.Status.Active) + int(job.Status.Succeeded) + int(job.Status.Failed)
	}

	// Pods info (we keep the count by state)
	pods, err := kubeClientset.CoreV1().Pods(cfg.ServicesNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	//podStates := map[string]int{}
	podStates := map[string]int{
		"Pending":   0,
		"Running":   0,
		"Succeeded": 0,
		"Failed":    0,
		"Unknown":   0,
	}
	for _, pod := range pods.Items {
		state := string(pod.Status.Phase)
		podStates[state]++
	}

	podInfo := types.PodStates{
		Total:  len(pods.Items),
		States: podStates,
	}

	clusterInfo.Oscar.JobsCount = totalJobs
	clusterInfo.Oscar.Pods = podInfo
	return nil
}
