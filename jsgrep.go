package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zncoder/check"
	"github.com/zncoder/mygo"
)

func loadJSON(jsonFile string) any {
	var f *os.File
	if jsonFile == "-" {
		f = maybeLoadCacheFile()
	} else {
		f = check.V(os.Open(jsonFile)).F("open json file", "file", jsonFile)
	}
	defer f.Close()

	var js any
	dec := json.NewDecoder(f)
	dec.UseNumber()
	check.E(dec.Decode(&js)).F("decode json from stdin")
	return js
}

func cacheFilename() string {
	cacheFile := os.Getenv("JSGREP_STDIN")
	if cacheFile == "" {
		// get user id
		cacheFile = filepath.Join(os.TempDir(), fmt.Sprintf("jsgrep-stdin-%d.json", os.Getuid()))
	}
	return cacheFile
}

func maybeLoadCacheFile() *os.File {
	cacheFile := cacheFilename()
	fi := check.V(os.Stdin.Stat()).F("stat stdin")
	if fi.Mode()&os.ModeCharDevice == 0 {
		// If stdin is a pipe, we read from it directly and cache the content.
		b := check.V(io.ReadAll(os.Stdin)).F("read stdin")
		check.E(os.WriteFile(cacheFile, b, 0600)).F("write cache file", "file", cacheFile)
	}
	return check.V(os.Open(cacheFile)).F("open cache file", "file", cacheFile)
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
	isKey := flag.Bool("k", false, "regexp is key")
	isVal := flag.Bool("v", false, "regexp is value")
	jsonFile := flag.String("f", "-", "json file")
	mygo.ParseFlag("[regexp]")
	if *isKey || *isVal {
		check.T(flag.NArg() > 0).F("missing regexp")
	}

	var keyPat, valPat string
	if *isKey {
		keyPat = flag.Arg(0)
	} else if *isVal {
		valPat = flag.Arg(0)
	} else if flag.NArg() > 0 {
		keyPat = flag.Arg(0)
		valPat = flag.Arg(0)
	}

	js := loadJSON(*jsonFile)
	flattened := flattenJSON("", js)

	var matched []jsonEntry
	if keyPat == "" && valPat == "" {
		matched = flattened
	} else {
		matched = matchEntries(flattened, keyPat, valPat)
	}

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
		case keyRe != nil && keyRe.MatchString(strings.ToLower(je.Key)):
			matched = append(matched, je)
		case valueRe != nil && matchValue(valueRe, je.Value):
			matched = append(matched, je)
		}
	}
	return matched
}
