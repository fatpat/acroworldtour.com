package importer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// collections are the export members the importer reads (one JSON-lines file
// per collection).
var collections = []string{"pilots", "teams", "judges", "seasons", "files", "tricks", "competitions"}

// source gives access to the export members, from a directory or a .tar.gz.
type source struct {
	files map[string][]byte // collection name -> JSON lines
}

// openSource loads the known collections of the export at src. A tar.gz is
// read in one pass (members like "./pilots.json"); a directory holds
// "<collection>.json" files. Missing collections are simply absent.
func openSource(src string) (*source, error) {
	st, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("open export: %w", err)
	}
	s := &source{files: map[string][]byte{}}
	if st.IsDir() {
		for _, c := range collections {
			b, err := os.ReadFile(filepath.Join(src, c+".json"))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", c, err)
			}
			s.files[c] = b
		}
		return s, nil
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("open export: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gunzip %s: %w", src, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := strings.TrimSuffix(path.Base(h.Name), ".json")
		if !strings.HasSuffix(h.Name, ".json") || !isCollection(name) {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read tar member %s: %w", h.Name, err)
		}
		s.files[name] = b
	}
	return s, nil
}

func isCollection(name string) bool {
	for _, c := range collections {
		if c == name {
			return true
		}
	}
	return false
}

// decodeLines decodes every JSON value of a collection into a T. A missing
// collection yields no rows.
func decodeLines[T any](s *source, collection string) ([]T, error) {
	dec := json.NewDecoder(bytes.NewReader(s.files[collection]))
	var out []T
	for {
		var v T
		if err := dec.Decode(&v); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, fmt.Errorf("decode %s row %d: %w", collection, len(out)+1, err)
		}
		out = append(out, v)
	}
}

// extNumber reads a number that may be plain or MongoDB extended JSON
// ({"$numberInt": "1"}, {"$numberLong": "1"}, {"$numberDouble": "1"}).
func extNumber(b []byte) (float64, bool, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return 0, false, nil
	}
	if b[0] == '{' {
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			return 0, false, fmt.Errorf("extended number %s: %w", b, err)
		}
		for _, k := range []string{"$numberInt", "$numberLong", "$numberDouble"} {
			if v, ok := m[k]; ok {
				f, err := strconv.ParseFloat(v, 64)
				return f, err == nil, err
			}
		}
		return 0, false, fmt.Errorf("unsupported extended number %s", b)
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return 0, false, fmt.Errorf("number %s: %w", b, err)
	}
	return f, true, nil
}

// Int is an integer that may be plain or extended JSON; Valid is false for null.
type Int struct {
	V     int64
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler.
func (i *Int) UnmarshalJSON(b []byte) error {
	f, ok, err := extNumber(b)
	if err != nil {
		return err
	}
	i.V, i.Valid = int64(f), ok
	return nil
}

// ID is a document id that may be a string, a number or {"$oid": "..."}.
type ID string

// UnmarshalJSON implements json.Unmarshaler.
func (id *ID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case len(b) == 0 || string(b) == "null":
		*id = ""
	case b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*id = ID(s)
	case b[0] == '{':
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
		if raw, ok := m["$oid"]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				return err
			}
			*id = ID(s)
			return nil
		}
		f, ok, err := extNumber(b)
		if err != nil || !ok {
			return fmt.Errorf("unsupported id %s", b)
		}
		*id = ID(strconv.FormatInt(int64(f), 10))
	default:
		f, _, err := extNumber(b)
		if err != nil {
			return err
		}
		*id = ID(strconv.FormatInt(int64(f), 10))
	}
	return nil
}

// Date is a timestamp normalised to RFC 3339 UTC text ("" for null). It reads
// legacy naive ISO strings (taken as UTC), plain dates and {"$date": ...}.
type Date string

var dateLayouts = []string{
	time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02",
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Date) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*d = ""
		return nil
	}
	if b[0] == '{' {
		var m struct {
			Date json.RawMessage `json:"$date"`
		}
		if err := json.Unmarshal(b, &m); err != nil || m.Date == nil {
			return fmt.Errorf("unsupported date %s", b)
		}
		b = bytes.TrimSpace(m.Date)
		if b[0] != '"' { // {"$numberLong": ms} or a bare number of ms
			f, ok, err := extNumber(b)
			if err != nil || !ok {
				return fmt.Errorf("unsupported date %s", b)
			}
			*d = Date(time.UnixMilli(int64(f)).UTC().Format(time.RFC3339))
			return nil
		}
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("date %s: %w", b, err)
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			*d = Date(t.UTC().Format(time.RFC3339))
			return nil
		}
	}
	return fmt.Errorf("unparseable date %q", s)
}

func (d Date) null() sql.NullString {
	return sql.NullString{String: string(d), Valid: d != ""}
}

// rawJSONArray returns the raw JSON of an array field, "[]" for null/absent.
func rawJSONArray(r json.RawMessage) string {
	s := strings.TrimSpace(string(r))
	if s == "" || s == "null" {
		return "[]"
	}
	return s
}
