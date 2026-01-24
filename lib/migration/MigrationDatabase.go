package migration

type Database interface {
	// Pads
	GetNextPads(lastPadId string, limit int) ([]Pad, error)
	GetPadRevisions(padId string, lastRev int, limit int) ([]PadRevision, error)

	// Authors
	GetNextAuthors(lastAuthorId string, limit int) ([]Author, error)

	// Readonly mappings
	GetNextReadonly2Pad(lastReadonlyId string, limit int) ([]Readonly2Pad, error)
	GetNextPad2Readonly(lastPadId string, limit int) ([]Pad2Readonly, error)

	// Token to Author mappings
	GetNextToken2Author(lastToken string, limit int) ([]Token2Author, error)

	// Chat messages
	GetPadChatMessages(padId string, lastChatNum int, limit int) ([]ChatMessage, error)

	// Groups
	GetNextGroups(lastGroupId string, limit int) ([]Group, error)
	GetNextGroup2Sessions(lastGroupId string, limit int) ([]Group2Sessions, error)
	GetNextAuthor2Sessions(lastAuthorId string, limit int) ([]Author2Sessions, error)
	GetNextSessions(lastSessionId string, limit int) ([]Session, error)

	// Utility
	Close() error
}
