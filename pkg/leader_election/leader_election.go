package leader_election

import (
	"fmt"
	"log"
	"os"
	//"os/user"
	//"strconv"
	"strings"
	"time"

	core_util "github.com/appscode/kutil/core/v1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

const (
	RoleNew      = "new"
	RoleExisting = "existing"
)

func RunLeaderElection() {

	leaderElectionLease := 3 * time.Second

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	// Change owner of Postgres data directory
	if err := setPermission(); err != nil {
		log.Fatalln(err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	parts := strings.Split(hostname, "-")
	statefulSetName := strings.Join(parts[:len(parts)-1], "-")

	fmt.Println(fmt.Sprintf(`We want "%v" as our leader`, hostname))

	config, err := restclient.InClusterConfig()
	if err != nil {
		log.Fatalln(err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln(err)
	}

	configMap := &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetLeaderLockName(statefulSetName),
			Namespace: namespace,
		},
	}
	if _, err := kubeClient.CoreV1().ConfigMaps(namespace).Create(configMap); err != nil && !kerr.IsAlreadyExists(err) {
		log.Fatalln(err)
	}

	resLock := &resourcelock.ConfigMapLock{
		ConfigMapMeta: configMap.ObjectMeta,
		Client:        kubeClient.CoreV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      hostname,
			EventRecorder: &record.FakeRecorder{},
		},
	}

	//runningFirstTime := true

	go func() {
		leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
			Lock:          resLock,
			LeaseDuration: leaderElectionLease,
			RenewDeadline: leaderElectionLease * 2 / 3,
			RetryPeriod:   leaderElectionLease / 3,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(stop <-chan struct{}) {
					fmt.Println("Got leadership, now do your jobs")
				},
				OnStoppedLeading: func() {
					fmt.Println("Lost leadership, now quit")
					os.Exit(1)
				},
				OnNewLeader: func(identity string) {
					fmt.Println("New leader.................")
					statefulSet, err := kubeClient.AppsV1().StatefulSets(namespace).Get(statefulSetName, metav1.GetOptions{})
					fmt.Println("identity.......................", err)
					if err != nil {
						log.Fatalln(err)
					}

					pods, err := kubeClient.CoreV1().Pods(namespace).List(metav1.ListOptions{
						LabelSelector: metav1.FormatLabelSelector(statefulSet.Spec.Selector),
					})
					if err != nil {
						log.Fatalln(err)
					}

					initialCluster := initialClusterUrls(pods.Items, statefulSetName)
					dataDir := "/var/lib/etcd"
					commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s "+
						"--initial-cluster=%s",
						dataDir, strings.Join(initialCluster, ","))

					for _, pod := range pods.Items {
						role := RoleExisting
						if pod.Name == identity {
							role = RoleNew
						}
						commands += fmt.Sprintf(" --name=%s --initial-advertise-peer-urls=%s "+
							"--listen-peer-urls=%s --listen-client-urls=%s --advertise-client-urls=%s --initial-cluster-state=%s",
							pod.Name,
							fmt.Sprintf("http://%s:2380", podAddress(pod, statefulSetName)),
							fmt.Sprintf("http://0.0.0.0:2380"),
							fmt.Sprintf("http://0.0.0.0:2379"),
							fmt.Sprintf("http://%s:2379", podAddress(pod, statefulSetName)),
							role)
						_, _, err = core_util.PatchPod(kubeClient, &pod, func(in *core.Pod) *core.Pod {
							in.Labels["kubedb.com/role"] = role
							in.Spec.Containers[0].Command = strings.Split(commands, " ")
							return in
						})
					}
				},
			},
		})
	}()

	select {}
}

func setPermission() error {
	/*u, err := user.Lookup("etcd")
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	err = os.Chown("/var/pv", uid, gid)
	if err != nil {
		return err
	}*/
	return nil
}

func GetLeaderLockName(offshootName string) string {
	return fmt.Sprintf("%s-leader-lock", offshootName)
}

func initialClusterUrls(items []core.Pod, offshootName string) []string {
	ps := make([]string, 0)
	for _, m := range items {
		ps = append(ps, fmt.Sprintf("%s=%s", m.Name, fmt.Sprintf("http://%s:2380", podAddress(m, offshootName))))
	}
	return ps
}

func podAddress(pod core.Pod, cluster string) string {
	return fmt.Sprintf("%s.%s.%s.svc", pod.Name, cluster, pod.Namespace)
}
