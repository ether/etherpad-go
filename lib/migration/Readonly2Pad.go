package migration

type Readonly2Pad struct {
	ReadonlyId string
	PadId      string
}

type Pad2Readonly struct {
	PadId      string
	ReadonlyId string
}
