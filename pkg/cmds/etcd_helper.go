package cmds

import (
	eh "github.com/kubedb/etcd/pkg/etcd-helper"
	"github.com/spf13/cobra"
	"github.com/coreos/etcd/
)

func NewCmdEtcdHelper() *cobra.Command {

	cmd := &cobra.Command{
		Use:               "etcd-helper",
		Short:             "Run etcd helper",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			eh.RunEtcdHelper(dfd)
		},
	}
	cmd.Flags().AddGoFlagSet(etcdConf.Cf.FlagSet)

	return cmd
}
