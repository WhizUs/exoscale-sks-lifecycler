package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	egoscalev2 "github.com/exoscale/egoscale/v2"
	egoscalev2oapi "github.com/exoscale/egoscale/v2/oapi"

	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	nodeLabelNodepoolId string = "node.exoscale.net/nodepool-id"
)

func initKubeClient() (*kubernetes.Clientset, error) {
	var kubeconfigPath string

	if viper.GetString("kubeconfig") != "" {
		kubeconfigPath = viper.GetString("kubeconfig")
	}
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func initExoscaleClient() (*egoscalev2.Client, error) {	
	var exoscaleApiEndpoint string

	if viper.GetString("exoscale_api_endpoint") != "" {
		exoscaleApiEndpoint = viper.GetString("exoscale_api_endpoint")
	} else {
		exoscaleApiEndpoint = fmt.Sprintf("https://api-%s.exoscale.com/v2", viper.GetString("exoscale_api_zone"))
	}

	egoclient, err := egoscalev2.NewClient(viper.GetString("exoscale_api_key"), viper.GetString("exoscale_api_secret"), egoscalev2.ClientOptWithAPIEndpoint(
		exoscaleApiEndpoint,
	))
	if err != nil {
		return nil, err
	}

	return egoclient, nil
}

func cordonNode(clientset *kubernetes.Clientset, nodeName string, unschedulable bool) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, getErr := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}

		node.Spec.Unschedulable = unschedulable
		_, updateErr := clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
		return updateErr
	})
	if retryErr != nil {
		fmt.Printf("Update failed: %v", retryErr)
		return retryErr
	}
	fmt.Printf("Node %s cordoned\n", nodeName)

	return nil
}

func evictPod(clientset *kubernetes.Clientset, pod corev1.Pod) error {
	if err := clientset.CoreV1().Pods(pod.Namespace).Evict(context.Background(), &policyv1beta1.Eviction{
		ObjectMeta:    metav1.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name},
		DeleteOptions: &metav1.DeleteOptions{},
	}); err != nil && !errors.IsNotFound(err) {
		fmt.Printf("Error evicting pod: %v\n", err)
		return err
	}
	fmt.Printf("Pod %s/%s evicted\n", pod.Namespace, pod.Name)

	return nil
}

// restartDeployment restarts the deployment of a pod
func restartDeployment(clientset *kubernetes.Clientset, deployment appsv1.Deployment) error {
	// podOwnerRef := metav1.GetControllerOf(&pod)
	// if podOwnerRef == nil {
	// 	return fmt.Errorf("pod %s/%s has no owner", pod.Namespace, pod.Name)
	// }

	// replicaSet, err := clientset.AppsV1().ReplicaSets(pod.Namespace).Get(context.Background(), podOwnerRef.Name, metav1.GetOptions{})
	// if err != nil {
	// 	fmt.Printf("Error getting replicaSet %s/%s: %v\n", pod.Namespace, podOwnerRef.Name, err)
	// 	return err
	// }

	// replicaSetOwnerRef := metav1.GetControllerOf(replicaSet)
	// if replicaSetOwnerRef == nil {
	// 	fmt.Printf("ReplicaSet %s/%s has no owner\n", pod.Namespace, replicaSet.Name)
	// 	return err
	// }

	// deployment, err := clientset.AppsV1().Deployments(pod.Namespace).Get(context.Background(), replicaSetOwnerRef.Name, metav1.GetOptions{})
	// if err != nil {
	// 	fmt.Printf("Error getting deployment %s/%s: %v\n", pod.Namespace, replicaSetOwnerRef.Name, err)
	// 	return err
	// }

	// if deployment.Status.UnavailableReplicas == 0 {
	// } else {
	// 	fmt.Printf("Deployment is currently progressing, skipping rollout restart.")
	// 	return nil
	// }

	data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format("20060102150405"))
	result, err := clientset.AppsV1().Deployments(deployment.Namespace).Patch(context.Background(), deployment.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		fmt.Printf("Error patching deployment %s/%s, result: %s, %v\n", deployment.Namespace, deployment.Name, result, err)
		return err
	}

	// // Wait for pod to terminate
	// for {
	// 	_, err := clientset.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
	// 	if err != nil {
	// 		fmt.Printf("Pod %s/%s has terminated: %v\n", pod.Namespace, pod.Name, err)
	// 		break
	// 	} else {
	// 		fmt.Printf("Pod %s/%s is still running\n", pod.Namespace, pod.Name)
	// 		time.Sleep(15 * time.Second)
	// 	}
	// }

	return nil
}

