package main

import (
	"testing"
	"time"
)

func TestSubFileName(t *testing.T) {
	tim, err := time.Parse(TIME_FORMAT, "2020-01-02 09:35:00")
	if err != nil {
		t.FailNow()
	}
	assert(t, "008", subFileName(tim, "%%", "name", "jpg"), "%%")
	assert(t, "007", subFileName(tim, "%% %a %N", "name", "jpg"), "%% %a %N")

	assert(t, "006", subFileName(tim, "%YYYY+%MM+%DD+%h+%m+%s+%n.%x", "name", "jpg"), "2020+01+02+09+35+00+name.jpg")
	assert(t, "005", subFileName(tim, "%YYYY+%MM+%DD+%n.%x", "name", "jpg"), "2020+01+02+name.jpg")
	assert(t, "004", subFileName(tim, "%h+%m+%s+%n.%x", "name", "jpg"), "09+35+00+name.jpg")
	assert(t, "003", subFileName(tim, "%n.%x", "name", "jpg"), "name.jpg")
	assert(t, "002", subFileName(tim, "%n", "name", "jpg"), "name")
	assert(t, "001", subFileName(tim, "", "name", "jpg"), "")
}

func assert(t *testing.T, id, val, expected string) {
	if val == expected {
		return
	}
	t.Fatalf("Failed: id:%s expected:%s actual:%s", id, expected, val)
}
