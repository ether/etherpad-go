package io

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	dbmodels "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type ExportEtherpad struct {
	hooks          *hooks.Hook
	PadManager     *pad.Manager
	AuthorManager  *author.Manager
	exportTxt      *ExportTxt
	exportPDF      *ExportPDF
	exportDocx     *ExportDocx
	exportOdt      *ExportOdt
	exportHtml     *ExportHtml
	exportMarkdown *ExportMarkdown
	logger         *zap.SugaredLogger
}

func NewExportEtherpad(hooks *hooks.Hook, padManager *pad.Manager, db db.DataStore, logger *zap.SugaredLogger, uiAssets embed.FS) *ExportEtherpad {
	exportTxt := ExportTxt{
		PadManager: padManager,
	}

	authorMgr := author.NewManager(db)

	exportEtherpad := &ExportEtherpad{
		hooks:          hooks,
		PadManager:     padManager,
		AuthorManager:  authorMgr,
		exportTxt:      &exportTxt,
		exportDocx:     NewExportDocx(padManager, authorMgr, hooks),
		exportOdt:      NewExportOdt(padManager, authorMgr, hooks),
		exportHtml:     NewExportHtml(padManager, authorMgr, hooks),
		exportMarkdown: NewExportMarkdown(padManager, hooks),
		logger:         logger,
	}

	// Create exportPDF with reference back to exportEtherpad for embedding JSON data
	exportEtherpad.exportPDF = &ExportPDF{
		uiAssets:       uiAssets,
		exportTxt:      &exportTxt,
		exportEtherpad: exportEtherpad,
		padManager:     padManager,
		authorManager:  authorMgr,
		Hooks:          hooks,
	}

	return exportEtherpad
}

