package bearer

import (
	"net/http"

	"go.uber.org/zap"
)

type Agent struct {
	Transport http.RoundTripper
	SecretKey string
	Logger    *zap.Logger

	// local vars
	configCache *Config
}

type Config struct {
	BlockedDomains []string `json:"blockedDomains"`
	// FIXME: add missing fieldss
}

type ReportLog struct {
	Protocol        string            `json:"protocol"`
	Path            string            `json:"path"`
	Hostname        string            `json:"hostname"`
	Method          string            `json:"method"`
	StartedAt       int               `json:"startedAt"`
	EndedAt         int               `json:"endedAt"`
	Type            string            `json:"type"`
	StatusCode      int               `json:"statusCode"`
	URL             string            `json:"url"`
	RequestHeaders  map[string]string `json:"requestHeaders"` // FIXME: map[string][]string?
	RequestBody     string            `json:"requestBody"`
	ResponseHeaders map[string]string `json:"responseHeaders"` // FIXME: map[string][]string?
	ResponseBody    string            `json:"responseBody"`
	// FIXME: Instrumentation
}
