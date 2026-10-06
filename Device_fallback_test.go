package onvif

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// countingTransport counts the requests that go through the caller's http.Client.
type countingTransport struct {
	requests atomic.Int32
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests.Add(1)
	return http.DefaultTransport.RoundTrip(req)
}

func newDeviceAt(server *httptest.Server, client *http.Client) (*Device, error) {
	return NewDevice(DeviceParams{
		Xaddr:      strings.TrimPrefix(server.URL, "http://"),
		Username:   "user",
		Password:   "pass",
		HttpClient: client,
	})
}

func TestCallMethod_DoesNotRetryWhenTheCameraDoesNotAnswer(t *testing.T) {
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()

	start := time.Now()
	_, err := newDeviceAt(server, &http.Client{Timeout: 50 * time.Millisecond})

	assert.Error(t, err)
	assert.Equal(t, int32(1), received.Load(), "a camera that did not answer must not get a digest retry")
	assert.Less(t, time.Since(start), 250*time.Millisecond, "the caller's timeout must bound the call")
}

func TestCallMethod_RetriesWithDigestThroughTheCallersClient(t *testing.T) {
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	transport := &countingTransport{}

	_, err := newDeviceAt(server, &http.Client{Transport: transport})

	assert.Error(t, err)
	assert.Equal(t, int32(2), received.Load(), "a camera that rejects WS-Security still gets the digest retry")
	assert.Equal(t, received.Load(), transport.requests.Load(), "the digest retry must use the caller's client")
}
