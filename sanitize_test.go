package bearer

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitize(t *testing.T) {
	saneReport := reportLog{
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
	}

	var tests = []struct {
		input          reportLog
		expectedOutput reportLog
		expectedErr    error
	}{
		{saneReport, saneReport, nil},
		{reportLog{RequestHeaders: map[string]string{"authorization": "hello"}}, reportLog{RequestHeaders: map[string]string{"authorization": "[FILTERED]"}}, nil},
		{reportLog{RequestHeaders: map[string]string{"Authorization": "hello"}}, reportLog{RequestHeaders: map[string]string{"Authorization": "[FILTERED]"}}, nil},
		{reportLog{RequestHeaders: map[string]string{"AutHorizAtion": "hello"}}, reportLog{RequestHeaders: map[string]string{"AutHorizAtion": "[FILTERED]"}}, nil},
		{reportLog{RequestHeaders: map[string]string{"Authorization2": "hello"}}, reportLog{RequestHeaders: map[string]string{"Authorization2": "hello"}}, nil},
		{reportLog{RequestHeaders: map[string]string{"2Authorization": "hello"}}, reportLog{RequestHeaders: map[string]string{"2Authorization": "hello"}}, nil},
		{reportLog{RequestHeaders: map[string]string{"Blah": "hello"}}, reportLog{RequestHeaders: map[string]string{"Blah": "hello"}}, nil},
		{reportLog{RequestHeaders: map[string]string{"Blah": "contact@example.com"}}, reportLog{RequestHeaders: map[string]string{"Blah": "[FILTERED].com"}}, nil},
		{reportLog{RequestHeaders: map[string]string{"Blah": "aaa bbb@ccc ddd eee@fff.ggg hhh"}}, reportLog{RequestHeaders: map[string]string{"Blah": "aaa [FILTERED] ddd [FILTERED].ggg hhh"}}, nil},
		{reportLog{ResponseHeaders: map[string]string{"authorization": "hello"}}, reportLog{ResponseHeaders: map[string]string{"authorization": "[FILTERED]"}}, nil},
		{reportLog{ResponseHeaders: map[string]string{"Authorization": "hello"}}, reportLog{ResponseHeaders: map[string]string{"Authorization": "[FILTERED]"}}, nil},
		{reportLog{ResponseHeaders: map[string]string{"AutHorizAtion": "hello"}}, reportLog{ResponseHeaders: map[string]string{"AutHorizAtion": "[FILTERED]"}}, nil},
		{reportLog{ResponseHeaders: map[string]string{"Authorization2": "hello"}}, reportLog{ResponseHeaders: map[string]string{"Authorization2": "hello"}}, nil},
		{reportLog{ResponseHeaders: map[string]string{"2Authorization": "hello"}}, reportLog{ResponseHeaders: map[string]string{"2Authorization": "hello"}}, nil},
		{reportLog{ResponseHeaders: map[string]string{"Blah": "hello"}}, reportLog{ResponseHeaders: map[string]string{"Blah": "hello"}}, nil},
		{reportLog{ResponseHeaders: map[string]string{"Blah": "contact@example.com"}}, reportLog{ResponseHeaders: map[string]string{"Blah": "[FILTERED].com"}}, nil},
		{reportLog{ResponseHeaders: map[string]string{"Blah": "aaa bbb@ccc ddd eee@fff.ggg hhh"}}, reportLog{ResponseHeaders: map[string]string{"Blah": "aaa [FILTERED] ddd [FILTERED].ggg hhh"}}, nil},
		{reportLog{URL: "http://api.example.com/blah/blih?bluh=bloh&blouh=blanh"}, reportLog{URL: "http://api.example.com/blah/blih?bluh=bloh&blouh=blanh"}, nil},
		{reportLog{URL: "http://api.example.com/blah/blih?bluh=Authorization&authorization=blanh"}, reportLog{URL: ""}, nil},
		{reportLog{URL: "http://api.example.com/email/contact@example.org"}, reportLog{URL: "http://api.example.com/email/[FILTERED].org"}, nil},
		{reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{"authorization":"blah"}`}, reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{"authorization":"[FILTERED]"}`}, nil},
		{reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json; charset=utf-8"}, RequestBody: `{"authorization":"blah"}`}, reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json; charset=utf-8"}, RequestBody: `{"authorization":"[FILTERED]"}`}, nil},
		{reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `[42]`}, reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `[42]`}, nil},
		{reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `42`}, reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `42`}, nil},
		{reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{}`}, reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{}`}, nil},
		// FIXME: {reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{"a":{"authorization":"blah"}}`}, reportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{"a":{"authorization}:"[FILTERED]"}`}, nil},
	}
	i := 0
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			err := test.input.sanitize()
			require.NoError(t, err)
			checkSamereportLogs(t, test.expectedOutput, test.input)
		})
		i++
	}
}

func checkSamereportLogs(t *testing.T, a, b reportLog) {
	t.Helper()

	assert.Equal(t, a.Protocol, b.Protocol)
	assert.Equal(t, a.Path, b.Path)
	assert.Equal(t, a.Hostname, b.Hostname)
	assert.Equal(t, a.Method, b.Method)
	assert.Equal(t, a.StartedAt, b.StartedAt)
	assert.Equal(t, a.EndedAt, b.EndedAt)
	assert.Equal(t, a.Type, b.Type)
	assert.Equal(t, a.StatusCode, b.StatusCode)
	au, err := url.Parse(a.URL)
	if err != nil {
		assert.Equal(t, a.URL, b.URL)
	} else {
		bu, err := url.Parse(b.URL)
		if !assert.NoError(t, err) {
			assert.Equal(t, au, bu)
		}
	}
	assert.Equal(t, a.RequestHeaders, b.RequestHeaders)
	assert.Equal(t, a.RequestBody, b.RequestBody)
	assert.Equal(t, a.ResponseHeaders, b.ResponseHeaders)
	assert.Equal(t, a.ResponseBody, b.ResponseBody)
}
