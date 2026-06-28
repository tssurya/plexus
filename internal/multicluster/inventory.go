package multicluster

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LabelCluster identifies Secrets that represent spoke clusters in the
	// Plexus cluster inventory. Each Secret must contain a kubeconfig in
	// its data and can carry arbitrary labels for clusterSelector matching.
	LabelCluster = "plexus.io/cluster"

	// SecretDataKey is the key within the Secret's data map that holds the
	// kubeconfig YAML for the spoke cluster.
	SecretDataKey = "kubeconfig"

	// hubClusterName is the well-known name for the management cluster.
	hubClusterName = "hub"
)

// ClusterInfo describes a single cluster in the Plexus inventory.
type ClusterInfo struct {
	// Name is the unique identifier for this cluster (derived from the
	// Secret name for spoke clusters, or a well-known name for the hub).
	Name string

	// Labels carries the cluster metadata used for clusterSelector matching
	// (e.g. plexus.io/region, topology.kubernetes.io/zone).
	Labels map[string]string

	// Client is a controller-runtime client targeting this cluster.
	Client client.Client

	// IsHub is true for the management/hub cluster where the AND CR lives.
	IsHub bool
}

// ClusterInventory provides access to the fleet of clusters that Plexus can
// render resources to. In single-cluster mode, the inventory contains only
// the hub. In multi-cluster mode, it also includes spoke clusters discovered
// from kubeconfig Secrets.
type ClusterInventory interface {
	// MatchClusters returns the clusters whose labels satisfy the given
	// label selector. A nil or empty selector matches all clusters.
	MatchClusters(selector *metav1.LabelSelector) ([]ClusterInfo, error)

	// GetCluster returns a single cluster by name, or an error if the
	// cluster is not in the inventory.
	GetCluster(name string) (ClusterInfo, error)

	// AllClusters returns every cluster in the inventory.
	AllClusters() ([]ClusterInfo, error)
}
