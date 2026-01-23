package io

import (
	"embed"
	"fmt"
	"strconv"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type ExportEtherpad struct {
	hooks         *hooks.Hook
	PadManager    *pad.Manager
	AuthorManager *author.Manager
	exportTxt     *ExportTxt
	exportPDF     *ExportPDF
	exportDocx    *ExportDocx
	exportOdt     *ExportOdt
	exportHtml    *ExportHtml
	logger        *zap.SugaredLogger
}

func NewExportEtherpad(hooks *hooks.Hook, padManager *pad.Manager, db db.DataStore, logger *zap.SugaredLogger, uiAssets embed.FS) *ExportEtherpad {
	exportTxt := ExportTxt{
		PadManager: padManager,
	}

	authorMgr := author.NewManager(db)

	exportEtherpad := &ExportEtherpad{
		hooks:         hooks,
		PadManager:    padManager,
		AuthorManager: authorMgr,
		exportTxt:     &exportTxt,
		exportDocx:    NewExportDocx(padManager, authorMgr, hooks),
		exportOdt:     NewExportOdt(padManager, authorMgr, hooks),
		exportHtml:    NewExportHtml(padManager, authorMgr, hooks),
		logger:        logger,
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

func (e *ExportEtherpad) GetPadRaw(padId string, readOnlyId *string) (*EtherpadExport, error) {
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

	export.Pad[dstPfx] = PadData{
		AText:          AText{Text: retrievedPad.AText.Text, Attribs: retrievedPad.AText.Attribs},
		Pool:           Pool{NumToAttrib: numToAttrib, NextNum: retrievedPad.Pool.NextNum},
		Head:           retrievedPad.Head,
		ChatHead:       retrievedPad.ChatHead,
		PublicStatus:   retrievedPad.PublicStatus,
		SavedRevisions: make([]any, 0),
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

	revisions, err := retrievedPad.GetRevisions(0, retrievedPad.Head)
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

		export.Revisions[key] = Revision{
			Changeset: rev.Changeset,
			Meta: RevisionMeta{
				Pool:      poolData,
				Author:    rev.AuthorId,
				Timestamp: &rev.Timestamp,
				AText: &AText{
					Text:    rev.AText.Text,
					Attribs: rev.AText.Attribs,
				},
			},
		}
	}
	return export, nil
}

func (e *ExportEtherpad) DoExport(ctx *fiber.Ctx, id string, readOnlyId *string, fileExportType string) error {
	fileName := id
	if readOnlyId != nil {
		fileName = *readOnlyId
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
		exportedPad, err := e.GetPadRaw(id, readOnlyId)
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
		return ctx.SendString(htmlContent)
	case "markdown", "md":
		ctx.Set("Content-Type", "text/markdown; charset=utf-8")
		markdownContent, err := e.exportHtml.GetPadMarkdownDocument(id, optRevNum, readOnlyId)
		if err != nil {
			e.logger.Warnf("Failed to get markdown document for id: %s with cause %s", id, err.Error())
			return ctx.Status(500).SendString(err.Error())
		}
		return ctx.SendString(markdownContent)
	default:
		return ctx.Status(400).SendString("Not Implemented")
	}
}
