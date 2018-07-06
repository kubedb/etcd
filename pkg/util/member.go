package util

import "fmt"

type Member struct {
	Name      string
	service   string
	Namespace string
	// ID field can be 0, which is unknown ID.
	// We know the ID of a member when we get the member information from etcd,
	// but not from Kubernetes pod list.
	ID uint64

	SecurePeer   bool
	SecureClient bool
}

func (m *Member) Addr() string {
	return fmt.Sprintf("%s.%s.%s.svc.cluster.local", m.Name, m.service, m.Namespace)
}

// ClientURL is the client URL for this member
func (m *Member) ClientURL() string {
	return fmt.Sprintf("%s://%s:2379", m.clientScheme(), m.Addr())
}

func (m *Member) clientScheme() string {
	if m.SecureClient {
		return "https"
	}
	return "http"
}

func (m *Member) peerScheme() string {
	if m.SecurePeer {
		return "https"
	}
	return "http"
}

func (m *Member) ListenClientURL() string {
	return fmt.Sprintf("%s://0.0.0.0:2379", m.clientScheme())
}
func (m *Member) ListenPeerURL() string {
	return fmt.Sprintf("%s://0.0.0.0:2380", m.peerScheme())
}

func (m *Member) PeerURL() string {
	return fmt.Sprintf("%s://%s:2380", m.peerScheme(), m.Addr())
}

func GetEndpointsFromCluster(cluster string)  {

}