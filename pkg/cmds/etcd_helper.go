package cmds

import (
	"log"

	"github.com/spf13/cobra"
	eh "kubedb.dev/etcd/pkg/etcd-helper"
	"kubedb.dev/etcd/pkg/etcdmain"
)

func NewCmdEtcdHelper() *cobra.Command {

	etcdConf := etcdmain.NewConfig()
	cmd := &cobra.Command{
		Use:               "etcd-helper",
		Short:             "Run etcd helper",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			if err := etcdConf.ConfigFromCmdLine(); err != nil {
				log.Fatalln(err)
			}
			eh.RunEtcdHelper(etcdConf)
		},
	}
	cmd.Flags().AddGoFlagSet(etcdConf.FlagSet)

	return cmd
}
