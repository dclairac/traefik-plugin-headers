package traefik_plugin_headers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	plugin "github.com/dclairac/traefik-plugin-headers"
)

type testUri struct {
	location string
	key      string
	expected string
}

func TestDefaultRules(t *testing.T) {
	cfg := plugin.CreateConfig()
	cfg.DefaultHeaders = []plugin.HeaderChange{
		{Header: "default set req", Req: true, Action: "set", Value: "YES"},
		{Header: "default set resp", Req: false, Action: "set", Value: "YES"},
		{Header: "default set default", Action: "set", Value: "YES"},
		{Header: "default set overide req", Value: "YES", Req: true, Action: "set"},
		{Header: "default unset req", Req: true, Action: "unset"},
		{Header: "default unset req not existing", Req: true, Action: "unset"},
		{Header: "default unset resp", Req: false, Action: "unset"},
		{Header: "default append sep req", Req: true, Action: "append", Sep: ", ", Value: "ADDED"},
		{Header: "default append sep req not existing", Req: true, Action: "append", Sep: ", ", Value: "ADDED"},
		{Header: "default append sep resp", Req: false, Action: "append", Sep: ", ", Value: "ADDED"},
		{Header: "default append nosep req", Req: true, Action: "append", Value: "ADDED"},
		{Header: "default edit req not existing", Req: true, Action: "edit", Sep: ", ", Value: "YES"},
		{Header: "default edit req wrong regexp", Req: true, Action: "edit", Sep: ", ", Replace: "max-age=[0-9]+", Value: "max-age=1000"},
		{Header: "default edit req good regexp", Req: true, Action: "edit", Sep: ", ", Replace: "max-age=[0-9]+", Value: "max-age=1000"},
		{Header: "default edit resp good regexp", Req: false, Action: "edit", Sep: ", ", Replace: "max-age=[0-9]+", Value: "max-age=1000"},
		{Header: "default edit req DT_ADD regexp", Req: true, Action: "edit", Sep: ", ", Replace: "expires=[^,]+", Value: "expires=@DT_ADD#86400@"},
	}

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	handler, err := plugin.New(ctx, next, cfg, "traefik_plugin_headers_test")
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	//Add some Headers (req/resp) to check overriding
	req.Header.Set("default set overide req", "NO")
	req.Header.Set("default unset req", "NO")
	recorder.Header().Set("default unset resp", "NO")
	req.Header.Set("default append sep req", "FIRST")
	recorder.Header().Set("default append sep resp", "FIRST")
	req.Header.Set("default append nosep req", "FIRST")
	req.Header.Set("default edit req wrong regexp", "no-cache, maxi-age=12089, expires 0")
	req.Header.Set("default edit req good regexp", "no-cache, max-age=12089, expires 0")
	recorder.Header().Set("default edit resp good regexp", "no-cache, max-age=12089, expires 0")
	req.Header.Set("default edit req DT_ADD regexp", "no-cache, expires=12089, max-age=toto")

	handler.ServeHTTP(recorder, req)

	//TEST SET
	assertReqHeader(t, req, "default set req", "YES")
	assertReqHeader(t, req, "default set overide req", "YES")
	assertRespHeader(t, recorder, "default set resp", "YES")
	assertRespHeader(t, recorder, "default set default", "YES")

	//TEST UNSET
	assertReqHeader(t, req, "default unset req", "")
	assertReqHeader(t, req, "default unset req not existing", "")
	assertRespHeader(t, recorder, "default unset resp", "")

	//TEST APPEND
	assertReqHeader(t, req, "default append sep req", "FIRST, ADDED")
	assertReqHeader(t, req, "default append sep req not existing", "ADDED")
	assertRespHeader(t, recorder, "default append sep resp", "FIRST, ADDED")
	assertReqMultipleHeader(t, req, "default append nosep req", "ADDED")

	//TEST EDIT
	assertReqHeader(t, req, "default edit req not existing", "YES")
	assertReqHeader(t, req, "default edit req wrong regexp", "no-cache, maxi-age=12089, expires 0, max-age=1000")
	assertReqHeader(t, req, "default edit req good regexp", "no-cache, max-age=1000, expires 0")
	assertRespHeader(t, recorder, "default edit resp good regexp", "no-cache, max-age=1000, expires 0")
	assertReqDtAddHeader(t, req, "default edit req DT_ADD regexp", 86400)
}

