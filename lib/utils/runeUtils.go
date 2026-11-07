package utils

import (
	"encoding/binary"
	"regexp"
	"strings"
	"unicode/utf8"
)

// RuneCount returns the same as utf8.RuneCountInString but checks eight
// ASCII bytes at a time, which is several times faster on mostly ASCII
// pad texts.
func RuneCount(s string) int {
	n := 0
	i := 0
	for i < len(s) {
		if i+8 <= len(s) && binary.LittleEndian.Uint64([]byte(s[i:i+8]))&0x8080808080808080 == 0 {
			i += 8
			n += 8
			continue
		}
		if s[i] < utf8.RuneSelf {
			i++
		} else {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
		}
		n++
	}
	return n
}

// RuneSlice returns the runes [start, end) of s. It works on byte offsets
// and returns a substring of s, so it does not allocate.
func RuneSlice(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	b0 := RuneByteOffset(s, start)
	b1 := b0 + RuneByteOffset(s[b0:], end-start)
	return s[b0:b1]
}

// RuneByteOffset returns the byte offset of the n-th rune in s, or len(s)
// if s has fewer runes.
func RuneByteOffset(s string, n int) int {
	i := 0
	for n > 0 && i < len(s) {
		// Skip runs of ASCII eight bytes at a time.
		for n >= 8 && i+8 <= len(s) && (s[i]|s[i+1]|s[i+2]|s[i+3]|s[i+4]|s[i+5]|s[i+6]|s[i+7])&0x80 == 0 {
			i += 8
			n -= 8
		}
		if n == 0 || i >= len(s) {
			break
		}
		if s[i] < utf8.RuneSelf {
			i++
		} else {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
		}
		n--
	}
	return i
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
