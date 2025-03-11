package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/zncoder/check"
	"github.com/zncoder/mygo"
)

func loadJSON() any {
	f := os.Stdin
	if flag.NArg() > 0 {
		f = check.V(os.Open(flag.Arg(0))).F("open json file")
		defer f.Close()
	}

	var js any
	dec := json.NewDecoder(f)
	dec.UseNumber()
	check.E(dec.Decode(&js)).F("decode json from stdin")
	return js
}

type jsonEntry struct {
	Key   string
	Value any
}

func quoteKey(k string) string {
	for i, r := range k {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', r == '_':
		case '0' <= r && r <= '9' && i != 0:
		default:
			return fmt.Sprintf(`"%s"`, k)
		}
	}
	return k
}

func flattenJSON(prefix string, js any) []jsonEntry {
	var entries []jsonEntry
	switch v := js.(type) {
	case map[string]any:
		for k, sv := range v {
			entries = append(entries, flattenJSON(prefix+"."+quoteKey(k), sv)...)
		}
	case []any:
		for i, sv := range v {
			entries = append(entries, flattenJSON(fmt.Sprintf("%s[%d]", prefix, i), sv)...)
		}
	case string, json.Number, bool, nil:
		entries = append(entries, jsonEntry{Key: prefix, Value: v})
	default:
		check.T(false).F("unknown type", "type", fmt.Sprintf("%T", v))
	}
	return entries
}

func main() {
	keyPat := flag.String("k", "", "key regexp, key is flattened in jq key format")
	valuePat := flag.String("v", "", "value regexp, value is matched as string")
	mygo.ParseFlag("[json_file]")

	js := loadJSON()
	flattened := flattenJSON("", js)

	matched := matchEntries(flattened, *keyPat, *valuePat)
	for _, je := range matched {
		fmt.Printf("%s %s\n", je.Key, formatValue(je.Value))
	}
}

func formatValue(val any) string {
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case json.Number:
		return v.String()
	case bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return "null"
	}
	return ""
}

func matchValue(valueRe *regexp.Regexp, val any) bool {
	switch v := val.(type) {
	case string:
		return valueRe.MatchString(strings.ToLower(v))
	case json.Number:
		return valueRe.MatchString(v.String())
	case bool:
		return valueRe.MatchString(fmt.Sprintf("%v", v))
	case nil:
		return valueRe.MatchString("null")
	}
	return false
}

func matchEntries(flattened []jsonEntry, keyPat, valuePat string) (matched []jsonEntry) {
	var keyRe, valueRe *regexp.Regexp
	if keyPat != "" {
		keyRe = check.V(regexp.Compile(strings.ToLower(keyPat))).F("compile key regexp")
	}
	if valuePat != "" {
		valueRe = check.V(regexp.Compile(strings.ToLower(valuePat))).F("compile value regexp")
	}
	for _, je := range flattened {
		switch {
		case keyRe != nil && !keyRe.MatchString(strings.ToLower(je.Key)),
			valueRe != nil && !matchValue(valueRe, je.Value):
			continue
		default:
			matched = append(matched, je)
		}
	}
	return matched
}
