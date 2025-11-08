package utils

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}

func RuneSlice(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	r := []rune(s)
	if start > len(r) {
		start = len(r)
	}
	if end < start {
		end = start
	}
	if end > len(r) {
		end = len(r)
	}
	return string(r[start:end])
}

func RuneIndex(s, sep string) int {
	if sep == "" {
		return 0
	}
	byteIdx := strings.Index(s, sep)
	if byteIdx < 0 {
		return -1
	}
	return utf8.RuneCountInString(s[:byteIdx])
}

func RuneLastIndex(s, sep string) int {
	if sep == "" {
		return RuneCount(s)
	}
	byteIdx := strings.LastIndex(s, sep)
	if byteIdx < 0 {
		return -1
	}
	return utf8.RuneCountInString(s[:byteIdx])
}

func RuneIndexFromRegex(re *regexp.Regexp, s string) (int, int) {
	inds := re.FindStringIndex(s)
	if inds == nil {
		return -1, -1
	}
	startByte, endByte := inds[0], inds[1]
	startRune := utf8.RuneCountInString(s[:startByte])
	endRune := utf8.RuneCountInString(s[:endByte])
	return startRune, endRune
}
