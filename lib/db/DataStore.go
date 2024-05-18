package db

type PadMethods interface {
	DoesPadExist(padID string) bool
	CreatePad(padID string) bool
	GetReadonlyPad(padId string) (string, error)
	CreatePad2ReadOnly(padId string, readonlyId string)
	CreateReadOnly2Pad(padId string, readonlyId string)
}

type DataStore interface {
	PadMethods
	GetReadOnly2Pad(id string) string
}
