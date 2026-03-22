/*
Copyright 2026 Dmitry Ponomaryov

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nsselector

import (
	"fmt"
	"path"
	"regexp"
)

type MatchType string

const (
	MatchTypeGlob   MatchType = "Glob"
	MatchTypeRegexp MatchType = "Regexp"
)

type Selector struct {
	matchType MatchType
	include   []matcher
	exclude   []matcher
}

type matcher interface {
	MatchString(string) bool
	String() string
}

type globMatcher string

func (g globMatcher) MatchString(s string) bool {
	ok, _ := path.Match(string(g), s)
	return ok
}
func (g globMatcher) String() string {
	return string(g)
}

type regexpMatcher struct {
	re *regexp.Regexp
}

func (r regexpMatcher) MatchString(s string) bool {
	return r.re.MatchString(s)
}
func (r regexpMatcher) String() string {
	return r.re.String()
}

func Compile(matchType MatchType, include, exclude []string) (*Selector, error) {
	if len(include) == 0 {
		switch matchType {
		case "", MatchTypeGlob:
			include = []string{"*"}
		case MatchTypeRegexp:
			include = []string{".*"}
		default:
			return nil, fmt.Errorf("unsupported matchType %q", matchType)
		}
	}
	s := &Selector{
		matchType: matchType,
		include:   make([]matcher, 0, len(include)),
		exclude:   make([]matcher, 0, len(exclude)),
	}
	var err error
	s.include, err = compileMatchers(matchType, include)
	if err != nil {
		return nil, fmt.Errorf("compile include patterns: %w", err)
	}
	s.exclude, err = compileMatchers(matchType, exclude)
	if err != nil {
		return nil, fmt.Errorf("compile exclude patterns: %w", err)
	}
	return s, nil
}

func (s *Selector) Match(name string) bool {
	included := false
	for _, m := range s.include {
		if m.MatchString(name) {
			included = true
			break
		}
	}
	if !included {
		return false
	}
	for _, m := range s.exclude {
		if m.MatchString(name) {
			return false
		}
	}
	return true
}

func compileMatchers(matchType MatchType, patterns []string) ([]matcher, error) {
	out := make([]matcher, 0, len(patterns))
	switch matchType {
	case "", MatchTypeGlob:
		for _, p := range patterns {
			out = append(out, globMatcher(p))
		}
		return out, nil
	case MatchTypeRegexp:
		for _, p := range patterns {
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("invalid regexp %q: %w", p, err)
			}
			out = append(out, regexpMatcher{re: re})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported matchType %q", matchType)
	}
}
