package traefik_plugin_headers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//Structure to store Headers transformations instructions
type HeaderChange struct {
	Name    string `yaml:"name"`
	Header  string `yaml:"header"`
	Req     bool   `yaml:"request"`
	Value   string `yaml:"value"`
	Replace string `yaml:"replace"`
	Sep     string `yaml:"sep"`
	Action  string `yaml:"action"`
}

//Structure to store RequestURI regexp and headers associated
type Rule struct {
	Name          string         `yaml:"name"`
	Regexp        string         `yaml:"regexp"`
	HeaderChanges []HeaderChange `yaml:"headerChanges"`
}

/*
  Plugin Configuration Structure
  - rules (optional): List of regex rules to select if headers transformations are necessary
  - defaultHeaders (optional): Headers transformations to apply if no other rule match
*/
type Config struct {
	DefaultHeaders []HeaderChange `yaml:"defaultHeaders"`
	Rules          []Rule         `yaml:"rules"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		DefaultHeaders: []HeaderChange{},
		Rules:          []Rule{},
	}
}

// TraefikPluginHeader a plugin to alter headers based on URL regexp rules.
type TraefikPluginHeader struct {
	next           http.Handler
	defaultHeaders []HeaderChange
	rules          []Rule
	name           string
}

// New created a new TraefikPluginHeader plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	//TODO: Check Configuration Here
	return &TraefikPluginHeader{
		defaultHeaders: config.DefaultHeaders,
		rules:          config.Rules,
		next:           next,
		name:           name,
	}, nil
}

func (a *TraefikPluginHeader) applyheaderChanges(hr []HeaderChange, rw http.ResponseWriter, req *http.Request) {
	for _, headerChange := range hr {
		if headerChange.Action != "unset" {
			//No need to check for Value replacement for unset type
			dateAddRe := regexp.MustCompile(`@DT_ADD#([\d]+)@`)
			if dateAddRe.MatchString(headerChange.Value) {
				//Extract the number of second to add at current date
				nbSecToAdd, err := strconv.Atoi(dateAddRe.FindStringSubmatch(headerChange.Value)[1])
				if err != nil {
					nbSecToAdd = 0
					//TODO: LOG CONVERT ERROR - SET 0
				}
				//Replace @DT_ADD#nb_seconds@ by calculated (now + nb seconds) datetime formatted with HTTP timeformat
				newDate := time.Now().Add(time.Second * time.Duration(nbSecToAdd)).Format(http.TimeFormat)
				headerChange.Value = dateAddRe.ReplaceAllString(headerChange.Value, newDate)
			}
		}
		switch headerChange.Action {
		case "set":
			if headerChange.Req {
				req.Header.Set(headerChange.Header, headerChange.Value)
			} else {
				rw.Header().Set(headerChange.Header, headerChange.Value)
			}
		case "unset":
			if headerChange.Req {
				req.Header.Del(headerChange.Header)
			} else {
				rw.Header().Del(headerChange.Header)
			}
		case "edit":
			if headerChange.Req {
				if strings.TrimSpace(req.Header.Get(headerChange.Header)) == "" {
					// Header not exist or is empty => Add header
					req.Header.Set(headerChange.Header, headerChange.Value)
				} else {
					re := regexp.MustCompile(headerChange.Replace)
					req.Header.Set(headerChange.Header, re.ReplaceAllString(req.Header.Get(headerChange.Header), headerChange.Value))

					if !strings.Contains(req.Header.Get(headerChange.Header), headerChange.Value) {
						//Regexp was not found, replacement was not done, add value to the end with separator
						req.Header.Set(headerChange.Header, req.Header.Get(headerChange.Header)+headerChange.Sep+headerChange.Value)
					}
				}
			} else {
				if strings.TrimSpace(rw.Header().Get(headerChange.Header)) == "" {
					// Header not exist or is empty => Add header
					rw.Header().Set(headerChange.Header, headerChange.Value)
				} else {
					re := regexp.MustCompile(headerChange.Replace)
					rw.Header().Set(headerChange.Header, re.ReplaceAllString(rw.Header().Get(headerChange.Header), headerChange.Value))
					if !strings.Contains(rw.Header().Get(headerChange.Header), headerChange.Value) {
						//Regexp was not found, replacement was not done, add value to the end with separator
						rw.Header().Set(headerChange.Header, rw.Header().Get(headerChange.Header)+headerChange.Sep+headerChange.Value)
					}
				}
			}

		case "append":
			if headerChange.Req {
				if headerChange.Sep != "" {
					//Sep is defined, we add header at the end of existing one
					if strings.TrimSpace(req.Header.Get(headerChange.Header)) == "" {
						// Header was not existing, create it
						req.Header.Set(headerChange.Header, headerChange.Value)
					} else {
						// Header already exist, add separator and value
						req.Header.Set(headerChange.Header, req.Header.Get(headerChange.Header)+headerChange.Sep+headerChange.Value)
					}
				} else {
					//Sep is undefined, add Header to the list
					req.Header.Add(headerChange.Header, headerChange.Value)
				}
			} else {
				if headerChange.Sep != "" {
					//Sep is defined, we add header at the end of existing one
					if strings.TrimSpace(rw.Header().Get(headerChange.Header)) == "" {
						// Header was not existing, create it
						rw.Header().Set(headerChange.Header, headerChange.Value)
					} else {
						// Header already exist, add separator and value
						rw.Header().Set(headerChange.Header, rw.Header().Get(headerChange.Header)+headerChange.Sep+headerChange.Value)
					}
				} else {
					//Sep is undefined, add Header to the list
					rw.Header().Add(headerChange.Header, headerChange.Value)
				}
			}
		default:
			//TODO LOG ERROR
			fmt.Printf("unknown action value for header rule [%s]. Valid actions are (set|unset|edit|append)\n", headerChange.Name)
		}
	}
}

func (a *TraefikPluginHeader) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	applyDefault := true
	for _, rule := range a.rules {
		//Check if one of the rules match and apply headers transformation if it's the case
		fmt.Printf("evaluate [%s] rule)\n", rule.Name)
		reqMatch := regexp.MustCompile(rule.Regexp)
		if reqMatch.MatchString(req.URL.Path) {
			a.applyheaderChanges(rule.HeaderChanges, rw, req)
			applyDefault = false
		}
	}
	if len(a.defaultHeaders) > 0 && applyDefault {
		// Apply defaults only if no rules was used for the request
		a.applyheaderChanges(a.defaultHeaders, rw, req)
	}
	a.next.ServeHTTP(rw, req)
}