// GetPadRaw builds a .etherpad export. With revNum set (a /:rev/export
// request) the export is a point-in-time snapshot: head, text and revisions
// stop at that revision, and chat only contains messages posted no later than
// it (etherpad-lite #8389). Without it the whole pad is exported.
func (e *ExportEtherpad) GetPadRaw(padId string, readOnlyId *string, revNum *int) (*EtherpadExport, error) {
	var dstPfx string
	var padIdToUse string
	if readOnlyId != nil {
		dstPfx = "pad:" + *readOnlyId + ":"
		padIdToUse = *readOnlyId
	} else {
		dstPfx = "pad:" + padId + ":"
		padIdToUse = padId
	}
	var customPrefixes []string

	e.hooks.ExecuteHooks("exportEtherpadAdditionalContent", &customPrefixes)
	retrievedPad, err := e.PadManager.GetPad(padId, nil, nil)
	if err != nil {
		return nil, err
	}
	chatMessages, err := retrievedPad.GetChatMessages(0, retrievedPad.ChatHead)
	if err != nil {
		return nil, err
	}

	padHead := retrievedPad.Head
	effectiveHead := padHead
	if revNum != nil && *revNum >= 0 && *revNum < padHead {
		effectiveHead = *revNum
	}
	exportAText := AText{Text: retrievedPad.AText.Text, Attribs: retrievedPad.AText.Attribs}
	if effectiveHead < padHead {
		bound := retrievedPad.GetInternalRevisionAText(effectiveHead)
		if bound == nil {
			return nil, fmt.Errorf("could not reconstruct revision %d of pad %s", effectiveHead, padId)
		}
		exportAText = AText{Text: bound.Text, Attribs: bound.Attribs}
	}
	exportChatHead := retrievedPad.ChatHead
	if revNum != nil {
		// Any rev-specific export (rev == head included) only carries chat
		// posted no later than that revision. Chat times can be set through
		// the API, so filter rather than stop at the first later message, and
		// renumber the kept messages so the export stays contiguous.
		var revTime *int64
		if effectiveHead >= 0 {
			revTime, _ = retrievedPad.GetRevisionDate(effectiveHead)
		}
		kept := make([]dbmodels.ChatMessageDBWithDisplayName, 0, len(*chatMessages))
		for _, m := range *chatMessages {
			if revTime != nil && m.Time != nil && *m.Time <= *revTime {
				m.Head = len(kept)
				kept = append(kept, m)
			}
		}
		*chatMessages = kept
		exportChatHead = len(kept) - 1
	}

	export := &EtherpadExport{
		Pad:       make(map[string]PadData),
		Authors:   make(map[string]GlobalAuthor),
		Revisions: make(map[string]Revision),
		Chats:     make(map[string]ChatMessage),
	}
	var numToAttrib = make(map[string][]string)
	for i, v := range retrievedPad.Pool.NumToAttrib {
		numToAttrib[strconv.Itoa(i)] = []string{
			v.Key,
			v.Value,
		}
	}

	// The pad record itself is "pad:<id>" (no trailing colon), as in
	// etherpad-lite, so exports can be imported there and vice versa.
	export.Pad[strings.TrimSuffix(dstPfx, ":")] = PadData{
		AText:          exportAText,
		Pool:           Pool{NumToAttrib: numToAttrib, NextNum: retrievedPad.Pool.NextNum},
		Head:           effectiveHead,
		ChatHead:       exportChatHead,
		PublicStatus:   retrievedPad.PublicStatus,
		SavedRevisions: make([]any, 0),
	}
	if settingsJSON := retrievedPad.PadSettingsJSON(); settingsJSON != "" {
		record := export.Pad[strings.TrimSuffix(dstPfx, ":")]
		record.PadSettings = json.RawMessage(settingsJSON)
		export.Pad[strings.TrimSuffix(dstPfx, ":")] = record
	}

	authors := make(map[string]*author.Author)

	for _, authorId := range retrievedPad.GetAllAuthors() {
		retrievedAuthor, err := e.AuthorManager.GetAuthor(authorId)
		if err != nil {
			return nil, err
		}
		authors[authorId] = retrievedAuthor
		export.Authors["globalAuthor:"+authorId] = GlobalAuthor{
			ColorId:   retrievedAuthor.ColorId,
			Name:      retrievedAuthor.Name,
			PadIDs:    padIdToUse,
			Timestamp: retrievedAuthor.Timestamp,
		}
	}

	for _, chatMessage := range *chatMessages {
		authorOfChat := authors[*chatMessage.ChatMessageDB.AuthorId]
		export.Chats[fmt.Sprintf("%schat:%d", dstPfx, chatMessage.Head)] = ChatMessage{
			Text:     chatMessage.ChatMessageDB.Message,
			Time:     chatMessage.Time,
			UserId:   chatMessage.ChatMessageDB.AuthorId,
			UserName: authorOfChat.Name,
		}
	}

	revisions, err := retrievedPad.GetRevisions(0, effectiveHead)
	if err != nil {
		return nil, err
	}

	for _, rev := range *revisions {
		key := fmt.Sprintf("%srevs:%d", dstPfx, rev.RevNum)

		var poolData *PoolWithAttribToNum
		if rev.Pool != nil {
			poolData = &PoolWithAttribToNum{
				NextNum:     rev.Pool.NextNum,
				AttribToNum: rev.Pool.AttribToNum,
				NumToAttrib: rev.Pool.NumToAttrib,
			}
		} else {
			poolData = &PoolWithAttribToNum{
				NextNum:     0,
				AttribToNum: make(map[string]int),
				NumToAttrib: make(map[string][]string),
			}
		}

		// Like etherpad-lite, only key revisions carry the full text.
		var revAText *AText
		if rev.AText.Text != "" {
			revAText = &AText{Text: rev.AText.Text, Attribs: rev.AText.Attribs}
		}
		export.Revisions[key] = Revision{
			Changeset: rev.Changeset,
			Meta: RevisionMeta{
				Pool:      poolData,
				Author:    rev.AuthorId,
				Timestamp: &rev.Timestamp,
				AText:     revAText,
			},
		}
	}
	return export, nil
}

