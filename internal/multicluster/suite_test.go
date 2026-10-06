package multicluster

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
	testCfg   *rest.Config
	scheme    *k8sruntime.Scheme
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestMulticluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multicluster Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		return
	}

	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{}
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	testCfg = cfg

	scheme = k8sruntime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if cancel != nil {
		cancel()
	}
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})

func kubeconfigFromRest(cfg *rest.Config) []byte {
	// These are promoted from the embedded rest.TLSClientConfig.
	ca := cfg.CAData
	cert := cfg.CertData
	key := cfg.KeyData

	apiCfg := clientcmdapi.NewConfig()
	apiCfg.Clusters["envtest"] = &clientcmdapi.Cluster{
		Server:                   cfg.Host,
		CertificateAuthorityData: ca,
	}
	apiCfg.AuthInfos["envtest"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: cert,
		ClientKeyData:         key,
		Token:                 cfg.BearerToken,
	}
	apiCfg.Contexts["envtest"] = &clientcmdapi.Context{
		Cluster:  "envtest",
		AuthInfo: "envtest",
	}
	apiCfg.CurrentContext = "envtest"

	b, err := clientcmd.Write(*apiCfg)
	Expect(err).NotTo(HaveOccurred())
	return b
}
