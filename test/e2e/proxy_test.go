//go:build e2e
// +build e2e

/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package e2e

import (
	"fmt"
	"net/url"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gorilla/websocket"
)

var _ = Describe("WebSocket Proxy", func() {

	Describe("Basic connectivity", func() {
		It("should proxy WebSocket data to TCP backend and back", func() {
			// Port-forward the proxy to localhost
			portForwardCmd := exec.Command("kubectl", "port-forward", "pod/ws-proxy-e2e", "18080:8080")
			err := portForwardCmd.Start()
			Expect(err).NotTo(HaveOccurred())
			defer portForwardCmd.Process.Kill()

			// Wait for port-forward to establish
			time.Sleep(2 * time.Second)

			By("connecting via WebSocket")
			u := url.URL{Scheme: "ws", Host: "127.0.0.1:18080", Path: "/"}
			ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			Expect(err).NotTo(HaveOccurred())
			defer ws.Close()

			By("sending test data")
			testData := []byte("hello from e2e test")
			err = ws.WriteMessage(websocket.BinaryMessage, testData)
			Expect(err).NotTo(HaveOccurred())

			By("reading echo response")
			_, response, err := ws.ReadMessage()
			Expect(err).NotTo(HaveOccurred())
			Expect(string(response)).To(Equal(string(testData)))
		})

		It("should reject connections when at capacity", func() {
			portForwardCmd := exec.Command("kubectl", "port-forward", "pod/ws-proxy-e2e", "18081:8080")
			err := portForwardCmd.Start()
			Expect(err).NotTo(HaveOccurred())
			defer portForwardCmd.Process.Kill()

			time.Sleep(2 * time.Second)

			u := url.URL{Scheme: "ws", Host: "127.0.0.1:18081", Path: "/"}

			By("filling up to max connections")
			var conns []*websocket.Conn
			for i := 0; i < 10; i++ {
				ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
				if err != nil {
					break
				}
				conns = append(conns, ws)
			}
			defer func() {
				for _, c := range conns {
					c.Close()
				}
			}()

			By("attempting one more connection (should be rejected)")
			_, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err == nil {
				Fail("expected connection to be rejected at capacity")
			}
			if resp != nil {
				Expect(resp.StatusCode).To(Equal(503))
			}
		})

		It("should report healthy on /health endpoint", func() {
			cmd := exec.Command("kubectl", "exec", "ws-proxy-e2e", "-c", "ws-proxy",
				"--", "/ws-proxy", "--healthcheck")
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("health check failed: %s", output))
		})
	})
})
