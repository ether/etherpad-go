package io

import (
	"path/filepath"
	"strings"

	"github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// ImportError represents an import error with a status code
type ImportError struct {
	Status  string
	Message string
}

func (e *ImportError) Error() string {
	if e.Message != "" {
		return e.Status + ": " + e.Message
	}
	return e.Status
}

// ImportResponse is the JSON response for import operations
type ImportResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    ImportData `json:"data"`
}

type ImportData struct {
	DirectDatabaseAccess bool `json:"directDatabaseAccess"`
}

// Known file extensions that can be imported
var knownFileEndings = []string{".txt", ".html", ".htm", ".etherpad", ".docx", ".doc", ".odt", ".rtf", ".pdf"}

// ImportHandler handles pad import operations
type ImportHandler struct {
	padManager      *pad.Manager
	securityManager *pad.SecurityManager
	padHandler      *ws.PadMessageHandler
	importer        *io.Importer
	settings        *settings.Settings
	logger          *zap.SugaredLogger
}

// NewImportHandler creates a new ImportHandler
func NewImportHandler(
	padManager *pad.Manager,
	securityManager *pad.SecurityManager,
	padHandler *ws.PadMessageHandler,
	importer *io.Importer,
	settings *settings.Settings,
	logger *zap.SugaredLogger,
) *ImportHandler {
	return &ImportHandler{
		padManager:      padManager,
		securityManager: securityManager,
		padHandler:      padHandler,
		importer:        importer,
		settings:        settings,
		logger:          logger,
	}
}

// ImportPad handles the import request
func (h *ImportHandler) ImportPad(ctx *fiber.Ctx) error {
	tokenCookie := ctx.Cookies("token")
	padId := ctx.Params("pad")

	// Check access
	grantedAccess, err := h.securityManager.CheckAccess(&padId, nil, &tokenCookie, nil)
	if err != nil {
		return ctx.Status(500).JSON(ImportResponse{
			Code:    2,
			Message: "internalError",
			Data:    ImportData{DirectDatabaseAccess: false},
		})
	}
	if grantedAccess.AccessStatus != "grant" {
		return ctx.Status(403).JSON(ImportResponse{
			Code:    1,
			Message: "accessDenied",
			Data:    ImportData{DirectDatabaseAccess: false},
		})
	}

	authorId := grantedAccess.AuthorId

	// Perform the import
	directDatabaseAccess, importErr := h.doImport(ctx, padId, authorId)

	if importErr != nil {
		h.logger.Warnf("Import failed: %v", importErr)
		return ctx.Status(400).JSON(ImportResponse{
			Code:    1,
			Message: importErr.Status,
			Data:    ImportData{DirectDatabaseAccess: false},
		})
	}

	return ctx.Status(200).JSON(ImportResponse{
		Code:    0,
		Message: "ok",
		Data:    ImportData{DirectDatabaseAccess: directDatabaseAccess},
	})
}

