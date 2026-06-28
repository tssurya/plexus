package multicluster

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretInventory discovers spoke clusters from kubeconfig Secrets
// labeled plexus.io/cluster=true. It also includes the hub cluster.
// The remaining Secret labels form the cluster metadata used for
// clusterSelector matching.
//
// The inventory is refreshed by calling Sync(), which should be invoked
// from the AND reconciler whenever a cluster Secret changes.
type SecretInventory struct {
	hubClient client.Client
	hubLabels map[string]string
	scheme    *runtime.Scheme
	namespace string
	log       logr.Logger

	mu       sync.RWMutex
	clusters map[string]ClusterInfo
}

// SecretInventoryOptions configures the SecretInventory.
type SecretInventoryOptions struct {
	// HubClient is the controller-runtime client for the hub cluster.
	HubClient client.Client

	// HubLabels are labels assigned to the hub cluster for selector matching.
	HubLabels map[string]string

	// Scheme is used when constructing clients for spoke clusters.
	Scheme *runtime.Scheme

	// Namespace restricts Secret discovery to a single namespace.
	// Empty string means all namespaces.
	Namespace string

	// Log is the logger.
	Log logr.Logger
}

func NewSecretInventory(opts SecretInventoryOptions) *SecretInventory {
	return &SecretInventory{
		hubClient: opts.HubClient,
		hubLabels: opts.HubLabels,
		scheme:    opts.Scheme,
		namespace: opts.Namespace,
		log:       opts.Log.WithName("cluster-inventory"),
		clusters:  make(map[string]ClusterInfo),
	}
}

// Sync reads all Secrets labeled plexus.io/cluster=true and rebuilds
// the inventory. Existing clients for unchanged clusters are preserved.
// Call this from the reconciler whenever cluster Secrets change.
func (s *SecretInventory) Sync(ctx context.Context) error {
	var secrets corev1.SecretList
	listOpts := []client.ListOption{
		client.MatchingLabels{LabelCluster: "true"},
	}
	if s.namespace != "" {
		listOpts = append(listOpts, client.InNamespace(s.namespace))
	}
	if err := s.hubClient.List(ctx, &secrets, listOpts...); err != nil {
		return fmt.Errorf("listing cluster secrets: %w", err)
	}

	newClusters := make(map[string]ClusterInfo, len(secrets.Items)+1)

	newClusters[hubClusterName] = ClusterInfo{
		Name:   hubClusterName,
		Labels: s.hubLabels,
		Client: s.hubClient,
		IsHub:  true,
	}

	s.mu.RLock()
	existing := s.clusters
	s.mu.RUnlock()

	for i := range secrets.Items {
		secret := &secrets.Items[i]
		name := secret.Name

		if name == hubClusterName {
			s.log.Info("skipping Secret with reserved name", "name", name)
			continue
		}

		clusterLabels := clusterLabelsFromSecret(secret)

		if prev, ok := existing[name]; ok && prev.Client != nil {
			prev.Labels = clusterLabels
			newClusters[name] = prev
			continue
		}

		spokeClient, err := s.clientFromSecret(secret)
		if err != nil {
			s.log.Error(err, "failed to create client for spoke cluster", "secret", name)
			continue
		}

		s.log.Info("discovered spoke cluster", "name", name, "labels", clusterLabels)
		newClusters[name] = ClusterInfo{
			Name:   name,
			Labels: clusterLabels,
			Client: spokeClient,
			IsHub:  false,
		}
	}

	s.mu.Lock()
	s.clusters = newClusters
	s.mu.Unlock()

	return nil
}

func (s *SecretInventory) MatchClusters(selector *metav1.LabelSelector) ([]ClusterInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if selector == nil {
		return s.allClustersLocked(), nil
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("parsing label selector: %w", err)
	}

	var matched []ClusterInfo
	for _, ci := range s.clusters {
		if sel.Matches(labels.Set(ci.Labels)) {
			matched = append(matched, ci)
		}
	}
	return matched, nil
}

func (s *SecretInventory) GetCluster(name string) (ClusterInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ci, ok := s.clusters[name]
	if !ok {
		return ClusterInfo{}, fmt.Errorf("cluster %q not found in inventory", name)
	}
	return ci, nil
}

func (s *SecretInventory) AllClusters() ([]ClusterInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allClustersLocked(), nil
}

func (s *SecretInventory) allClustersLocked() []ClusterInfo {
	all := make([]ClusterInfo, 0, len(s.clusters))
	for _, ci := range s.clusters {
		all = append(all, ci)
	}
	return all
}

func (s *SecretInventory) clientFromSecret(secret *corev1.Secret) (client.Client, error) {
	kubeconfig, ok := secret.Data[SecretDataKey]
	if !ok {
		return nil, fmt.Errorf("secret %q missing %q key", secret.Name, SecretDataKey)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parsing kubeconfig from secret %q: %w", secret.Name, err)
	}

	rest.AddUserAgent(restConfig, "plexus-controller")

	c, err := client.New(restConfig, client.Options{Scheme: s.scheme})
	if err != nil {
		return nil, fmt.Errorf("creating client for cluster %q: %w", secret.Name, err)
	}
	return c, nil
}

// clusterLabelsFromSecret extracts cluster metadata labels from the Secret,
// filtering out the inventory label itself.
func clusterLabelsFromSecret(secret *corev1.Secret) map[string]string {
	result := make(map[string]string, len(secret.Labels))
	for k, v := range secret.Labels {
		if k == LabelCluster {
			continue
		}
		result[k] = v
	}
	return result
}
