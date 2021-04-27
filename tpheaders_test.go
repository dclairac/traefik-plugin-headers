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

func TestDefaultRules(t *testing.T) {
	testCases := []struct {
		desc           string
		headers        http.Header
		defaultHeaders map[string]Header
		expected       http.Header
	}{
		{
			desc: "default set req",
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action: "set",
					Value:  "foo",
				},
			},
			expected: http.Header{
				"Header-1": []string{"foo"},
			},
		},
		{
			desc: "default set override",
			headers: http.Header{
				"Header-1": []string{"foo"},
			},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action: "set",
					Value:  "bar",
				},
			},
			expected: http.Header{
				"Header-1": []string{"bar"},
			},
		},
		{
			desc: "default unset req",
			headers: http.Header{
				"Header-1": []string{"foo"},
			},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action: "unset",
					Value:  "bar",
				},
			},
			expected: http.Header{},
		},
		{
			desc:    "default unset req not exist",
			headers: http.Header{},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action: "unset",
					Value:  "bar",
				},
			},
			expected: http.Header{},
		},
		{
			desc: "default append",
			headers: http.Header{
				"Header-1": []string{"foo"},
			},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action: "append",
					Value:  "bar",
				},
			},
			expected: http.Header{
				"Header-1": []string{"foo", "bar"},
			},
		},
		{
			desc:    "default append not exist",
			headers: http.Header{},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action: "append",
					Value:  "bar",
				},
			},
			expected: http.Header{
				"Header-1": []string{"bar"},
			},
		},
		{
			desc:    "default edit req not existing",
			headers: http.Header{},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action: "edit",
					Value:  "bar",
				},
			},
			expected: http.Header{
				"Header-1": []string{"bar"},
			},
		},
		{
			desc: "default edit req wrong regexp",
			headers: http.Header{
				"Header-1": []string{"foo=123"},
			},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action:  "edit",
					Replace: "foo=[azeazeazea]+",
					Value:   "bar=456",
				},
			},
			expected: http.Header{
				"Header-1": []string{"foo=123", "bar=456"},
			},
		},
		{
			desc: "default edit req wrong regexp",
			headers: http.Header{
				"Header-1": []string{"foo=123"},
			},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action:  "edit",
					Replace: "foo=[0-9]+",
					Value:   "bar=456",
				},
			},
			expected: http.Header{
				"Header-1": []string{"bar=456"},
			},
		},
		{
			desc: "default edit req DT_ADD regexp",
			headers: http.Header{
				"Header-1": []string{"foo=123"},
			},
			defaultHeaders: map[string]Header{
				"Header-1": {
					Action:  "edit",
					Replace: "foo=[^,]+",
					Value:   "foo=@DT_ADD#86400@",
				},
			},
			expected: http.Header{
				"Header-1": []string{fmt.Sprintf("foo=%s", time.Now().Add(time.Second*time.Duration(86400)).Format(http.TimeFormat))},
			},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			ctx := context.Background()
			cfg := &Config{
				DefaultHeaders: test.defaultHeaders,
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
				t.Errorf("maps not equals\nactual:   %v\nexpected: %v", req.Header, test.expected)
			}
		})
	}
}

