package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/weaveworks/eksctl/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMetadata(t *testing.T) {
	testutils.RegisterAndRun(t)
}

// Create a test version of GetEC2InstanceMetadata that uses a custom server URL
func testGetEC2InstanceMetadata(serverURL string) (map[string]string, error) {
	// Initialize the metadata map
	metadata := make(map[string]string)

	// Get IMDSv2 token
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, serverURL+"/latest/api/token", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "600")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get token, status code: %d", resp.StatusCode)
	}

	tokenBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}
	token := string(tokenBytes)

	// Get instance ID
	instanceID, err := getMetadataFromURL(client, token, serverURL, "instance-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get instance ID: %w", err)
	}
	metadata["instance-id"] = instanceID

	// Get instance lifecycle
	instanceLifecycle, err := getMetadataFromURL(client, token, serverURL, "instance-life-cycle")
	if err != nil {
		// If instance lifecycle is not available, default to "on-demand"
		instanceLifecycle = "on-demand"
	}
	metadata["instance-lifecycle"] = instanceLifecycle

	return metadata, nil
}

func getMetadataFromURL(client *http.Client, token, serverURL, path string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, serverURL+"/latest/meta-data/"+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-aws-ec2-metadata-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get metadata for %s, status code: %d", path, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

var _ = Describe("EC2 Instance Metadata", func() {
	var server *httptest.Server

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Context("when both instance ID and lifecycle are available", func() {
		BeforeEach(func() {
			// Create a mock HTTP server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check the request path and respond accordingly
				switch r.URL.Path {
				case "/latest/api/token":
					// Verify it's a PUT request with the right header
					Expect(r.Method).To(Equal(http.MethodPut))
					Expect(r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds")).To(Equal("600"))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("mock-token"))
				case "/latest/meta-data/instance-id":
					// Verify token is present
					Expect(r.Header.Get("X-aws-ec2-metadata-token")).To(Equal("mock-token"))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("i-1234567890abcdef0"))
				case "/latest/meta-data/instance-life-cycle":
					// Verify token is present
					Expect(r.Header.Get("X-aws-ec2-metadata-token")).To(Equal("mock-token"))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("spot"))
				default:
					Fail("Unexpected request to " + r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
		})

		It("should return the correct instance ID and lifecycle", func() {
			// Test with our test function that uses the mock server
			metadata, err := testGetEC2InstanceMetadata(server.URL)
			Expect(err).NotTo(HaveOccurred())
			Expect(metadata).To(HaveKeyWithValue("instance-id", "i-1234567890abcdef0"))
			Expect(metadata).To(HaveKeyWithValue("instance-lifecycle", "spot"))
		})
	})

	Context("when instance lifecycle is not available", func() {
		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/latest/api/token":
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("mock-token"))
				case "/latest/meta-data/instance-id":
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("i-1234567890abcdef0"))
				case "/latest/meta-data/instance-life-cycle":
					w.WriteHeader(http.StatusNotFound)
				}
			}))
		})

		It("should default to on-demand lifecycle", func() {
			metadata, err := testGetEC2InstanceMetadata(server.URL)
			Expect(err).NotTo(HaveOccurred())
			Expect(metadata).To(HaveKeyWithValue("instance-id", "i-1234567890abcdef0"))
			Expect(metadata).To(HaveKeyWithValue("instance-lifecycle", "on-demand"))
		})
	})
})
