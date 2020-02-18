package bearer

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Example() {
	ReplaceGlobals(Init(os.Getenv("BEARER_SECRETKEY")))

	// perform request
	resp, err := http.Get("...")
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp)
}

func Example_custom() {
	logger, _ := zap.NewDevelopment()
	agent := &Agent{
		SecretKey: os.Getenv("BEARER_SECRETKEY"),
		Logger:    logger,
		Transport: http.DefaultTransport,
	}
	defer agent.Flush()
	client := &http.Client{Transport: agent}

	// perform request
	resp, err := client.Get("...")
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp)
}

func TestAgent_Config(t *testing.T) {
	sk := os.Getenv("BEARER_SECRETKEY")
	if sk == "" {
		t.Skip()
	}
	agent := Agent{SecretKey: sk}
	config, err := agent.Config()
	require.NoError(t, err)
	assert.NotNil(t, config)
}

func TestAgent_logRecords(t *testing.T) {
	records := []ReportLog{
		{
			Protocol:        "https",
			Path:            "/sample",
			Hostname:        "api.example.com",
			Method:          "GET",
			StartedAt:       int(time.Now().Add(-80*time.Millisecond).UnixNano() / 1000000),
			EndedAt:         int(time.Now().UnixNano() / 1000000),
			Type:            "REQUEST_END",
			StatusCode:      200,
			URL:             "http://api.example.com/sample",
			RequestHeaders:  map[string]string{"Accept": "application/json"},
			RequestBody:     `{"body":"data"}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
			ResponseBody:    `{"ok":true}`,
			// instrumentation: ,
		},
	}
	t.Run("unauthenticated", func(t *testing.T) {
		agent := Agent{}
		for i := 0; i < 10; i++ {
			err := agent.logRecords(records)
			require.Error(t, err)
		}
	})

	t.Run("unauthenticated/concurrent", func(t *testing.T) {
		agent := Agent{}
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				err := agent.logRecords(records)
				require.Error(t, err)
				wg.Done()
			}()
			wg.Wait()
		}
	})

	sk := os.Getenv("BEARER_SECRETKEY")
	if sk == "" {
		t.Skip()
	}
	t.Run("authenticated", func(t *testing.T) {
		agent := Agent{SecretKey: sk}
		for i := 0; i < 3; i++ {
			err := agent.logRecords(records)
			require.NoError(t, err)
		}
	})
}

func TestRoundTrip(t *testing.T) {
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Hello", "World")
		w.Write([]byte("200 OK"))
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	t.Run("unauthenticated", func(t *testing.T) {
		client := &http.Client{
			Transport: &Agent{},
		}

		resp, err := client.Get(ts.URL)
		require.NoError(t, err)
		assert.Equal(t, resp.StatusCode, 200)
	})

	t.Run("blocked-domain", func(t *testing.T) {
		client := &http.Client{
			Transport: &Agent{
				configCache: &Config{
					BlockedDomains: []string{"localhost", "127.0.0.1"},
				},
			},
		}
		resp, err := client.Get(ts.URL)
		assert.True(t, errors.Is(err, ErrBlockedDomain))
		assert.Nil(t, resp)
	})

	sk := os.Getenv("BEARER_TOKEN")
	if sk == "" {
		t.Skip()
	}
	t.Run("authenticated", func(t *testing.T) {
		client := &http.Client{
			Transport: &Agent{SecretKey: sk},
		}
		resp, err := client.Get(ts.URL + "/test")
		require.NoError(t, err)
		assert.Equal(t, resp.StatusCode, 200)
	})
}
