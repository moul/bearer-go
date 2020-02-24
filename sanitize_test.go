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
	saneReport := ReportLog{
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
		input          ReportLog
		expectedOutput ReportLog
		expectedErr    error
	}{
		{saneReport, saneReport, nil},
		{ReportLog{RequestHeaders: map[string]string{"authorization": "hello"}}, ReportLog{RequestHeaders: map[string]string{"authorization": "[FILTERED]"}}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Authorization": "hello"}}, ReportLog{RequestHeaders: map[string]string{"Authorization": "[FILTERED]"}}, nil},
		{ReportLog{RequestHeaders: map[string]string{"AutHorizAtion": "hello"}}, ReportLog{RequestHeaders: map[string]string{"AutHorizAtion": "[FILTERED]"}}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Authorization2": "hello"}}, ReportLog{RequestHeaders: map[string]string{"Authorization2": "hello"}}, nil},
		{ReportLog{RequestHeaders: map[string]string{"2Authorization": "hello"}}, ReportLog{RequestHeaders: map[string]string{"2Authorization": "hello"}}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Blah": "hello"}}, ReportLog{RequestHeaders: map[string]string{"Blah": "hello"}}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Blah": "contact@example.com"}}, ReportLog{RequestHeaders: map[string]string{"Blah": "[FILTERED].com"}}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Blah": "aaa bbb@ccc ddd eee@fff.ggg hhh"}}, ReportLog{RequestHeaders: map[string]string{"Blah": "aaa [FILTERED] ddd [FILTERED].ggg hhh"}}, nil},
		{ReportLog{ResponseHeaders: map[string]string{"authorization": "hello"}}, ReportLog{ResponseHeaders: map[string]string{"authorization": "[FILTERED]"}}, nil},
		{ReportLog{ResponseHeaders: map[string]string{"Authorization": "hello"}}, ReportLog{ResponseHeaders: map[string]string{"Authorization": "[FILTERED]"}}, nil},
		{ReportLog{ResponseHeaders: map[string]string{"AutHorizAtion": "hello"}}, ReportLog{ResponseHeaders: map[string]string{"AutHorizAtion": "[FILTERED]"}}, nil},
		{ReportLog{ResponseHeaders: map[string]string{"Authorization2": "hello"}}, ReportLog{ResponseHeaders: map[string]string{"Authorization2": "hello"}}, nil},
		{ReportLog{ResponseHeaders: map[string]string{"2Authorization": "hello"}}, ReportLog{ResponseHeaders: map[string]string{"2Authorization": "hello"}}, nil},
		{ReportLog{ResponseHeaders: map[string]string{"Blah": "hello"}}, ReportLog{ResponseHeaders: map[string]string{"Blah": "hello"}}, nil},
		{ReportLog{ResponseHeaders: map[string]string{"Blah": "contact@example.com"}}, ReportLog{ResponseHeaders: map[string]string{"Blah": "[FILTERED].com"}}, nil},
		{ReportLog{ResponseHeaders: map[string]string{"Blah": "aaa bbb@ccc ddd eee@fff.ggg hhh"}}, ReportLog{ResponseHeaders: map[string]string{"Blah": "aaa [FILTERED] ddd [FILTERED].ggg hhh"}}, nil},
		{ReportLog{URL: "http://api.example.com/blah/blih?bluh=bloh&blouh=blanh"}, ReportLog{URL: "http://api.example.com/blah/blih?bluh=bloh&blouh=blanh"}, nil},
		{ReportLog{URL: "http://api.example.com/blah/blih?bluh=Authorization&authorization=blanh"}, ReportLog{URL: ""}, nil},
		{ReportLog{URL: "http://api.example.com/email/contact@example.org"}, ReportLog{URL: "http://api.example.com/email/[FILTERED].org"}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{"authorization":"blah"}`}, ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{"authorization":"[FILTERED]"}`}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `[42]`}, ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `[42]`}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `42`}, ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `42`}, nil},
		{ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{}`}, ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{}`}, nil},
		// FIXME: {ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{"a":{"authorization":"blah"}}`}, ReportLog{RequestHeaders: map[string]string{"Content-Type": "application/json"}, RequestBody: `{"a":{"authorization}:"[FILTERED]"}`}, nil},
	}
	i := 0
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			err := test.input.sanitize()
			require.NoError(t, err)
			checkSameReportLogs(t, test.expectedOutput, test.input)
		})
		i++
	}
}

func checkSameReportLogs(t *testing.T, a, b ReportLog) {
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