func TestRegexpRules(t *testing.T) {
	cfg := plugin.CreateConfig()
	cfg.DefaultHeaders = []plugin.HeaderChange{
		{Header: "Expires", Req: false, Action: "set", Value: "0"},
		{Header: "Cache-Control", Req: false, Action: "set", Value: "no-cache, no-store, max-age=0, must-revalidate"},
		{Header: "Pragma", Req: false, Action: "set", Value: "no-cache"},
		{Header: "Cache-Test", Req: false, Action: "set", Value: "NO CACHE - DEFAULT"},
	}
	cfg.Rules = []plugin.Rule{
		{
			Name:   "Low Cache by Extension",
			Regexp: `(jpe?g|gif|png|js)$`,
			HeaderChanges: []plugin.HeaderChange{
				{Header: "Cache-Control", Req: false, Action: "set", Value: "public, max-age=86400"},
				{Header: "Expires", Req: false, Action: "set", Value: "@DT_ADD#86400@"},
				{Header: "Cache-Test", Req: false, Action: "set", Value: "LOW CACHE"},
			},
		}, {
			Name:   "No Cache by URL",
			Regexp: `(nocache|no-cache)`,
			HeaderChanges: []plugin.HeaderChange{
				{Header: "Cache-Control", Req: false, Action: "set", Value: "public, max-age=86400"},
				{Header: "Expires", Req: false, Action: "set", Value: "@DT_ADD#86400@"},
				{Header: "Cache-Test", Req: false, Action: "set", Value: "NO CACHE"},
			},
		}, {
			Name:   "Long Cache by URL",
			Regexp: `\.cache\.`,
			HeaderChanges: []plugin.HeaderChange{
				{Header: "Cache-Control", Req: false, Action: "set", Value: "public, max-age=86400"},
				{Header: "Expires", Req: false, Action: "set", Value: "@DT_ADD#86400@"},
				{Header: "Cache-Test", Req: false, Action: "set", Value: "LONG CACHE"},
			},
		},
	}

	toTest := []testUri{
		{location: "http://test01.uri.com/test.gif", key: "Cache-Test", expected: "LOW CACHE"},
		{location: "http://test02.uri.com/test.png", key: "Cache-Test", expected: "LOW CACHE"},
		{location: "http://test03.uri.com/test.jpg", key: "Cache-Test", expected: "LOW CACHE"},
		{location: "http://test04.uri.com/test.jpeg", key: "Cache-Test", expected: "LOW CACHE"},
		{location: "http://test05.uri.com/test.js", key: "Cache-Test", expected: "LOW CACHE"},
		{location: "http://test06.uri.com/test.html", key: "Cache-Test", expected: "NO CACHE - DEFAULT"},
		{location: "http://test07.uri.com/testnocache.jpeg", key: "Cache-Test", expected: "NO CACHE"},
		{location: "http://test08.uri.com/testnocache.html", key: "Cache-Test", expected: "NO CACHE"},
		{location: "http://test09.uri.com/test-no-cache-10.js", key: "Cache-Test", expected: "NO CACHE"},
		{location: "http://test10.uri.com/test-no-cache-10.html", key: "Cache-Test", expected: "NO CACHE"},
		{location: "http://test11.uri.com/test-cache-10.js", key: "Cache-Test", expected: "LOW CACHE"},
		{location: "http://test12.uri.com/test-cache-10.html", key: "Cache-Test", expected: "NO CACHE - DEFAULT"},
		{location: "http://test13.uri.com/test.cache.10.js", key: "Cache-Test", expected: "LONG CACHE"},
		{location: "http://test14.uri.com/test.cache.10.html", key: "Cache-Test", expected: "LONG CACHE"},
	}

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	handler, err := plugin.New(ctx, next, cfg, "traefik_plugin_headers_test")
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range toTest {
		recorder := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, test.location, nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(recorder, req)
		assertRespHeader(t, recorder, test.key, test.expected)
	}
}

func assertReqHeader(t *testing.T, req *http.Request, key, expected string) {
	t.Helper()

	if req.Header.Get(key) != expected {
		t.Errorf("test [%s]: invalid header Value: [%s]", key, req.Header.Get(key))
	}
}
func assertRespHeader(t *testing.T, resp *httptest.ResponseRecorder, key, expected string) {
	t.Helper()

	if resp.Header().Get(key) != expected {
		t.Errorf("test [%s]: invalid header Value: [%s]", key, resp.Header().Get(key))
	}
}
func assertReqMultipleHeader(t *testing.T, req *http.Request, key, expected string) {
	t.Helper()

	if req.Header.Values(key)[len(req.Header.Values(key))-1] != expected {
		t.Errorf("test [%s]: invalid header Value: [%s]", key, req.Header.Values(key)[len(req.Header.Values(key))-1])
	}
}
func assertReqDtAddHeader(t *testing.T, req *http.Request, key string, nbSec int) {
	t.Helper()
	if strings.Contains(req.Header.Get(key), time.Now().Add(time.Second*time.Duration(nbSec)).Format("Mon Jan 2")) {
		t.Errorf("test [%s]: invalid header Value: [%s]", key, req.Header.Get(key))
	}
}
