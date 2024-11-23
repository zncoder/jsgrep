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

type jsonObject struct {
	Key string
	Obj map[string]any
}

func matchObject(obj map[string]any, key string, val any, match func(string, any) bool) bool {
	for k, sv := range obj {
		if match(k, sv) {
			return true
		}
	}
	return false
}

func walkObjectTree(
	keyPrefix []jsonKey,
	key string,
	val any,
	match func(string, any) bool,
) ([]jsonEntry, []jsonObject) {
	var matched []jsonEntry
	var objs []jsonObject
	switch v := val.(type) {
	case map[string]any:
		if matchObject(v, key, val, match) {
			objs = append(objs, jsonObject{Key: toJqKey(keyPrefix), Obj: v})
		}

		for k, sv := range v {
			submatched, subobjs := walkObjectTree(append(keyPrefix, jsonKey{Key: k}), k, sv, match)
			if len(submatched) > 0 {
				matched = append(matched, submatched...)
			}
			if len(subobjs) > 0 {
				objs = append(objs, subobjs...)
			}
		}
	case []any:
		for i, sv := range v {
			submatched, subobjs := walkObjectTree(append(keyPrefix, jsonKey{Index: i}), key, sv, match)
			if len(submatched) > 0 {
				matched = append(matched, submatched...)
			}
			if len(subobjs) > 0 {
				objs = append(objs, subobjs...)
			}
		}
	case string, json.Number, bool, nil:
		if match(key, v) {
			matched = append(matched, jsonEntry{Key: toJqKey(keyPrefix), Value: v})
		}
	}
	return matched, objs
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
	outputFormat := flag.String("o", "l", "output format: l/line, j/json, i/indent, k/key, c/count, a/array, o/obj")
	key := flag.String("k", "", "key regexp")
	value := flag.String("v", "", "value regexp, value is matched as string")
	filter := flag.String("f", "", "regexp to filter keys, / is replaced with [.]")
	flag.Parse()
	keyRe := regexp.MustCompile(*key)
	valRe := regexp.MustCompile(*value)
	var filterRe *regexp.Regexp
	if *filter != "" {
		filterRe = regexp.MustCompile(strings.Replace(*filter, "/", "[.]", -1))
	}

	js := loadJSON()
	matched, objs := walkObjectTree(nil, "", js, func(key string, v any) bool { return matchValue(keyRe, valRe, key, v) })

	if filterRe != nil {
		var m []jsonEntry
		for _, je := range matched {
			if filterRe.MatchString(je.Key) {
				m = append(m, je)
			}
		}
		matched = m
	}

	switch *outputFormat {
	case "l", "line":
		for _, je := range matched {
			fmt.Printf("%s %v\n", je.Key, je.Value)
		}

	case "j", "json":
		check.E(json.NewEncoder(os.Stdout).Encode(matched)).F("encode json to stdout")

	case "i", "indent":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "    ")
		check.E(enc.Encode(matched)).F("encode json to stdout")

	case "k", "key":
		for _, je := range matched {
			fmt.Println(je.Key)
		}

	case "c", "count":
		fmt.Println(len(matched))

	case "a", "array":
		var sb strings.Builder
		sb.WriteString("[")
		for i, je := range matched {
			if i != 0 {
				sb.WriteString(",")
			}
			sb.WriteString(je.Key)
		}
		sb.WriteString("]")
		fmt.Println(sb.String())

	case "v", "value":
		for _, je := range matched {
			fmt.Println(je.Value)
		}

	case "o", "obj":
		m := make(map[string]any)
		for _, jo := range objs {
			m[jo.Key] = jo.Obj
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "    ")
		check.E(enc.Encode(m)).F("encode json to stdout")

	default:
		check.T(false).F("unknown format", "arg", *outputFormat)
	}
}
