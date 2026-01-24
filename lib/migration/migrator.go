package migration

import (
	"fmt"

	"github.com/ether/etherpad-go/lib/db"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/utils"
	"go.uber.org/zap"
)

type Migrator struct {
	oldEtherpadDB *SQLDatabase
	newDataStore  db.DataStore
	logger        *zap.SugaredLogger
}

func NewMigrator(oldEtherpadDB *SQLDatabase, newDataStore db.DataStore, logger *zap.SugaredLogger) *Migrator {
	return &Migrator{
		logger:        logger,
		oldEtherpadDB: oldEtherpadDB,
		newDataStore:  newDataStore,
	}
}

func (m *Migrator) MigrateAuthors() error {
	m.logger.Info("Starting migration of authors...")
	lastAuthorId := ""
	for {
		authors, err := m.oldEtherpadDB.GetNextAuthors(lastAuthorId, 100)
		if err != nil {
			return fmt.Errorf("failed to get authors: %v", err)
		}
		if len(authors) == 0 {
			break
		}

		for _, author := range authors {
			m.logger.Debug("Author: %s (%s)\n", author.Id, author.Name)
			if err := m.newDataStore.SaveAuthor(db2.AuthorDB{
				ID:        author.Id,
				ColorId:   utils.ColorPalette[author.ColorId],
				Name:      &author.Name,
				Timestamp: author.Timestamp,
			}); err != nil {
				return fmt.Errorf("failed to save author %s: %v", author.Id, err)
			}
			lastAuthorId = author.Id
		}
	}
	m.logger.Info("Finished migration of authors.")
	return nil
}

func (m *Migrator) MigratePads() error {
	m.logger.Info("Starting migration of pads...")
	lastPadId := ""
	for {
		pads, err := m.oldEtherpadDB.GetNextPads(lastPadId, 100)
		if err != nil {
			return fmt.Errorf("failed to get authors: %v", err)
		}
		if len(pads) == 0 {
			break
		}

		for _, pad := range pads {
			savedRevisions := make(map[int]db2.SavedRevision)
			for _, savedRev := range pad.SavedRevisions {
				var labelForDB *string
				if savedRev.Label != "" {
					labelForDB = &savedRev.Label
				}
				savedRevisions[savedRev.RevNum] = db2.SavedRevision{
					RevNum:    savedRev.RevNum,
					SavedBy:   savedRev.SavedById,
					Timestamp: savedRev.Timestamp,
					Label:     labelForDB,
					Id:        savedRev.Id,
				}
			}

			m.logger.Debug("Pad: %s (%s)\n", pad.PadId)
			if err := m.newDataStore.CreatePad(pad.PadId, db2.PadDB{
				RevNum:         pad.Head,
				ChatHead:       pad.ChatHead,
				ReadOnlyId:     "",
				AText:          pad.AText,
				Pool:           pad.Pool,
				PublicStatus:   pad.PublicStatus,
				SavedRevisions: savedRevisions,
			}); err != nil {
				return fmt.Errorf("failed to save author %s: %v", pad.PadId, err)
			}
			lastPadId = pad.PadId
		}
	}
	m.logger.Info("Finished migration of pads.")
	return nil
}

func (m *Migrator) MigrateRevisions() error {
	m.logger.Info("Starting migration of revisions...")
	lastMigrationNum := -1
	lastPadId := ""
	for {
		pads, err := m.oldEtherpadDB.GetNextPads(lastPadId, 10)
		if err != nil {
			return fmt.Errorf("failed to get pads for revision migration: %v", err)
		}
		if len(pads) == 0 {
			break
		}

		for _, pad := range pads {
			lastPadId = pad.PadId
			lastMigrationNum = -1
			m.logger.Debug("Migrating revisions for pad: %s\n", pad.PadId)
			padRevisions, err := m.oldEtherpadDB.GetPadRevisions(lastPadId, lastMigrationNum, 100)
			if err != nil {
				return fmt.Errorf("failed to get pad revisions: %v", err)
			}
			if len(padRevisions) == 0 {
				break
			}

			for _, rev := range padRevisions {
				m.logger.Debug("Migrating revision %d for pad %s\n", rev.RevNum, pad.PadId)
				atext := db2.AText{
					Text:    rev.Meta.Atext.Text,
					Attribs: rev.Meta.Atext.Attribs,
				}
				revPool := db2.RevPool{
					NumToAttrib: rev.Meta.Pool.NumToAttrib,
					AttribToNum: rev.Meta.Pool.AttribToNum,
					NextNum:     rev.Meta.Pool.NextNum,
				}

				if err := m.newDataStore.SaveRevision(pad.PadId, rev.RevNum, rev.Changeset, atext, revPool, &rev.Meta.Author, rev.Meta.Timestamp); err != nil {
					return fmt.Errorf("failed to save revision %d for pad %s: %v", rev.RevNum, pad.PadId, err)
				}
				lastMigrationNum = rev.RevNum
			}
			lastPadId = pad.PadId
		}
	}
	m.logger.Info("Finished migration of revisions.")
	return nil
}
