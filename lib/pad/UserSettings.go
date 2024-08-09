package pad

type UserSettings struct {
	CanCreate         bool
	ReadOnly          bool
	PadAuthorizations *map[string]string
}
