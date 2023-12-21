package integration_test

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func getBeskarReference(path string) string {
	return BeskarAddr + "/" + path
}

var _ = Describe("Beskar", func() {
	Describe("Test", Ordered, func() {
		localImage := "library/alpine:latest"
		remoteImage := "mirror.gcr.io/library/alpine:latest"

		It("Copy image", func() {
			platform := &v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
			}
			err := crane.Copy(remoteImage, getBeskarReference(localImage), crane.WithPlatform(platform))
			Expect(err).To(BeNil())
		})

		It("List Tag", func() {
			idx := strings.IndexByte(localImage, ':')
			Expect(idx).ToNot(Equal(-1))

			tags, err := crane.ListTags(getBeskarReference(localImage[:idx]))
			Expect(err).To(BeNil())
			Expect(tags).To(Equal([]string{"latest"}))
		})

		It("Delete image", func() {
			err := crane.Delete(getBeskarReference(localImage))
			Expect(err).To(BeNil())
		})
	})
})
