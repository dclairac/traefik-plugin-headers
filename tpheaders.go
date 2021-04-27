// Package traefik_plugin_headers a plugin to c-edit headers.
package traefik_plugin_headers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Header holds Headers transformations instructions.
type Header struct {
	Description string `yaml:"name"`
	Value       string `yaml:"value"`
	Replace     string `yaml:"replace"`
	Action      string `yaml:"action"`
}

// Rule holds RequestURI regexp and headers associated.
type Rule struct {
	Name            string            `yaml:"name"`
	Regexp          string            `yaml:"regexp"`
	RequestHeaders  map[string]Header `yaml:"requestHeaders"`
	ResponseHeaders map[string]Header `yaml:"responseHeaders"`
}

// Config holds Plugin Configuration Structure.
//  - rules (optional): List of regex rules to select if headers transformations are necessary
//  - defaultHeaders (optional): Headers transformations to apply if no other rule match
type Config struct {
	Rules []Rule `yaml:"rules"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		Rules: []Rule{},
	}
}

// TraefikPluginHeader a plugin to alter headers based on URL regexp rules.
type TraefikPluginHeader struct {
	next  http.Handler
	rules []Rule
	name  string
}

// New created a new TraefikPluginHeader plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	// Check Configuration Here
	return &TraefikPluginHeader{
		rules: config.Rules,
		next:  next,
		name:  name,
	}, nil
}

func (h *TraefikPluginHeader) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	applyDefault := true

	for _, rule := range h.rules {
		// Check if one of the rules match and apply headers transformation if it's the case
		log.Printf("evaluate [%s] rule)\n", rule.Name)
		reqMatch := regexp.MustCompile(rule.Regexp)
		if (rule.Regexp == "NO_MATCH" && applyDefault) || reqMatch.MatchString(req.URL.Path) {
			// Changes are applied if:
			// - Regexp = "NO_MATCH" and no rule have match the request before
			// - Regexp Match the request
			editHeaders(rule.RequestHeaders, req.Header)
			applyDefault = false
		}
	}

	wrappedWriter := &responseWriter{
		ResponseWriter: rw,
		path:           req.URL.Path,
		rules:          h.rules,
	}

	h.next.ServeHTTP(wrappedWriter, req)

	bodyBytes := wrappedWriter.buffer.Bytes()

	if _, err := rw.Write(bodyBytes); err != nil {
		log.Printf("unable to write rewrited body: %v", err)
	}
}

func editHeaders(hr map[string]Header, headers http.Header) {
	for k, v := range hr {
		switch v.Action {
		case "set":
			datas := strings.Split(v.Value, ",")
			headers.Set(k, adapt(datas[0]))
			for _, s := range datas[1:] {
				headers.Add(k, adapt(s))
			}
		case "unset":
			headers.Del(k)
		case "edit":
			if headers.Get(k) == "" {
				// Header not exist or is empty => Set header
				datas := strings.Split(v.Value, ",")
				headers.Set(k, adapt(datas[0]))
				for _, s := range datas[1:] {
					headers.Add(k, adapt(s))
				}
			} else {
				re := regexp.MustCompile(v.Replace)
				headers.Set(k, adapt(re.ReplaceAllString(headers.Get(k), v.Value)))

				if !strings.Contains(headers.Get(k), adapt(v.Value)) {
					// Regexp was not found, replacement was not done, add value to the end with separator
					headers.Add(k, adapt(v.Value))
				}
			}

		case "append":
			// Header was not existing, create it
			headers.Add(k, adapt(v.Value))
		default:
			log.Printf("unknown action value for header rule [%s]. Valid actions are (set|unset|edit|append)\n", v.Description)
		}
	}
}

func adapt(s string) string {
	// No need to check for Value replacement for unset type
	dateAddRe := regexp.MustCompile(`@DT_ADD#([\d]+)@`)
	if !dateAddRe.MatchString(s) {
		return strings.TrimSpace(s)
	}

	// Extract the number of second to add at current date.
	nbSecToAdd, err := strconv.Atoi(dateAddRe.FindStringSubmatch(s)[1])
	if err != nil {
		nbSecToAdd = 0
	}
	// Replace @DT_ADD#nb_seconds@ by calculated (now + nb seconds) datetime formatted with HTTP time format.
	newDate := time.Now().Add(time.Second * time.Duration(nbSecToAdd)).Format(http.TimeFormat)

	return strings.TrimSpace(dateAddRe.ReplaceAllString(s, newDate))
}

type responseWriter struct {
	buffer      bytes.Buffer
	wroteHeader bool
	rules       []Rule
	path        string
	http.ResponseWriter
}

func (r *responseWriter) WriteHeader(statusCode int) {
	applyDefault := true

	for _, rule := range r.rules {
		// Check if one of the rules match and apply headers transformation if it's the case
		log.Printf("evaluate [%s] rule)\n", rule.Name)
		reqMatch := regexp.MustCompile(rule.Regexp)
		if (rule.Regexp == "NO_MATCH" && applyDefault) || reqMatch.MatchString(r.path) {
			editHeaders(rule.ResponseHeaders, r.ResponseWriter.Header())
			applyDefault = false
		}
	}

	r.wroteHeader = true

	// Delegates the Content-Length Header creation to the final body write.
	r.ResponseWriter.Header().Del("Content-Length")

	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseWriter) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	return r.buffer.Write(p)
}

func (r *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("%T is not a http.Hijacker", r.ResponseWriter)
	}

	return hijacker.Hijack()
}

func (r *responseWriter) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
