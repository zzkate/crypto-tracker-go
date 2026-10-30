package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var symbolRe = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)

type httpError struct {
	Status  int
	Code    string
	Message string
}

func (e *httpError) Error() string { return e.Message }

func badRequest(format string, args ...any) *httpError {
	return &httpError{Status: http.StatusBadRequest, Code: "validation_error", Message: fmt.Sprintf(format, args...)}
}

func parseID(s string) (int64, error) {
	if s == "" {
		return 0, badRequest("id must be a positive integer")
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, badRequest("id must be a positive integer")
	}
	return id, nil
}

func queryInt(q url.Values, name string, fallback, min, max int) (int, error) {
	v := q.Get(name)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < min || n > max {
		return 0, badRequest("%s must be between %d and %d", name, min, max)
	}
	return n, nil
}

func parsePagination(q url.Values) (limit, offset int, err error) {
	if limit, err = queryInt(q, "limit", 100, 1, 1000); err != nil {
		return
	}
	offset, err = queryInt(q, "offset", 0, 0, 1<<62)
	return
}

func parseDate(q url.Values, name string) (string, error) {
	v := q.Get(name)
	if v == "" {
		return "", nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return "", badRequest("%s must be a valid ISO date", name)
	}
	return canonTime(t), nil
}

// parseCoinInput разбирает тело монеты. partial=true — для PATCH.
func parseCoinInput(body []byte, partial bool) (symbol, name *string, err error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, nil, &httpError{Status: http.StatusBadRequest, Code: "invalid_json", Message: "request body must be valid JSON"}
	}

	if raw, ok := m["symbol"]; ok {
		s, ok := raw.(string)
		if !ok {
			return nil, nil, badRequest("symbol must be a string")
		}
		s = strings.ToUpper(strings.TrimSpace(s))
		if !symbolRe.MatchString(s) {
			return nil, nil, badRequest("symbol must be 2-10 alphanumeric characters")
		}
		symbol = &s
	}
	if raw, ok := m["name"]; ok {
		n, ok := raw.(string)
		if !ok {
			return nil, nil, badRequest("name must be a string")
		}
		n = strings.TrimSpace(n)
		if n == "" || len(n) > 100 {
			return nil, nil, badRequest("name must be 1-100 characters")
		}
		name = &n
	}

	if partial {
		if symbol == nil && name == nil {
			return nil, nil, badRequest("at least one of symbol or name is required")
		}
	} else if symbol == nil || name == nil {
		return nil, nil, badRequest("symbol and name are required")
	}
	return symbol, name, nil
}
