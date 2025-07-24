package utils

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// GetEC2InstanceMetadata retrieves EC2 instance metadata and returns it as a map
func GetEC2InstanceMetadata() (map[string]string, error) {
	// Initialize the metadata map
	metadata := make(map[string]string)

	// Get IMDSv2 token
	token, err := getIMDSToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get IMDS token: %w", err)
	}

	// Get instance ID
	instanceID, err := getMetadata(token, "instance-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get instance ID: %w", err)
	}
	metadata["alpha.eksctl.io/instance-id"] = instanceID

	// Get instance lifecycle
	instanceLifecycle, err := getMetadata(token, "instance-life-cycle")
	if err != nil {
		// If instance lifecycle is not available, default to "on-demand"
		instanceLifecycle = "on-demand"
	}
	metadata["node-lifecycle"] = instanceLifecycle

	return metadata, nil
}

// getIMDSToken gets a token for IMDSv2
func getIMDSToken() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPut, "http://169.254.169.254/latest/api/token", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "600")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get token, status code: %d", resp.StatusCode)
	}

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// getMetadata gets specific metadata using the provided token
func getMetadata(token, path string) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/"+path, nil)
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
