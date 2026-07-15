//go:build e2e
// +build e2e

/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	proxyImage  = envOrDefault("E2E_PROXY_IMAGE", "jupyter-k8s-ws-proxy:test")
	kindCluster = envOrDefault("KIND_CLUSTER", "ws-proxy-e2e")
)

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	fmt.Fprintf(GinkgoWriter, "Starting workspace-websocket-proxy E2E test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	// Ensure we're targeting the E2E Kind cluster
	kindContext := fmt.Sprintf("kind-%s", kindCluster)
	By(fmt.Sprintf("switching kubectl context to %s", kindContext))
	cmd := exec.Command("kubectl", "config", "use-context", kindContext)
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(),
		fmt.Sprintf("Kind context %s not found. Run: make setup-test-e2e\nOutput: %s", kindContext, output))

	// Deploy test resources (proxy pod + echo TCP server)
	By("deploying test pod with ws-proxy sidecar and echo server")
	cmd = exec.Command("kubectl", "apply", "-f", "testdata/test-pod.yaml")
	output, err = cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to deploy test pod: %s", output))

	By("waiting for test pod to be ready")
	cmd = exec.Command("kubectl", "wait", "pod/ws-proxy-e2e",
		"--for=condition=Ready", "--timeout=60s")
	output, err = cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Test pod not ready: %s", output))
})

var _ = AfterSuite(func() {
	By("cleaning up test resources")
	cmd := exec.Command("kubectl", "delete", "-f", "testdata/test-pod.yaml", "--ignore-not-found")
	_, _ = cmd.CombinedOutput()
})

// run executes a command and returns combined output.
func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	return string(output), err
}
