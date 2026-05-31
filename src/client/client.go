// client.go - HTTP client for cassonic REST API
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client wraps http.Client with cassonic-specific auth and helpers.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	debug      bool
}

// newClient creates a configured Client.
func newClient(baseURL, token string, debug bool) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		debug: debug,
	}
}

// do executes an HTTP request with optional JSON body and returns the response.
// On 401 with TOKEN_REVOKED or TOKEN_EXPIRED, it prints an error and exits 1.
func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cassonic-cli/"+Version)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.debug {
		fmt.Fprintf(os.Stderr, "→ %s %s\n", method, url)
		if body != nil {
			data, _ := json.MarshalIndent(body, "  ", "  ")
			fmt.Fprintf(os.Stderr, "  body: %s\n", data)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if c.debug {
		fmt.Fprintf(os.Stderr, "← %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(bodyBytes)
		if strings.Contains(bodyStr, "TOKEN_REVOKED") || strings.Contains(bodyStr, "TOKEN_EXPIRED") {
			fmt.Fprintln(os.Stderr, colorize(ansiRed, "error: your API token has been revoked. Run 'cassonic-cli login' to re-authenticate."))
			deleteToken()
			os.Exit(1)
		}
		// Restore body for caller to inspect.
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	return resp, nil
}

// get performs a GET request and unmarshals the JSON response into out.
func (c *Client) get(path string, out any) error {
	resp, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.decodeResponse(resp, out)
}

// post performs a POST request with a JSON body and unmarshals the response.
func (c *Client) post(path string, body, out any) error {
	resp, err := c.do(http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.decodeResponse(resp, out)
}

// delete performs a DELETE request.
func (c *Client) delete(path string) error {
	resp, err := c.do(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// put performs a PUT request with a JSON body and unmarshals the response.
func (c *Client) put(path string, body, out any) error {
	resp, err := c.do(http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.decodeResponse(resp, out)
}

// decodeResponse checks the status code and unmarshals JSON into out.
func (c *Client) decodeResponse(resp *http.Response, out any) error {
	if resp.StatusCode >= 400 {
		var errBody map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
			if detail, ok := errBody["detail"].(string); ok && detail != "" {
				return fmt.Errorf("server error %d: %s", resp.StatusCode, detail)
			}
			if msg, ok := errBody["message"].(string); ok && msg != "" {
				return fmt.Errorf("server error %d: %s", resp.StatusCode, msg)
			}
		}
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// getRaw performs a GET and returns the raw response body bytes.
func (c *Client) getRaw(path string) ([]byte, int, error) {
	resp, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}

// postRaw performs a POST and returns raw response bytes.
func (c *Client) postRaw(path string, body any) ([]byte, int, error) {
	resp, err := c.do(http.MethodPost, path, body)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}
