package helpers

import (
	"encoding/json"
	"fmt"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var _ = DescribeTable("versionCheck reconciler",
	func(version string, specifiedHttpPort int32, expectedHttpPort int32, expectedSecurityConfigPort int32, expectedSecurityConfigPath string) {
		instance := &opsterv1.OpenSearchCluster{
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					Version:  version,
					HttpPort: specifiedHttpPort,
				},
			},
		}

		actualHttpPort, actualSecurityConfigPort, actualConfigPath := VersionCheck(instance)

		Expect(actualHttpPort).To(Equal(expectedHttpPort))
		Expect(actualSecurityConfigPort).To(Equal(expectedSecurityConfigPort))
		Expect(actualConfigPath).To(Equal(expectedSecurityConfigPath))
	},
	Entry("When no http port is specified and version 1.3.0 is used", "1.3.0", int32(0), int32(9200), int32(9300), "/usr/share/opensearch/plugins/opensearch-security/securityconfig"),
	Entry("When no http port is specified and version 2.0 is used", "2.0", int32(0), int32(9200), int32(9200), "/usr/share/opensearch/config/opensearch-security"),
	Entry("When an http port is specified and version 1.3.0 is used", "1.3.0", int32(6000), int32(6000), int32(9300), "/usr/share/opensearch/plugins/opensearch-security/securityconfig"),
	Entry("When an http port is specified and version 2.0 is used", "2.0", int32(6000), int32(6000), int32(6000), "/usr/share/opensearch/config/opensearch-security"),
)

var _ = Describe("bvdsdhsr", func() {
	It("dtsrtrastras", func() {
		a := &apiextensionsv1.JSON{Raw: []byte{123, 34, 105, 110, 100, 101, 120, 46, 110, 117, 109, 98, 101, 114, 95, 111, 102, 95, 114, 101, 112, 108, 105, 99, 97, 115, 34, 58, 34, 49, 34, 44, 34, 105, 110, 100, 101, 120, 46, 110, 117, 109, 98, 101, 114, 95, 111, 102, 95, 115, 104, 97, 114, 100, 115, 34, 58, 34, 49, 34, 44, 34, 105, 110, 100, 101, 120, 46, 114, 101, 102, 114, 101, 115, 104, 95, 105, 110, 116, 101, 114, 118, 97, 108, 34, 58, 34, 51, 48, 115, 34, 125}}
		b := &apiextensionsv1.JSON{Raw: []byte{123, 34, 105, 110, 100, 101, 120, 34, 58, 123, 34, 110, 117, 109, 98, 101, 114, 95, 111, 102, 95, 114, 101, 112, 108, 105, 99, 97, 115, 34, 58, 34, 49, 34, 44, 34, 110, 117, 109, 98, 101, 114, 95, 111, 102, 95, 115, 104, 97, 114, 100, 115, 34, 58, 34, 49, 34, 44, 34, 114, 101, 102, 114, 101, 115, 104, 95, 105, 110, 116, 101, 114, 118, 97, 108, 34, 58, 34, 51, 48, 115, 34, 125, 125}}

		var map1, map2 map[string]interface{}
		json.Unmarshal(a.Raw, &map1)
		json.Unmarshal(b.Raw, &map2)

		fmt.Println(map1)
		fmt.Println(map2)

		Fail("Not implemented")
	})
})