func TestRegexpRules(t *testing.T) {
	defaultHeaders := map[string]Header{
		"Expires":       {Action: "set", Value: "0"},
		"Cache-Control": {Action: "set", Value: "no-cache, no-store, max-age=0, must-revalidate"},
		"Pragma":        {Action: "set", Value: "no-cache"},
		"Cache-Test":    {Action: "set", Value: "NO CACHE - DEFAULT"},
	}
	rules := []Rule{
		{
			Name:   "No Cache by URL",
			Regexp: `(nocache|no-cache)`,
			ResponseHeaders: map[string]Header{
				"Cache-Control": {Action: "set", Value: "public, max-age=86400"},
				"Expires":       {Action: "set", Value: "@DT_ADD#86400@"},
				"Cache-Test":    {Action: "set", Value: "NO CACHE"},
			},
		},
		{
			Name:   "Long Cache by URL",
			Regexp: `\.cache\.`,
			ResponseHeaders: map[string]Header{
				"Cache-Control": {Action: "set", Value: "public, max-age=86400"},
				"Expires":       {Action: "set", Value: "@DT_ADD#86400@"},
				"Cache-Test":    {Action: "set", Value: "LONG CACHE"},
			},
		},
		{
			Name:   "Low Cache by Extension",
			Regexp: "(jpe?g|gif|png|js)$",
			ResponseHeaders: map[string]Header{
				"Cache-Control": {Action: "set", Value: "public, max-age=86400"},
				"Expires":       {Action: "set", Value: "@DT_ADD#86400@"},
				"Cache-Test":    {Action: "set", Value: "LOW CACHE"},
			},
		},
	}

	testCases := []struct {
		desc               string
		url                string
		headers            http.Header
		defaultHeaders     map[string]Header
		rules              []Rule
		respHeaderExpected http.Header
	}{
		{
			desc:           "http://test01.uri.com/test.gif",
			url:            "http://test01.uri.com/test.gif",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LOW CACHE"},
			},
		},
		{
			desc:           "http://test02.uri.com/test.png",
			url:            "http://test02.uri.com/test.png",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LOW CACHE"},
			},
		},
		{
			desc:           "http://test03.uri.com/test.jpg",
			url:            "http://test03.uri.com/test.jpg",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LOW CACHE"},
			},
		},
		{
			desc:           "http://test04.uri.com/test.jpeg",
			url:            "http://test04.uri.com/test.jpeg",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LOW CACHE"},
			},
		},
		{
			desc:           "http://test05.uri.com/test.js",
			url:            "http://test05.uri.com/test.js",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LOW CACHE"},
			},
		},
		{
			desc:           "http://test06.uri.com/test.html",
			url:            "http://test06.uri.com/test.html",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"no-cache", "no-store", "max-age=0", "must-revalidate"},
				"Expires":       []string{"0"},
				"Pragma":        []string{"no-cache"},
				"Cache-Test":    []string{"NO CACHE - DEFAULT"},
			},
		},
		{
			desc:           "http://test07.uri.com/testnocache.jpeg",
			url:            "http://test07.uri.com/testnocache.jpeg",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"NO CACHE"},
			},
		},
		{
			desc:           "http://test08.uri.com/testnocache.html",
			url:            "http://test08.uri.com/testnocache.html",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"NO CACHE"},
			},
		},
		{
			desc:           "http://test09.uri.com/test-no-cache-10.js",
			url:            "http://test09.uri.com/test-no-cache-10.js",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"NO CACHE"},
			},
		},
		{
			desc:           "http://test10.uri.com/test-no-cache-10.html",
			url:            "http://test10.uri.com/test-no-cache-10.html",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"NO CACHE"},
			},
		},
		{
			desc:           "http://test11.uri.com/test-cache-10.js",
			url:            "http://test11.uri.com/test-cache-10.js",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LOW CACHE"},
			},
		},
		{
			desc:           "http://test12.uri.com/test-cache-10.html",
			url:            "http://test12.uri.com/test-cache-10.html",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"no-cache", "no-store", "max-age=0", "must-revalidate"},
				"Expires":       []string{"0"},
				"Pragma":        []string{"no-cache"},
				"Cache-Test":    []string{"NO CACHE - DEFAULT"},
			},
		},
		{
			desc:           "http://test13.uri.com/test.cache.10.js",
			url:            "http://test13.uri.com/test.cache.10.js",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LONG CACHE"},
			},
		},
		{
			desc:           "http://test14.uri.com/test.cache.10.html",
			url:            "http://test14.uri.com/test.cache.10.html",
			defaultHeaders: defaultHeaders,
			rules:          rules,
			respHeaderExpected: http.Header{
				"Cache-Control": []string{"public", "max-age=86400"},
				"Expires":       []string{time.Now().Add(time.Second * time.Duration(86400)).Format(http.TimeFormat)},
				"Cache-Test":    []string{"LONG CACHE"},
			},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			ctx := context.Background()
			cfg := &Config{
				DefaultHeaders: test.defaultHeaders,
				Rules:          test.rules,
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