// doImport performs the actual import
func (h *ImportHandler) doImport(ctx *fiber.Ctx, padId string, authorId string) (bool, *ImportError) {
	// Get uploaded file
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		h.logger.Warn("Import failed: no file uploaded")
		return false, &ImportError{Status: "uploadFailed", Message: "no file uploaded"}
	}

	// Check file size
	if h.settings.ImportMaxFileSize > 0 && fileHeader.Size > int64(h.settings.ImportMaxFileSize) {
		h.logger.Warnf("Import failed: file too large (%d bytes)", fileHeader.Size)
		return false, &ImportError{Status: "maxFileSize"}
	}

	// Get file extension
	fileEnding := strings.ToLower(filepath.Ext(fileHeader.Filename))

	// Check if file extension is known
	fileEndingKnown := false
	for _, ending := range knownFileEndings {
		if fileEnding == ending {
			fileEndingKnown = true
			break
		}
	}

	if !fileEndingKnown {
		if h.settings.AllowUnknownFileEnds {
			// Treat unknown file as .txt
			fileEnding = ".txt"
		} else {
			h.logger.Warnf("Import failed: unknown file type %s", fileEnding)
			return false, &ImportError{Status: "uploadFailed", Message: "unknown file type"}
		}
	}

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		h.logger.Warnf("Import failed: could not open file: %v", err)
		return false, &ImportError{Status: "uploadFailed", Message: "could not open file"}
	}
	defer file.Close()

	// Read file content
	content := make([]byte, fileHeader.Size)
	_, err = file.Read(content)
	if err != nil {
		h.logger.Warnf("Import failed: could not read file: %v", err)
		return false, &ImportError{Status: "uploadFailed", Message: "could not read file"}
	}

	// Handle different file types
	switch fileEnding {
	case ".etherpad":
		return h.importEtherpad(padId, authorId, content)
	case ".html", ".htm":
		return h.importHTML(padId, authorId, string(content))
	case ".txt":
		return h.importText(padId, authorId, string(content))
	case ".docx", ".doc":
		return h.importDocx(padId, authorId, content)
	case ".odt":
		return h.importOdt(padId, authorId, content)
	case ".rtf":
		return h.importRtf(padId, authorId, content)
	case ".pdf":
		return h.importPdf(padId, authorId, content)
	default:
		return false, &ImportError{Status: "uploadFailed", Message: "unsupported file type"}
	}
}

// importEtherpad imports a .etherpad file (direct database access)
func (h *ImportHandler) importEtherpad(padId string, authorId string, content []byte) (bool, *ImportError) {
	// Check if pad already has content
	newText := "\n"
	retrievedPad, err := h.padManager.GetPad(padId, &newText, &authorId)
	if err != nil {
		h.logger.Warnf("Import failed: could not get pad: %v", err)
		return false, &ImportError{Status: "internalError", Message: "could not get pad"}
	}

	if retrievedPad.Head >= 10 {
		h.logger.Warn("Aborting direct database import attempt of a pad that already has content")
		return false, &ImportError{Status: "padHasData"}
	}

	// Parse the etherpad JSON
	if err := h.importer.SetPadRaw(padId, content, authorId); err != nil {
		h.logger.Warnf("Import failed: could not import etherpad: %v", err)
		return false, &ImportError{Status: "importFailed", Message: err.Error()}
	}

	// Direct database access - client should reload
	return true, nil
}

// importHTML imports an HTML file
func (h *ImportHandler) importHTML(padId string, authorId string, content string) (bool, *ImportError) {
	newText := "\n"
	retrievedPad, err := h.padManager.GetPad(padId, &newText, &authorId)
	if err != nil {
		h.logger.Warnf("Import failed: could not get pad: %v", err)
		return false, &ImportError{Status: "internalError", Message: "could not get pad"}
	}

	// Import HTML content
	if err := h.importer.SetPadHTML(retrievedPad, content, authorId); err != nil {
		h.logger.Warnf("Import failed: could not import HTML: %v", err)
		return false, &ImportError{Status: "importFailed", Message: err.Error()}
	}

	// Unload and reload pad to ensure fresh state
	h.padManager.UnloadPad(padId)
	retrievedPad, err = h.padManager.GetPad(padId, &newText, &authorId)
	if err != nil {
		h.logger.Warnf("Import failed: could not reload pad: %v", err)
		return false, &ImportError{Status: "internalError", Message: "could not reload pad"}
	}

	// Notify connected clients
	h.padHandler.UpdatePadClients(retrievedPad)

	return false, nil
}