// Get the nodepool ID of the selected node
func getNodepoolId(node corev1.Node) (string, error) {
	var sksNodepoolId string
	labelKey := nodeLabelNodepoolId
	labelValue, exists := node.Labels[labelKey]
	if exists {
		sksNodepoolId = labelValue
	} else {
		errMsg := fmt.Sprintf("Label '%s' does not exist on the node", labelKey)
		fmt.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}

	return sksNodepoolId, nil
}

// Get the nodepool of the selected node
func getNodepool(egoclient *egoscalev2.Client, ctx context.Context, sksClusterId string, sksNodepoolId string) (egoscalev2.SKSNodepool, error) {
	getSksNodepoolResponse, err := egoclient.GetSksNodepoolWithResponse(ctx, sksClusterId, sksNodepoolId)
	if err != nil {
		fmt.Printf("%s", err)
		return egoscalev2.SKSNodepool{}, err
	}

	var sksNodepool egoscalev2.SKSNodepool
	if err := json.Unmarshal([]byte(getSksNodepoolResponse.Body), &sksNodepool); err != nil {
		fmt.Printf("Error occurred during unmarshaling. Error: %s", err)
		return egoscalev2.SKSNodepool{}, err
	}

	return sksNodepool, nil
}

// Scale the nodepool of the selected node
func scaleNodepool(egoclient *egoscalev2.Client, ctx context.Context, _ corev1.Node, sksClusterId string, sksNodepoolId string) error {

	// Get nodepool of the selected node
	sksNodepool, err := getNodepool(egoclient, ctx, sksClusterId, sksNodepoolId)
	if err != nil {
		fmt.Printf("Error while trying to get nodepool: %s", err)
		return err
	}

	// Scale nodepool + 1
	sksNodepoolSizeNext := (*sksNodepool.Size + 1)
	_, err = egoclient.ScaleSksNodepoolWithResponse(ctx, sksClusterId, sksNodepoolId, egoscalev2oapi.ScaleSksNodepoolJSONRequestBody{
		Size: sksNodepoolSizeNext,
	})
	if err != nil {
		fmt.Printf("Error while trying to scale nodepool '%s': %s", sksNodepoolId, err)
		return err
	}

	return nil
}

func nodeHasRunningJobs(clientset *kubernetes.Clientset, nodeName string) (bool, error) {
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return false, err
	}

	for _, pod := range pods.Items {
		if pod.Labels["batch.kubernetes.io/job-name"] != "" && pod.Status.Phase == corev1.PodRunning {
			return true, nil
		}
	}

	return false, nil
}

// Wait until pods are healthy in the cluster
func waitPodsRunning(clientset *kubernetes.Clientset) error {
	fmt.Printf("Waiting for pods to be running.\n")
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Wait for all pods to be running
	for _, pod := range pods.Items {
		for {
			if PodRunningOrSucceeded(pod) {
				break
			}
			time.Sleep(5 * time.Second)
			podObj, err := clientset.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			pod = *podObj
		}
	}

	return nil
}

// Wait until all nodes are ready in the cluster
func waitNodesReady(clientset *kubernetes.Clientset) error {
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Wait for all nodes to be ready
	fmt.Printf("Waiting for nodes to be ready...\n")
	for _, node := range nodes.Items {
		for {
			if nodeReady(node) && kubeSystemPodsReady(clientset, node.Name){
				break
			}

			fmt.Printf("Node %s is not ready yet. Sleeping for 15 seconds.\n", node.Name)
			time.Sleep(15 * time.Second)
			nodeObj, err := clientset.CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			node = *nodeObj
		}
	}

	return nil
}

func nodeReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func kubeSystemPodsReady(clientset *kubernetes.Clientset, nodeName string) bool {
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return false
	}

	for _, pod := range pods.Items {
		if pod.Namespace == "kube-system" && (pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded) {
			return false
		}
	}

	return true
}

// Check if a pod is running
func PodRunningOrSucceeded(pod corev1.Pod) bool {
	return (pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodSucceeded)
}

func parseLabelSelector(labelSelector string) map[string]string {
	labels := make(map[string]string)
	
	// Split the input by commas to get individual key-value pairs
	pairs := strings.Split(labelSelector, ",")
	for _, pair := range pairs {
			// Split each pair by '=' to separate key and value
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
					key := strings.TrimSpace(kv[0])
					value := strings.TrimSpace(kv[1])
					labels[key] = value
			}
	}
	return labels
}
