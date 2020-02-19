package bearer

import "strings"

// Config is retrieved from Bearer's API.
type Config struct {
	BlockedDomains []string `json:"blockedDomains"`
	// FIXME: add missing fieldss
}

// ReportLog is the log object sent to Bearer's API.
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
	RequestHeaders  map[string]string `json:"requestHeaders"`
	RequestBody     string            `json:"requestBody"`
	ResponseHeaders map[string]string `json:"responseHeaders"`
	ResponseBody    string            `json:"responseBody"`
	// FIXME: Instrumentation
}

// RequestContentType returns the value of the requesting "Content-Type" HTTP header.
func (r ReportLog) RequestContentType() string {
	if r.RequestHeaders != nil {
		for k, v := range r.RequestHeaders {
			if strings.ToLower(k) == "content-type" {
				return v
			}
		}
	}
	return ""
}

// ResponseContentType returns the value of the replying "Content-Type" HTTP header.
func (r ReportLog) ResponseContentType() string {
	if r.ResponseHeaders != nil {
		for k, v := range r.ResponseHeaders {
			if strings.ToLower(k) == "content-type" {
				return v
			}
		}
	}
	return ""
}
