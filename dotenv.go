package main

import (
	"os"
	"strings"
)

// loadDotEnv читает простой .env (KEY=VALUE, комментарии, кавычки) и задаёт
// переменные только если они ещё не заданы — env/Docker/CI имеют приоритет.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // файла нет — не ошибка
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
