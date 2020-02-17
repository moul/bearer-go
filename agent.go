package bearer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Agent struct {
	Transport          http.RoundTripper
	SecretKey          string
	Logger             *zap.Logger
	Context            context.Context
	RefreshConfigEvery time.Duration

	// local vars
	configCache *Config
	configMutex sync.Mutex
}

// Init configures the default http.DefaultTransport with sane default values
func Init(secretKey string) *Agent {
	agent := &Agent{SecretKey: secretKey}
	return agent
}

// ReplaceGlobals replaces the global http.DefaultTransport, and returns
// a function to restore the original value.
func ReplaceGlobals(n http.RoundTripper) func() {
	prev := http.DefaultTransport
	http.DefaultTransport = n
	return func() { ReplaceGlobals(prev) }
}

// RoundTrip implements the http.RoundTripper interface
func (a *Agent) RoundTrip(req *http.Request) (*http.Response, error) {
	for _, domain := range a.config().BlockedDomains {
		if domain == req.URL.Hostname() {
			return nil, ErrBlockedDomain
		}
	}

	var reqReader io.ReadCloser
	if req.Body != nil {
		buf, _ := ioutil.ReadAll(req.Body)
		reqReader = ioutil.NopCloser(bytes.NewBuffer(buf))
		req.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	}

	start := time.Now()
	resp, err := a.transport().RoundTrip(req)
	end := time.Now()

	if a.SecretKey != "" {
		record := ReportLog{
			Protocol:        req.URL.Scheme,
			Path:            req.URL.Path,
			Hostname:        req.URL.Hostname(),
			Method:          req.Method,
			StartedAt:       int(start.UnixNano() / 1000000),
			EndedAt:         int(end.UnixNano() / 1000000),
			Type:            "REQUEST_END",
			StatusCode:      resp.StatusCode,
			URL:             req.URL.String(),
			RequestHeaders:  goHeadersToBearerHeaders(req.Header),
			ResponseHeaders: goHeadersToBearerHeaders(resp.Header),
		}
		if resp.Body != nil {
			buf, _ := ioutil.ReadAll(resp.Body)
			respReader := ioutil.NopCloser(bytes.NewBuffer(buf))
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
			respBody, _ := ioutil.ReadAll(respReader)
			record.ResponseBody = string(respBody)
		}
		if reqReader != nil {
			reqBody, _ := ioutil.ReadAll(reqReader)
			record.RequestBody = string(reqBody)
		}
		if err := a.logRecords([]ReportLog{record}); err != nil {
			a.logger().Warn("log records", zap.Error(err))
		}
	}

	// here we can handle retry/circuit-breaking policies, i.e.:
	/*
		        if resp.StatusCode == 429 {
				time.Sleep(time.Second)
				return a.RoundTrip(req)
			}
	*/
	return resp, err
}

func (a Agent) Config() (*Config, error) {
	req, err := http.NewRequest("GET", "https://config.bearer.sh/config", nil)
	if err != nil {
		return nil, fmt.Errorf("create config request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", a.SecretKey)

	ret, err := a.transport().RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer ret.Body.Close()

	// parse body
	body, err := ioutil.ReadAll(ret.Body)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a Agent) Flush() error {
	return nil
}

func goHeadersToBearerHeaders(input http.Header) map[string]string {
	ret := map[string]string{}
	for key, values := range input {
		// bearer headers only support one value per key
		// so we take the first one and ignore the other ones
		ret[key] = values[0]
	}
	return ret
}

func (a Agent) context() context.Context {
	if a.Context != nil {
		return a.Context
	}
	return context.Background()
}

func (a Agent) logger() *zap.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return zap.NewNop()
}

func (a Agent) transport() http.RoundTripper {
	if a.Transport != nil {
		return a.Transport
	}
	return defaultHTTPTransport
}

func (a *Agent) config() *Config {
	a.configMutex.Lock()
	defer a.configMutex.Unlock()
	if a.configCache == nil {
		var err error
		a.configCache, err = a.Config()
		if err != nil {
			a.logger().Warn("fetch bearer config", zap.Error(err))
			return nil
		}
	}

	return a.configCache
}

func (a Agent) logRecords(records []ReportLog) error {
	if len(records) < 1 {
		return nil
	}

	type logsRequest struct {
		SecretKey string `json:"secretKey"`
		Runtime   struct {
			Type    string `json:"type"`
			Version string `json:"version"`
		} `json:"runtime"`
		Agent struct {
			Type     string `json:"type"`
			Version  string `json:"version"`
			LogLevel string `json:"log_level"`
			// FIXME: Config
		} `json:"agent"`
		Logs []ReportLog `json:"logs"`
	}
	input := logsRequest{SecretKey: a.SecretKey, Logs: records}
	input.Runtime.Type = "go"
	input.Runtime.Version = runtime.Version()
	input.Agent.Type = "bearer-go"
	input.Agent.Version = Version
	input.Agent.LogLevel = "ALL"

	inputJson, err := json.Marshal(input)
	if err != nil {
		return err
	}
	reqBody := ioutil.NopCloser(strings.NewReader(string(inputJson)))
	req, err := http.NewRequest("POST", "https://agent.bearer.sh/logs", reqBody)
	if err != nil {
		return fmt.Errorf("create logs request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	ret, err := a.transport().RoundTrip(req)
	if err != nil {
		return fmt.Errorf("perform logs request: %w", err)
	}
	defer ret.Body.Close()
	switch ret.StatusCode {
	case 200:
		return nil
	default:
		/*
			body, err := ioutil.ReadAll(ret.Body)
			if err != nil {
				return fmt.Errorf("read logs body: %w", err)
			}
			var resp struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return fmt.Errorf("parse logs response: %w", err)
			}
		*/

		return fmt.Errorf("unsupported status code: %d", ret.StatusCode)
	}
}

// defaultHTTPTransport is the same as the stdlib http.DefaultTransport
// we use a dedicated one here to avoid having issues when overriding it
var defaultHTTPTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}
