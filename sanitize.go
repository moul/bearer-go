package bearer

import (
	"encoding/json"
	"net/url"
	"regexp"
)

const (
	defaultStripSensitiveKeys   = `(?i)^authorization$|^password$|^secret$|^passwd$|^api.?key$|^access.?token$|^auth.?token$|^credentials$|^mysql_pwd$|^stripetoken$|^card.?number.?$|^secret$|^client.?id$|^client.?secret$`
	defaultStripSensitiveRegex  = `[a-zA-Z0-9]{1}[a-zA-Z0-9.!#$%&’*+=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9-]+(?:\\.[a-zA-Z0-9-]+)*|(?:\\d[ -]*?){13,16}`
	defaultSensitivePlaceholder = `[FILTERED]`
)

var (
	sensitiveKeys   = regexp.MustCompile(defaultStripSensitiveKeys)
	sensitiveValues = regexp.MustCompile(defaultStripSensitiveRegex)
	// FIXME: remove globals
)

// sanitize prevents most of the credentials from being sent to Bearer
func (r *ReportLog) sanitize() error {
	// sanitize headers
	if r.RequestHeaders != nil {
		for k, v := range r.RequestHeaders {
			if sensitiveKeys.MatchString(k) {
				r.RequestHeaders[k] = defaultSensitivePlaceholder
			} else {
				r.RequestHeaders[k] = sensitiveValues.ReplaceAllString(v, defaultSensitivePlaceholder)
			}
		}
	}
	if r.ResponseHeaders != nil {
		for k, v := range r.ResponseHeaders {
			if sensitiveKeys.MatchString(k) {
				r.ResponseHeaders[k] = defaultSensitivePlaceholder
			} else {
				r.ResponseHeaders[k] = sensitiveValues.ReplaceAllString(v, defaultSensitivePlaceholder)
			}
		}
	}

	// sanitize URL & query
	if r.URL != "" {
		r.URL = sensitiveValues.ReplaceAllString(r.URL, defaultSensitivePlaceholder)
		r.Path = sensitiveValues.ReplaceAllString(r.Path, defaultSensitivePlaceholder)
		u, err := url.Parse(r.URL)
		if err != nil {
			return err
		}
		changed := false
		queries := u.Query()
		for k, values := range queries {
			if sensitiveKeys.MatchString(k) {
				for idx := range values {
					values[idx] = defaultSensitivePlaceholder
				}
				changed = true
			}
		}
		if changed {
			u.RawQuery = queries.Encode()
			r.URL = u.String()
		}
	}

	// sanitize bodies
	if r.RequestBody != "" && r.RequestContentType() == "application/json" {
		body, err := sanitizeJSON(r.RequestBody)
		if err != nil {
			return err
		}
		r.RequestBody = body
	}
	if r.ResponseBody != "" && r.ResponseContentType() == "application/json" {
		body, err := sanitizeJSON(r.ResponseBody)
		if err != nil {
			return err
		}
		r.ResponseBody = body
	}

	return nil
}

func sanitizeJSON(input string) (string, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		// json cannot unmarshal to the map[string]interface{} destination
		// we cannot check for key/values
		return input, nil
	}

	for k, v := range obj {
		if sensitiveKeys.MatchString(k) {
			obj[k] = defaultSensitivePlaceholder
		} else {
			switch t := v.(type) {
			case string:
				obj[k] = sensitiveValues.ReplaceAllString(t, defaultSensitivePlaceholder)
				// FIXME: support nested maps
			}
		}
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return input, err
	}
	return string(out), nil
}
