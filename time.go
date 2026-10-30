package main

import "time"

// canonTimeLayout — фиксированная ширина (UTC, 9 знаков наносекунд), чтобы
// лексикографический порядок строк совпадал с хронологическим. Иначе
// "10:00:00Z" (без долей) лексикографически больше "10:00:00.5Z", хотя по
// времени раньше — и фильтры from/to, и ORDER BY по fetched_at дают неверный
// результат на границах секунд.
const canonTimeLayout = "2006-01-02T15:04:05.000000000Z"

func canonTime(t time.Time) string { return t.UTC().Format(canonTimeLayout) }

// normTime приводит RFC3339/RFC3339Nano-строку к каноническому виду;
// если строка не парсится, возвращает её как есть.
func normTime(s string) string {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return canonTime(t)
	}
	return s
}
