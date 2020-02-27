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
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Agent is the main object of this library.
// You need to initialize an agent first, and then you need
// to configure your HTTP clients to use it as a RoundTripper.
type Agent struct {
	// Agent implements the http.RoundTripper interface
	http.RoundTripper

	// SecretKey is your Bearer Secret Key; available on https://app.bearer.sh/keys
	// Required
	SecretKey string

	// If set, the RoundTripper interface actually used to make requests
	// If nil, an equivalent of http.DefaultTransport is used
	Transport http.RoundTripper

	// If set, will be used for internal logging.
	Logger *zap.Logger

	// If set, this context will be used by the agent for managing its internal goroutines
	// and performing operational requests.
	Context context.Context

	// Duration between two config refreshes.
	// If empty, will use 5s as default.
	RefreshConfigEvery time.Duration

	// local vars
	configCache   *Config
	configMutex   sync.RWMutex
	configUpdates int
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

var (
	isParseableContentType = regexp.MustCompile(`(?i)json|text|xml|x-www-form-urlencoded`)
)

// RoundTrip implements the http.RoundTripper interface
func (a *Agent) RoundTrip(req *http.Request) (*http.Response, error) {
	for _, domain := range a.config().BlockedDomains {
		if domain == req.URL.Hostname() {
			return nil, ErrBlockedDomain
		}
	}

	var reqReader io.ReadCloser
	if req.Body != nil && a.isAvailable() {
		buf, err := ioutil.ReadAll(req.Body)
		if err != nil {
			a.logger().Error("read request body", zap.Error(err))
			return nil, err
		}
		reqReader = ioutil.NopCloser(bytes.NewBuffer(buf))
		req.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	}

	start := time.Now()
	resp, roundtripError := a.transport().RoundTrip(req)
	end := time.Now()

	if a.isAvailable() {
		record := newRecord(req, resp, start, end, reqReader, a.logger(), roundtripError)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					a.logger().Error("panic", zap.Any("r", r))
					// FIXME: log an internal error
				}
			}()
			if err := a.logRecords([]reportLog{record}); err != nil {
				a.logger().Warn("log record", zap.Error(err))
			}
		}()
	}

	// here we can handle retry/circuit-breaking policies, i.e.:
	/*
		        if resp.StatusCode == 429 {
				time.Sleep(time.Second)
				return a.RoundTrip(req)
			}
	*/
	return resp, roundtripError
}

func newRecord(req *http.Request, resp *http.Response, start, end time.Time, reqReader io.ReadCloser, logger *zap.Logger, roundtripError error) reportLog {
	record := reportLog{
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
	if roundtripError == nil && resp.Body != nil && isParseableContentType.MatchString(record.RequestContentType()) {
		buf, _ := ioutil.ReadAll(resp.Body)
		respReader := ioutil.NopCloser(bytes.NewBuffer(buf))
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
		respBody, _ := ioutil.ReadAll(respReader)
		record.ResponseBody = string(respBody)
	}
	if reqReader != nil && isParseableContentType.MatchString(record.ResponseContentType()) {
		reqBody, _ := ioutil.ReadAll(reqReader)
		record.RequestBody = string(reqBody)
	}
	if err := record.sanitize(); err != nil {
		logger.Warn("sanitize record", zap.Error(err))
	}
	return record
}

func (a *Agent) isAvailable() bool {
	return a.SecretKey != ""
}

// Config fetches and returns a fresh Bearer configuration for your current token
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

// Flush flushes any buffered log entries. Applications should take care to call Flush before exiting.
func (a Agent) Flush() error {
	// FIXME: this function is just a placeholder before we switch to a new async mechanism
	return nil
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
		a.configUpdates++
		a.configCache, err = a.Config()
		if err != nil {
			a.logger().Warn("fetch bearer config", zap.Error(err))
			return nil
		}

		// start a goroutine to refresh config regularly
		duration := a.RefreshConfigEvery
		if duration <= 0 {
			duration = 5 * time.Second
		}
		go func() {
			for {
				time.Sleep(duration)
				newConfig, err := a.Config()
				if err != nil {
					a.logger().Warn("fetch bearer config", zap.Error(err))
				} else {
					a.configMutex.Lock()
					a.configUpdates++
					a.configCache = newConfig
					a.configMutex.Unlock()
				}
			}
		}()
	}

	return a.configCache
}

func (a Agent) logRecords(records []reportLog) error {
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
		Logs []reportLog `json:"logs"`
	}
	input := logsRequest{SecretKey: a.SecretKey, Logs: records}
	input.Runtime.Type = "go"
	input.Runtime.Version = runtime.Version()
	input.Agent.Type = "bearer-go"
	input.Agent.Version = version
	input.Agent.LogLevel = "ALL"

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return err
	}
	reqBody := ioutil.NopCloser(strings.NewReader(string(inputJSON)))
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