func (e *ExportEtherpad) DoExport(ctx fiber.Ctx, id string, readOnlyId *string, fileExportType string) error {
	fileName := id
	if readOnlyId != nil {
		fileName = *readOnlyId
	}

	// exportFileName hook: plugins may override the download filename.
	readOnlyIdStr := ""
	if readOnlyId != nil {
		readOnlyIdStr = *readOnlyId
	}
	fileNameCtx := &events.ExportFileNameContext{
		PadId:      id,
		ReadOnlyId: readOnlyIdStr,
		ExportType: fileExportType,
	}
	e.hooks.ExecuteExportFileNameHooks(fileNameCtx)
	if fileNameCtx.FileName() != "" {
		fileName = fileNameCtx.FileName()
	}

	ctx.Attachment(fileName + "." + fileExportType)
	optRev := ctx.Params("rev")
	var optRevNum *int = nil
	if optRev != "" {
		actualRev, err := utils.CheckValidRev(optRev)
		if err != nil {
			return ctx.Status(400).SendString(err.Error())
		}
		optRevNum = actualRev

	}

	switch fileExportType {
	case "etherpad":
		exportedPad, err := e.GetPadRaw(id, readOnlyId, optRevNum)
		if err != nil {
			return ctx.Status(500).SendString(err.Error())
		}
		marshalledPad, err := exportedPad.MarshalJSON()
		if err != nil {
			return ctx.Status(500).SendString(err.Error())
		}
		return ctx.Send(marshalledPad)
	case "txt":
		textString, err := e.exportTxt.GetPadTxtDocument(id, optRevNum)
		if err != nil {
			e.logger.Warnf("Failed to get txt document for id: %s with cause %s", id, err.Error())
			return ctx.Status(500).SendString(err.Error())
		}
		return ctx.SendString(*textString)
	case "pdf":
		ctx.Set("Content-Type", "application/pdf")
		pdfBytes, err := e.exportPDF.GetPadPdfDocument(id, optRevNum)
		if err != nil {
			e.logger.Warnf("Failed to get pdf document for id: %s with cause %s", id, err.Error())
			return ctx.Status(500).SendString(err.Error())
		}
		return ctx.Send(pdfBytes)
	case "doc", "docx", "word":
		ctx.Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		docxBytes, err := e.exportDocx.GetPadDocxDocument(id, optRevNum)
		if err != nil {
			e.logger.Warnf("Failed to get docx document for id: %s with cause %s", id, err.Error())
			return ctx.Status(500).SendString(err.Error())
		}
		return ctx.Send(docxBytes)
	case "odt", "open":
		ctx.Set("Content-Type", "application/vnd.oasis.opendocument.text")
		odtBytes, err := e.exportOdt.GetPadOdtDocument(id, optRevNum)
		if err != nil {
			e.logger.Warnf("Failed to get odt document for id: %s with cause %s", id, err.Error())
			return ctx.Status(500).SendString(err.Error())
		}
		return ctx.Send(odtBytes)
	case "html":
		ctx.Set("Content-Type", "text/html; charset=utf-8")
		htmlContent, err := e.exportHtml.GetPadHTMLDocument(id, optRevNum, readOnlyId)
		if err != nil {
			e.logger.Warnf("Failed to get html document for id: %s with cause %s", id, err.Error())
			return ctx.Status(500).SendString(err.Error())
		}
		// exportHTMLSend hook: plugins may replace the full HTML document.
		sendCtx := &events.ExportHTMLSendContext{PadId: id, HTML: &htmlContent}
		e.hooks.ExecuteExportHTMLSendHooks(sendCtx)
		return ctx.SendString(htmlContent)
	case "markdown", "md":
		ctx.Set("Content-Type", "text/markdown; charset=utf-8")
		markdownContent, err := e.exportMarkdown.GetPadMarkdownDocument(id, optRevNum)
		if err != nil {
			e.logger.Warnf("Failed to get markdown document for id: %s with cause %s", id, err.Error())
			return ctx.Status(500).SendString(err.Error())
		}
		return ctx.SendString(*markdownContent)
	default:
		return ctx.Status(400).SendString("Not Implemented")
	}
}