// importText imports a plain text file
func (h *ImportHandler) importText(padId string, authorId string, content string) (bool, *ImportError) {
	// Check if content is ASCII (or valid UTF-8)
	if !isValidText(content) {
		h.logger.Warn("Import failed: file contains invalid characters")
		return false, &ImportError{Status: "uploadFailed", Message: "file contains invalid characters"}
	}

	newText := "\n"
	retrievedPad, err := h.padManager.GetPad(padId, &newText, &authorId)
	if err != nil {
		h.logger.Warnf("Import failed: could not get pad: %v", err)
		return false, &ImportError{Status: "internalError", Message: "could not get pad"}
	}

	// Set the text directly
	if err := retrievedPad.SetText(content, &authorId); err != nil {
		h.logger.Warnf("Import failed: could not set text: %v", err)
		return false, &ImportError{Status: "importFailed", Message: err.Error()}
	}

	// Unload and reload pad
	h.padManager.UnloadPad(padId)
	retrievedPad, err = h.padManager.GetPad(padId, &newText, &authorId)
	if err != nil {
		h.logger.Warnf("Import failed: could not reload pad: %v", err)
		return false, &ImportError{Status: "internalError", Message: "could not reload pad"}
	}

	// Notify connected clients
	h.padHandler.UpdatePadClients(retrievedPad)

	return false, nil
}

// importDocx imports a DOCX file
func (h *ImportHandler) importDocx(padId string, authorId string, content []byte) (bool, *ImportError) {
	text, err := h.importer.ExtractTextFromDocx(content)
	if err != nil {
		h.logger.Warnf("Import failed: could not extract text from DOCX: %v", err)
		return false, &ImportError{Status: "importFailed", Message: "could not read DOCX file"}
	}

	return h.importText(padId, authorId, text)
}

// importOdt imports an ODT file
func (h *ImportHandler) importOdt(padId string, authorId string, content []byte) (bool, *ImportError) {
	text, err := h.importer.ExtractTextFromOdt(content)
	if err != nil {
		h.logger.Warnf("Import failed: could not extract text from ODT: %v", err)
		return false, &ImportError{Status: "importFailed", Message: "could not read ODT file"}
	}

	return h.importText(padId, authorId, text)
}

// importRtf imports an RTF file
func (h *ImportHandler) importRtf(padId string, authorId string, content []byte) (bool, *ImportError) {
	text, err := h.importer.ExtractTextFromRtf(content)
	if err != nil {
		h.logger.Warnf("Import failed: could not extract text from RTF: %v", err)
		return false, &ImportError{Status: "importFailed", Message: "could not read RTF file"}
	}

	return h.importText(padId, authorId, text)
}

// importPdf imports a PDF file
func (h *ImportHandler) importPdf(padId string, authorId string, content []byte) (bool, *ImportError) {
	text, err := h.importer.ExtractTextFromPdf(content)
	if err != nil {
		h.logger.Warnf("Import failed: could not extract text from PDF: %v", err)
		return false, &ImportError{Status: "importFailed", Message: "could not read PDF file"}
	}

	return h.importText(padId, authorId, text)
}

// isValidText checks if the content is valid text (no binary/control characters except newlines/tabs)
func isValidText(content string) bool {
	for _, r := range content {
		// Allow printable characters, newlines, tabs, and carriage returns
		if r > 127 || (r < 32 && r != '\n' && r != '\t' && r != '\r') {
			// Allow high bytes for UTF-8
			if r > 240 {
				return false
			}
		}
	}
	return true
}

// Legacy function for backward compatibility
func ImportPad(ctx *fiber.Ctx, securityManager *pad.SecurityManager) error {
	tokenCookie := ctx.Cookies("token")
	padId := ctx.Params("pad")
	grantedAccess, err := securityManager.CheckAccess(&padId, nil, &tokenCookie, nil)
	if err != nil {
		return ctx.Status(500).JSON(ImportResponse{
			Code:    2,
			Message: "internalError",
			Data:    ImportData{DirectDatabaseAccess: false},
		})
	}
	if grantedAccess.AccessStatus != "grant" {
		return ctx.Status(403).JSON(ImportResponse{
			Code:    1,
			Message: "accessDenied",
			Data:    ImportData{DirectDatabaseAccess: false},
		})
	}

	// This legacy function doesn't have access to all required dependencies
	// It should be replaced with the ImportHandler
	return ctx.Status(501).JSON(ImportResponse{
		Code:    2,
		Message: "notImplemented",
		Data:    ImportData{DirectDatabaseAccess: false},
	})
}
