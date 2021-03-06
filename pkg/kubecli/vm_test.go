/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2017 Red Hat, Inc.
 *
 */

package kubecli

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubevirt.io/kubevirt/pkg/api/v1"
)

var _ = Describe("Kubevirt VM Client", func() {

	var upgrader websocket.Upgrader
	var server *ghttp.Server
	var client KubevirtClient
	basePath := "/apis/kubevirt.io/v1alpha1/namespaces/default/virtualmachines"
	vmPath := basePath + "/testvm"

	BeforeEach(func() {
		var err error
		server = ghttp.NewServer()
		client, err = GetKubevirtClientFromFlags(server.URL(), "")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should fetch a VM", func() {
		vm := v1.NewMinimalVM("testvm")
		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", vmPath),
			ghttp.RespondWithJSONEncoded(http.StatusOK, vm),
		))
		fetchedVM, err := client.VM(k8sv1.NamespaceDefault).Get("testvm", k8smetav1.GetOptions{})

		Expect(server.ReceivedRequests()).To(HaveLen(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(fetchedVM).To(Equal(vm))
	})

	It("should detect non existent VMs", func() {
		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", vmPath),
			ghttp.RespondWithJSONEncoded(http.StatusNotFound, errors.NewNotFound(schema.GroupResource{}, "testvm")),
		))
		_, err := client.VM(k8sv1.NamespaceDefault).Get("testvm", k8smetav1.GetOptions{})

		Expect(server.ReceivedRequests()).To(HaveLen(1))
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should fetch a VM list", func() {
		vm := v1.NewMinimalVM("testvm")
		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", basePath),
			ghttp.RespondWithJSONEncoded(http.StatusOK, NewVMList(*vm)),
		))
		fetchedVMList, err := client.VM(k8sv1.NamespaceDefault).List(k8smetav1.ListOptions{})

		Expect(server.ReceivedRequests()).To(HaveLen(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(fetchedVMList.Items).To(HaveLen(1))
		Expect(fetchedVMList.Items[0]).To(Equal(*vm))
	})

	It("should create a VM", func() {
		vm := v1.NewMinimalVM("testvm")
		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", basePath),
			ghttp.RespondWithJSONEncoded(http.StatusCreated, vm),
		))
		createdVM, err := client.VM(k8sv1.NamespaceDefault).Create(vm)

		Expect(server.ReceivedRequests()).To(HaveLen(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(createdVM).To(Equal(vm))
	})

	It("should update a VM", func() {
		vm := v1.NewMinimalVM("testvm")
		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("PUT", vmPath),
			ghttp.RespondWithJSONEncoded(http.StatusOK, vm),
		))
		updatedVM, err := client.VM(k8sv1.NamespaceDefault).Update(vm)

		Expect(server.ReceivedRequests()).To(HaveLen(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedVM).To(Equal(vm))
	})

	It("should delete a VM", func() {
		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("DELETE", vmPath),
			ghttp.RespondWithJSONEncoded(http.StatusOK, nil),
		))
		err := client.VM(k8sv1.NamespaceDefault).Delete("testvm", &k8smetav1.DeleteOptions{})

		Expect(server.ReceivedRequests()).To(HaveLen(1))
		Expect(err).ToNot(HaveOccurred())
	})

	It("should allow to connect a stream to a VM", func() {
		vncPath := "/apis/subresources.kubevirt.io/v1alpha1/namespaces/default/virtualmachines/testvm/vnc"

		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", vncPath),
			func(w http.ResponseWriter, r *http.Request) {
				_, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
			},
		))
		_, err := client.VM(k8sv1.NamespaceDefault).VNC("testvm")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should handle a failure connecting to the VM", func() {
		vncPath := "/apis/subresources.kubevirt.io/v1alpha1/namespaces/default/virtualmachines/testvm/vnc"

		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", vncPath),
			func(w http.ResponseWriter, r *http.Request) {
				return
			},
		))
		_, err := client.VM(k8sv1.NamespaceDefault).VNC("testvm")
		Expect(err).To(HaveOccurred())
	})

	It("should exchange data with the VM", func() {
		vncPath := "/apis/subresources.kubevirt.io/v1alpha1/namespaces/default/virtualmachines/testvm/vnc"

		server.AppendHandlers(ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", vncPath),
			func(w http.ResponseWriter, r *http.Request) {
				c, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					panic("server upgrader failed")
				}
				defer c.Close()

				for {
					mt, message, err := c.ReadMessage()
					if err != nil {
						io.WriteString(GinkgoWriter, fmt.Sprintf("server read failed: %v\n", err))
						break
					}

					err = c.WriteMessage(mt, message)
					if err != nil {
						io.WriteString(GinkgoWriter, fmt.Sprintf("server write failed: %v\n", err))
						break
					}
				}
			},
		))

		By("establishing connection")

		vnc, err := client.VM(k8sv1.NamespaceDefault).VNC("testvm")
		Expect(err).ToNot(HaveOccurred())

		By("wiring the pipes")

		pipeInReader, pipeInWriter := io.Pipe()
		pipeOutReader, pipeOutWriter := io.Pipe()

		go func() {
			vnc.Stream(StreamOptions{
				In:  pipeInReader,
				Out: pipeOutWriter,
			})
		}()

		By("sending data around")
		msg := "hello, vnc!"
		bufIn := make([]byte, 64)
		copy(bufIn[:], msg)

		_, err = pipeInWriter.Write(bufIn)
		Expect(err).ToNot(HaveOccurred())

		By("reading back data")
		bufOut := make([]byte, 64)
		_, err = pipeOutReader.Read(bufOut)
		Expect(err).ToNot(HaveOccurred())

		By("checking the result")
		Expect(bufOut).To(Equal(bufIn))
	})

	AfterEach(func() {
		server.Close()
	})
})
