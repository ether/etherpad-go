package utils

type Session struct {
	Author   string
	PadId    string
	revision int
}

var SessionStore = make(map[string]Session)
