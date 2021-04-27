package traefik_plugin_headers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHeaderTransformationAndRequest(t *testing.T) {
	testCases := []struct {
		desc     string
		headers  http.Header
		rules    []Rule
		expected http.Header
	}{
		{
			desc: "set",
			rules: []Rule{
				{
					Name:           "01 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "set", Value: "foo"}},
				},
			},
			expected: http.Header{"Header-1": []string{"foo"}},
		},
		{
			desc:    "set overide",
			headers: http.Header{"Header-1": []string{"foo"}},
			rules: []Rule{
				{
					Name:           "02 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "set", Value: "bar"}},
				},
			},
			expected: http.Header{"Header-1": []string{"bar"}},
		},
		{
			desc:    "unset",
			headers: http.Header{"Header-1": []string{"foo"}},
			rules: []Rule{
				{
					Name:           "03 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "unset"}},
				},
			},
			expected: http.Header{},
		},
		{
			desc:    "unset not exist",
			headers: http.Header{},
			rules: []Rule{
				{
					Name:           "04 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "unset"}},
				},
			},
			expected: http.Header{},
		},
		{
			desc:    "append",
			headers: http.Header{"Header-1": []string{"foo"}},
			rules: []Rule{
				{
					Name:           "05 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "append", Value: "bar"}},
				},
			},
			expected: http.Header{"Header-1": []string{"foo", "bar"}},
		},
		{
			desc:    "append not exist",
			headers: http.Header{},
			rules: []Rule{
				{
					Name:           "06 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "append", Value: "bar"}},
				},
			},
			expected: http.Header{"Header-1": []string{"bar"}},
		},
		{
			desc:    "edit not existing",
			headers: http.Header{},
			rules: []Rule{
				{
					Name:           "07 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "edit", Value: "bar"}},
				},
			},
			expected: http.Header{"Header-1": []string{"bar"}},
		},
		{
			desc:    "edit wrong regexp",
			headers: http.Header{"Header-1": []string{"foo=123"}},
			rules: []Rule{
				{
					Name:           "08 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "edit", Replace: "foo=[azeazeazea]+", Value: "bar=456"}},
				},
			},
			expected: http.Header{"Header-1": []string{"foo=123", "bar=456"}},
		},
		{
			desc:    "edit good regexp",
			headers: http.Header{"Header-1": []string{"foo=123"}},
			rules: []Rule{
				{
					Name:           "09 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "edit", Replace: "foo=[0-9]+", Value: "foo=456"}},
				},
			},
			expected: http.Header{"Header-1": []string{"foo=456"}},
		},
		{
			desc:    "edit DT_ADD regexp",
			headers: http.Header{"Header-1": []string{"foo=123"}},
			rules: []Rule{
				{
					Name:           "10 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "edit", Replace: "foo=[^,]+", Value: "foo=@DT_ADD#86400@"}},
				},
			},
			expected: http.Header{"Header-1": []string{fmt.Sprintf("foo=%s", time.Now().Add(time.Second*time.Duration(86400)).Format(http.TimeFormat))}},
		},
		{
			desc:    "edit multiple values not existing",
			headers: http.Header{},
			rules: []Rule{
				{
					Name:           "11 REQ rule",
					Regexp:         "NO_MATCH",
					RequestHeaders: map[string]Header{"Header-1": {Action: "edit", Replace: "foo=[^,]+", Value: "bar=123, foo=@DT_ADD#86400@"}},
				},
			},
			expected: http.Header{"Header-1": []string{"bar=123", fmt.Sprintf("foo=%s", time.Now().Add(time.Second*time.Duration(86400)).Format(http.TimeFormat))}},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			ctx := context.Background()
			cfg := &Config{
				Rules: test.rules,
			}
			next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

			handler, err := New(ctx, next, cfg, "traefik_plugin_headers_test")
			if err != nil {
				t.Fatal(err)
			}

			recorder := httptest.NewRecorder()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
			if err != nil {
				t.Fatal(err)
			}

			for k, v := range test.headers {
				req.Header.Set(k, strings.Join(v, ","))
			}

			handler.ServeHTTP(recorder, req)

			if !reflect.DeepEqual(req.Header, test.expected) {
				t.Errorf("maps not equals on test %s\nactual:   %v\nexpected: %v", test.desc, req.Header, test.expected)
			}
		})
	}
}

