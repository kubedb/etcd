package etcd_helper

import (
	"fmt"
	"log"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

func RunEtcdHelper() {

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}
	parts := strings.Split(hostname, "-")
	statefulSetName := strings.Join(parts[:len(parts)-1], "-")

	leader := fmt.Sprintf("%s-0", statefulSetName)
	config, err := restclient.InClusterConfig()
	if err != nil {
		log.Fatalln(err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln(err)
	}

	pod, err := kubeClient.CoreV1().Pods(namespace).Get(hostname, metav1.GetOptions{})
	if err != nil {
		log.Fatalln(err)
	}
	clusterState := "new"
	if hostname != leader {
		clusterState = "existing"
	}
	fmt.Println(clusterState, pod)
	fmt.Println(pod.Spec.Containers[0].Command)

}
