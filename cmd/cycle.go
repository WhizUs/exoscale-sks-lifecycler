/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// cycleCmd represents the cycle command
var cycleCmd = &cobra.Command{
	Use:   "cycle",
	Short: "Replace all nodes in a nodepool.",
	Long: `Replace all nodes in a nodepool. The procedure is as follows:
- Scale the nodepool up by one node (by default all nodes and nodepools are considered).
- Wait for the new node to be added to the nodepool.
- Cordon the node.
- Pods that are managed by daemonsets are skipped.
- Pods that are managed by deployments are rescheduled by restarting the deployment.
- Evict all remaining pods from the node.
- Evict the node from the nodepool.
- Wait for the pods to be running on other nodes.

The procedure is repeated for all nodes in the nodepool.
Nodes which have job pods running are cordoned, but the eviction is skipped.`,
	Run: func(cmd *cobra.Command, args []string) {
		desiredK8sVersion := viper.GetString("desired_k8s_version")
		exoscaleZone := viper.GetString("exoscale_api_zone")
		sksClusterId := viper.GetString("sks_cluster_id")
		evictNodesLabelSelector := viper.GetString("evict_nodes_labelselector")


		ctx := context.Background()

		clientset, err := initKubeClient()
		if err != nil {
			panic(err.Error())
		}

		egoclient, err := initExoscaleClient()
		if err != nil {
			panic(err.Error())
		}


		sksCluster, err := egoclient.GetSKSCluster(ctx, exoscaleZone, sksClusterId)
		if err != nil {
			panic(err.Error())
		}
	


		nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}

		evictNodesLabels := parseLabelSelector(evictNodesLabelSelector)

		// Iterate over all nodes
		for _, node := range nodes.Items {
			var hasDesiredVersion bool = false
			if node.Status.NodeInfo.KubeletVersion == desiredK8sVersion {
				fmt.Printf("Node %s is already on desired version %s\n", node.Name, node.Status.NodeInfo.KubeletVersion)
				hasDesiredVersion = true
			} else {				
				fmt.Printf("Node %s is not on desired version\n", node.Name)
			}
			
			var isSelectedForEvictionDueToLabels bool = false
			for key, value := range evictNodesLabels {
				if nodeValue, exists := node.Labels[key]; exists && nodeValue == value {
					fmt.Printf("Node %s is selected for eviction due to evictNodesLabelSelector config.\n", node.Name)
					isSelectedForEvictionDueToLabels = true
					break
				}
			}
	
			if !hasDesiredVersion || (hasDesiredVersion && isSelectedForEvictionDueToLabels) {
				// Proceed with the operation
			} else {
				continue
			}
		
			fmt.Printf("Node %s is currently on version %s\n", node.Name, node.Status.NodeInfo.KubeletVersion)

			sksNodepoolId, err := getNodepoolId(node)
			if err != nil {
				fmt.Printf("Error while trying to get nodepool ID: %s", err)
			}

			sksNodepool, err := getNodepool(egoclient, ctx, sksClusterId, sksNodepoolId)
			if err != nil {
				fmt.Printf("Error while trying to get nodepool: %s", err)
			}
			

			// nodepoolNodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			// 	LabelSelector: nodeLabelNodepoolId + "=" + sksNodepoolId,
			// })
			// if err != nil {
			// 	fmt.Printf("Error while trying to list nodes: %s", err)
			// }

			// if err := scaleNodepool(egoclient, ctx, node, sksClusterId, sksNodepoolId); err != nil {
			// 	fmt.Printf("Error while trying to scale nodepool: %s", err)
			// }

			// // Wait until the nodepool has been scaled
			// for {

			// 	// Check via the Kubernetes API if a new node has been added to the nodepool
			// 	nodepoolNodesNext, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			// 		LabelSelector: nodeLabelNodepoolId + "=" + sksNodepoolId,
			// 	})
			// 	if err != nil {
			// 		fmt.Printf("Error while trying to list nodes: %s", err)
			// 	}
			// 	if len(nodepoolNodesNext.Items) > len(nodepoolNodes.Items) {
			// 		fmt.Printf("Node has been added to the nodepool.\n")
			// 		break
			// 	}

			// 	fmt.Printf("Waiting for a new node to be added to the nodepool. Sleeping for 15 seconds.\n")
			// 	time.Sleep(15 * time.Second)
			// }

			if err := waitNodesReady(clientset); err != nil {
				fmt.Printf("Error while waiting for nodes to be ready: %s", err)
			}

			if err := cordonNode(clientset, node.Name, true); err != nil {
				fmt.Printf("Error while cordoning node: %s", err)
			}

			// If the node has running jobs, continue to the next node
			hasRunningJobs, err := nodeHasRunningJobs(clientset, node.Name)
			if err != nil {
				fmt.Printf("Error while checking if node has running jobs: %s", err)
			}
			if hasRunningJobs {
				fmt.Printf("Node %s has running jobs, skipping eviction and continuing to next node.\n", node.Name)
				continue
			}
		
			// Loop over all pods on the node until there are no more reschedulable pods left on the node.
			// Reschedulable pods are pods which are managed by a Deployment.
			for {
				var reschedulablePodsCount int = 0
				var podsTerminatingCount int = 0

				pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
					FieldSelector: "spec.nodeName=" + node.Name,
				})
				if err != nil {
					panic(err.Error())
				}
	
				for _, pod := range pods.Items {
					if pod.DeletionTimestamp != nil {
						podsTerminatingCount += 1
						fmt.Printf("Pod %s/%s is already terminating\n", pod.Namespace, pod.Name)
						continue
					}

					podOwnerRef := metav1.GetControllerOf(&pod)
					if podOwnerRef == nil {
						fmt.Printf("pod %s/%s has no owner", pod.Namespace, pod.Name)
					} else {
						if podOwnerRef.Kind == "DaemonSet" {
							fmt.Printf("Pod %s/%s is managed by a DaemonSet, skipping eviction\n", pod.Namespace, pod.Name)
							continue
						}

						if podOwnerRef.Kind == "ReplicaSet" {
							replicaSet, err := clientset.AppsV1().ReplicaSets(pod.Namespace).Get(context.Background(), podOwnerRef.Name, metav1.GetOptions{})
							if err != nil {
								fmt.Printf("Error getting replicaSet %s/%s: %v\n", pod.Namespace, podOwnerRef.Name, err)
							}
						
							replicaSetOwnerRef := metav1.GetControllerOf(replicaSet)
							if replicaSetOwnerRef.Kind == "Deployment" {
								reschedulablePodsCount += 1

								deployment, err := clientset.AppsV1().Deployments(pod.Namespace).Get(context.Background(), replicaSetOwnerRef.Name, metav1.GetOptions{})
								if err != nil {
									fmt.Printf("Error getting deployment %s/%s: %v\n", pod.Namespace, replicaSetOwnerRef.Name, err)
								}

								if deployment.Status.UnavailableReplicas == 0 {
									if err := restartDeployment(clientset, *deployment); err != nil {
										fmt.Printf("Error while restarting deployment: %s", err)
									}	
								} else {
									fmt.Printf("Deployment %s/%s is currently progressing, skipping rollout restart.\n", deployment.Namespace, deployment.Name)
								}

								continue
							}
						}
					}

					if err := evictPod(clientset, pod); err != nil {
						fmt.Printf("Error while evicting pod: %s", err)
					}
				}

				if reschedulablePodsCount == 0 && podsTerminatingCount == 0 {
					break
				} else {
					fmt.Println("Not all reschedulable pods have been rescheduled, sleeping for 15 seconds.")
					time.Sleep(time.Second * 15)
				}
			}

			// if err := waitPodsRunning(clientset); err != nil {
			// 	fmt.Printf("Error while waiting for pods to be running: %s", err)
			// }

			if err := egoclient.EvictSKSNodepoolMembers(ctx, exoscaleZone, sksCluster, &sksNodepool, []string{node.Status.NodeInfo.SystemUUID}); err != nil {
				fmt.Printf("Error while evicting node from nodepool: %s", err)
			}
			fmt.Printf("Node %s evicted from nodepool %s\n", node.Name, sksNodepoolId)

			// if err := waitPodsRunning(clientset); err != nil {
			// 	fmt.Printf("Error while waiting for pods to be running: %s", err)
			// }
		}



	},
}

func init() {
	nodepoolCmd.AddCommand(cycleCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// cycleCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// cycleCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