func TestRegexpRulesAndResponse(t *testing.T) {
	// Test Response headers, regexp and priority/overload
	rules := []Rule{
		{
			Name:   "01 Low Cache by Extension",
			Regexp: "(jpe?g|gif|png|js)$",
			ResponseHeaders: map[string]Header{
				"Cache-Control": {Action: "set", Value: "public, max-age=86400"},
				"Expires":       {Action: "set", Value: "@DT_ADD#86400@"},
				"Cache-Test":    {Action: "set", Value: "LOW CACHE"},
			},
		},
		{
			Name:   "02 No Cache by URL",
			Regexp: `(nocache|no-cache)`,
			ResponseHeaders: map[string]Header{
				"Cache-Control": {Action: "set", Value: "public, max-age=86400"},
				"Expires":       {Action: "set", Value: "@DT_ADD#86400@"},
				"Cache-Test":    {Action: "set", Value: "NO CACHE"},
			},
		},
		{
			Name:   "03 Long Cache by URL",
			Regexp: `\.cache\.`,
			ResponseHeaders: map[string]Header{
				"Cache-Control": {Action: "set", Value: "public, max-age=86400"},
				"Expires":       {Action: "set", Value: "@DT_ADD#86400@"},
				"Cache-Test":    {Action: "set", Value: "LONG CACHE"},
			},
		},
		{
			Name:   "04 Only if no match - default",
			Regexp: "NO_MATCH",
			ResponseHeaders: map[string]Header{
				"Expires":       {Action: "set", Value: "0"},
				"Cache-Control": {Action: "set", Value: "no-cache, no-store, max-age=0, must-revalidate"},
				"Pragma":        {Action: "set", Value: "no-cache"},
				"Cache-Test":    {Action: "set", Value: "NO CACHE - DEFAULT"},
			},
		},
		{
			Name:   "05 after no match",
			Regexp: `(after-no-match)`,
			ResponseHeaders: map[string]Header{
				"Expires":     {Action: "set", Value: "100"},
				"Cache-Test2": {Action: "set", Value: "AFTER"},
			},
		},
		{
			Name:   "06 Never used",
			Regexp: "NO_MATCH",
			ResponseHeaders: map[string]Header{
				"Expires":       {Action: "set", Value: "0"},
				"Cache-Control": {Action: "set", Value: "no-cache, no-store, max-age=0, must-revalidate"},
				"Pragma":        {Action: "set", Value: "no-cache"},
				"Cache-Test":    {Action: "set", Value: "ERROR"},
			},
		},
	}

	testCases := []struct {
		desc               string
		url                string
		headers            http.Header
		rules              []Rule
		respHeaderExpected http.Header
	}{
		{ // Test if Rule 01 is applied (.gif)
			desc:  "http://test01.uri.com/test.gif",
			url:   "http://test01.uri.com/test.gif",
			rules: rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LOW CACHE"},
			},
		},
		{ // Test if Rule 04 (NO_MATCH) is applied and not Rule 06 (NO_MATCH)
			desc:  "http://test02.uri.com/test.html",
			url:   "http://test02.uri.com/test.html",
			rules: rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"no-cache", "no-store", "max-age=0", "must-revalidate"},
				"Expires":       []string{"0"},
				"Pragma":        []string{"no-cache"},
				"Cache-Test":    []string{"NO CACHE - DEFAULT"},
			},
		},
		{ // Test if Rule 03 (.cache.) overload  Rule 01 (.js)
			desc:  "http://test03.uri.com/override.cache.test.js",
			url:   "http://test03.uri.com/override.cache.test.js",
			rules: rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LONG CACHE"},
			},
		},
		{ // Test if Rule 05 (after-no-match) overload  Rule 04 (NO_MATCH)
			desc:  "http://test04.uri.com/uri-after-no-match-test.html",
			url:   "http://test04.uri.com/uri-after-no-match-test.html",
			rules: rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"no-cache", "no-store", "max-age=0", "must-revalidate"},
				"Expires":       []string{"100"},
				"Pragma":        []string{"no-cache"},
				"Cache-Test":    []string{"NO CACHE - DEFAULT"},
				"Cache-Test2":   []string{"AFTER"},
			},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			ctx := context.Background()
			cfg := &Config{
				Rules: test.rules,
			}
			next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(http.StatusOK)
			})

			handler, err := New(ctx, next, cfg, "traefik_plugin_headers_test")
			if err != nil {
				t.Fatal(err)
			}

			recorder := httptest.NewRecorder()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, test.url, nil)
			if err != nil {
				t.Fatal(err)
			}

			for k, v := range test.headers {
				req.Header.Set(k, strings.Join(v, ","))
			}

			handler.ServeHTTP(recorder, req)

			if !reflect.DeepEqual(recorder.Header(), test.respHeaderExpected) {
				t.Errorf("maps not equals\n actual:   %v\n expected: %v", recorder.Header(), test.respHeaderExpected)
			}
		})
	}
}
