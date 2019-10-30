/*
Copyright The KubeDB Authors.

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
package etcd_helper

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coreos/etcd/pkg/types"
)

func TestUrl(t *testing.T) {
	cluster := "mach0=http://1.1.1.1:2380,mach0=http://2.2.2.2:2380,mach1=http://3.3.3.3:2380,mach2=http://4.4.4.4:2380"
	urls, err := types.NewURLsMap(cluster)
	fmt.Println(urls.URLs(), err)
	for _, v := range urls {
		//	fmt.Println(u, "<>", v[0])
		hostport := strings.Split(v[0].Host, ":")
		url := fmt.Sprintf("%s://%s:2379", v[0].Scheme, hostport[0])
		fmt.Println(url)
	}
}
