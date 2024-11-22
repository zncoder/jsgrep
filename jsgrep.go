package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/zncoder/check"
)

func loadJSON() any {
	var js any
	dec := json.NewDecoder(os.Stdin)
	dec.UseNumber()
	check.E(dec.Decode(&js)).F("decode json from stdin")
	return js
}

type jsonKey struct {
	Key   string
	Index int
}

func (jk jsonKey) String() string {
	if jk.Key != "" {
		return jk.Key
	} else {
		return fmt.Sprintf("[%d]", jk.Index)
	}
}

type jsonEntry struct {
	Key   string
	Value any
}

func toJqKey(keyPrefix []jsonKey) string {
	var sb strings.Builder
	for _, k := range keyPrefix {
		sb.WriteString(".")
		sb.WriteString(k.String())
	}
	return sb.String()
}

func (je jsonEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{je.Key: je.Value})
}

func walkObjectTree(keyPrefix []jsonKey, key string, val any, match func(string, any) bool) []jsonEntry {
	var matched []jsonEntry
	switch v := val.(type) {
	case map[string]any:
		for k, sv := range v {
			submatched := walkObjectTree(append(keyPrefix, jsonKey{Key: k}), k, sv, match)
			if len(submatched) > 0 {
				matched = append(matched, submatched...)
			}
		}
	case []any:
		for i, sv := range v {
			submatched := walkObjectTree(append(keyPrefix, jsonKey{Index: i}), key, sv, match)
			if len(submatched) > 0 {
				matched = append(matched, submatched...)
			}
		}
	case string, json.Number, bool, nil:
		if match(key, v) {
			matched = append(matched, jsonEntry{Key: toJqKey(keyPrefix), Value: v})
		}
	}
	return matched
}

func matchValue(keyRe, valRe *regexp.Regexp, key string, val any) bool {
	if key != "" && !keyRe.MatchString(key) {
		return false
	}

	switch v := val.(type) {
	case string:
		if valRe.MatchString(v) {
			return true
		}
	case json.Number:
		if valRe.MatchString(v.String()) {
			return true
		}
	case bool:
		if valRe.MatchString(fmt.Sprintf("%v", v)) {
			return true
		}
	case nil:
		if valRe.MatchString("null") {
			return true
		}
	}
	return false
}

func main() {
	indent := flag.Bool("i", false, "indent")
	keyOnly := flag.Bool("k", false, "key only")
	valueOnly := flag.Bool("r", false, "value only")
	countOnly := flag.Bool("c", false, "count only")
	filter := flag.String("f", "", "filter keys")
	flag.Parse()
	check.T(flag.NArg() == 2).F("usage: key_re value_re")
	keyRe := regexp.MustCompile(flag.Arg(0))
	valRe := regexp.MustCompile(flag.Arg(1))
	var filterRe *regexp.Regexp
	if *filter != "" {
		filterRe = regexp.MustCompile(strings.Replace(*filter, "/", "[.]", -1))
	}

	js := loadJSON()
	matched := walkObjectTree(nil, "", js, func(key string, v any) bool { return matchValue(keyRe, valRe, key, v) })

	if filterRe != nil {
		var m []jsonEntry
		for _, je := range matched {
			if filterRe.MatchString(je.Key) {
				m = append(m, je)
			}
		}
		matched = m
	}

	switch {
	case *keyOnly:
		for _, je := range matched {
			fmt.Println(je.Key)
		}

	case *valueOnly:
		for _, je := range matched {
			fmt.Println(je.Value)
		}

	case *countOnly:
		fmt.Println(len(matched))

	default:
		enc := json.NewEncoder(os.Stdout)
		if *indent {
			enc.SetIndent("", "    ")
		}
		check.E(enc.Encode(matched)).F("encode json to stdout")
	}
}
