package config

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseEnvFile reads a docker compose env file (.env.prod, .env.training):
// KEY=VALUE lines, blank lines and # comments, an optional "export ",
// values optionally in matching single or double quotes (taken literally),
// and " #" comments after unquoted values. A later key wins. Errors name
// the line number only, never its content: these files hold secrets.
func ParseEnvFile(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || !envKey.MatchString(k) {
			return nil, fmt.Errorf("env file line %d: not KEY=VALUE", n)
		}
		v = strings.TrimSpace(v)
		switch {
		case v != "" && (v[0] == '"' || v[0] == '\''):
			if len(v) < 2 || v[len(v)-1] != v[0] {
				return nil, fmt.Errorf("env file line %d: unterminated quote", n)
			}
			v = v[1 : len(v)-1]
		default:
			if i := strings.Index(v, " #"); i >= 0 {
				v = strings.TrimSpace(v[:i])
			}
		}
		out[k] = v
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read env file: %w", err)
	}
	return out, nil
}
