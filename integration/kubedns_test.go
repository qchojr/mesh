package integration

import (
	"time"

	"github.com/go-check/check"
	"github.com/sirupsen/logrus"
	"github.com/traefik/mesh/v2/integration/k3d"
	"github.com/traefik/mesh/v2/integration/tool"
	"github.com/traefik/mesh/v2/integration/try"
	checker "github.com/vdemeester/shakers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubeDNSSuite.
type KubeDNSSuite struct {
	logger  logrus.FieldLogger
	cluster *k3d.Cluster
	tool    *tool.Tool
}

func (s *KubeDNSSuite) SetUpSuite(c *check.C) {
	var err error

	requiredImages := []k3d.DockerImage{
		{Name: "traefik/mesh:latest", Local: true},
		{Name: "traefik/whoami:v1.6.0"},
		{Name: "giantswarm/tiny-tools:3.9"},
		{Name: "gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.7"},
		{Name: "gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.7"},
		{Name: "gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.7"},
	}

	s.logger = logrus.New()
	s.cluster, err = k3d.NewCluster(s.logger, masterURL, k3dClusterName,
		k3d.WithoutTraefik(),
		k3d.WithoutCoreDNS(),
		k3d.WithImages(requiredImages...),
	)
	c.Assert(err, checker.IsNil)

	c.Assert(s.cluster.CreateNamespace(s.logger, traefikMeshNamespace), checker.IsNil)
	c.Assert(s.cluster.CreateNamespace(s.logger, testNamespace), checker.IsNil)

	c.Assert(s.cluster.Apply(s.logger, smiCRDs), checker.IsNil)
	c.Assert(s.cluster.Apply(s.logger, "testdata/tool/tool.yaml"), checker.IsNil)
	c.Assert(s.cluster.Apply(s.logger, "testdata/dns/"), checker.IsNil)
	c.Assert(s.cluster.Apply(s.logger, "testdata/traefik-mesh/dns.yaml"), checker.IsNil)

	c.Assert(s.cluster.WaitReadyPod("tool", testNamespace, 60*time.Second), checker.IsNil)
	c.Assert(s.cluster.WaitReadyDeployment("kube-dns", metav1.NamespaceSystem, 60*time.Second), checker.IsNil)
	c.Assert(s.cluster.WaitReadyDeployment("traefik-mesh-dns", traefikMeshNamespace, 60*time.Second), checker.IsNil)

	s.tool = tool.New(s.logger, "tool", testNamespace)
}

func (s *KubeDNSSuite) TearDownSuite(c *check.C) {
	if s.cluster != nil {
		c.Assert(s.cluster.Stop(s.logger), checker.IsNil)
	}
}

func (s *KubeDNSSuite) TestKubeDNSDig(c *check.C) {
	s.logger.Info("Asserting KubeDNS has been patched successfully and can be dug")

	// Wait for kubeDNS, as the pods will be restarted by DNS patching.
	c.Assert(s.cluster.WaitReadyDeployment("kube-dns", metav1.NamespaceSystem, 60*time.Second), checker.IsNil)

	err := try.Retry(func() error {
		return s.tool.Dig("whoami.whoami.traefik.mesh")
	}, 60*time.Second)
	c.Assert(err, checker.IsNil)
}
