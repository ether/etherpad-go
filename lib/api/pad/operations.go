package pad

import (
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib"
	errors2 "github.com/ether/etherpad-go/lib/api/errors"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
)

// RestoreRevisionRequest represents the request to restore a revision
type RestoreRevisionRequest struct {
	Rev      int    `json:"rev"`
	AuthorId string `json:"authorId"`
}

// CopyPadRequest represents the request to copy a pad
type CopyPadRequest struct {
	DestinationID string `json:"destinationID"`
	Force         bool   `json:"force"`
}

// CopyPadWithoutHistoryRequest represents the request to copy a pad without history
type CopyPadWithoutHistoryRequest struct {
	DestinationID string `json:"destinationID"`
	Force         bool   `json:"force"`
	AuthorId      string `json:"authorId"`
}

// MovePadRequest represents the request to move a pad
type MovePadRequest struct {
	DestinationID string `json:"destinationID"`
	Force         bool   `json:"force"`
}

// ReadOnlyIDResponse represents the response with a read-only ID
type ReadOnlyIDResponse struct {
	ReadOnlyID string `json:"readOnlyID"`
}

// PadIDResponse represents the response with a pad ID
type PadIDResponse struct {
	PadID string `json:"padID"`
}

// PublicStatusRequest represents the request to set public status
type PublicStatusRequest struct {
	PublicStatus bool `json:"publicStatus"`
}

// PublicStatusResponse represents the response with public status
type PublicStatusResponse struct {
	PublicStatus bool `json:"publicStatus"`
}

// AuthorsResponse represents the response with author IDs
type AuthorsResponse struct {
	AuthorIDs []string `json:"authorIDs"`
}

// SendMessageRequest represents the request to send a message to clients
type SendMessageRequest struct {
	Msg string `json:"msg"`
}

// ChatHeadResponse represents the response with chat head
type ChatHeadResponse struct {
	ChatHead int `json:"chatHead"`
}

// DiffHTMLRequest represents the request for diff HTML
type DiffHTMLRequest struct {
	StartRev int `json:"startRev"`
	EndRev   int `json:"endRev"`
}

// DiffHTMLResponse represents the response with diff HTML
type DiffHTMLResponse struct {
	HTML    string   `json:"html"`
	Authors []string `json:"authors"`
}

// RestoreRevision godoc
// @Summary Restore a revision
// @Description Restores a revision from the past as a new changeset
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body RestoreRevisionRequest true "Revision and Author ID"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/restoreRevision [post]
func RestoreRevision(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request RestoreRevisionRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Validate revision
		if request.Rev > pad.Head {
			return c.Status(400).JSON(errors2.RevisionHigherThanHeadError)
		}

		// Get the atext at the target revision
		atext := pad.GetInternalRevisionAText(request.Rev)
		if atext == nil {
			return c.Status(500).JSON(errors2.InternalApiError)
		}

		oldText := pad.Text()
		atextText := atext.Text + "\n"

		// Create a new changeset with a helper builder object
		builder := changeset.NewBuilder(len(oldText))

		// Iterate over attribute runs
		textIndex := 0
		newTextStart := 0
		newTextEnd := len(atextText)
		ops, err := changeset.DeserializeOps(atext.Attribs)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalApiError)
		}

		for _, op := range *ops {
			nextIndex := textIndex + op.Chars
			if !(nextIndex <= newTextStart || textIndex >= newTextEnd) {
				start := max(newTextStart, textIndex)
				end := min(newTextEnd, nextIndex)
				builder.Insert(atextText[start:end], changeset.KeepArgs{}, nil)
			}
			textIndex = nextIndex
		}

		// Remove old text
		lastNewlinePos := strings.LastIndex(oldText, "\n")
		if lastNewlinePos < 0 {
			builder.Remove(len(oldText)-1, 0)
		} else {
			newlineCount := strings.Count(oldText, "\n")
			builder.Remove(lastNewlinePos, newlineCount-1)
			builder.Remove(len(oldText)-lastNewlinePos-1, 0)
		}

		cs := builder.ToString()

		// Append the revision
		authorId := request.AuthorId
		_, err = pad.AppendRevision(cs, &authorId)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		// Update clients
		initStore.Handler.UpdatePadClients(pad)

		return c.SendStatus(200)
	}
}

// GetReadOnlyID godoc
// @Summary Get read-only ID
// @Description Returns the read-only link ID of a pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} ReadOnlyIDResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/readOnlyID [get]
func GetReadOnlyID(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Verify pad exists
		_, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Get the read-only ID
		readOnlyID := initStore.ReadOnlyManager.GetReadOnlyId(padId)

		return c.JSON(ReadOnlyIDResponse{
			ReadOnlyID: readOnlyID,
		})
	}
}

// GetPadID godoc
// @Summary Get pad ID from read-only ID
// @Description Returns the pad ID based on the read-only ID
// @Tags Pads
// @Accept json
// @Produce json
// @Param roId path string true "Read-only ID"
// @Success 200 {object} PadIDResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/readonly/{roId} [get]
func GetPadID(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		roId := c.Params("roId")

		// Get the pad ID
		padID, err := initStore.ReadOnlyManager.GetPadId(roId)
		if err != nil || padID == nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		return c.JSON(PadIDResponse{
			PadID: *padID,
		})
	}
}

// ListAuthorsOfPad godoc
// @Summary List authors of a pad
// @Description Returns an array of author IDs who contributed to this pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} AuthorsResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/authors [get]
func ListAuthorsOfPad(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Get all authors from the pool
		authorIDs := make([]string, 0)
		pad.Pool.EachAttrib(func(attr apool.Attribute) {
			if attr.Key == "author" && attr.Value != "" {
				// Check if not already in list
				found := false
				for _, id := range authorIDs {
					if id == attr.Value {
						found = true
						break
					}
				}
				if !found {
					authorIDs = append(authorIDs, attr.Value)
				}
			}
		})

		return c.JSON(AuthorsResponse{
			AuthorIDs: authorIDs,
		})
	}
}

// GetChatHead godoc
// @Summary Get chat head
// @Description Returns the chat head (last number of the last chat message) of the pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} ChatHeadResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/chatHead [get]
func GetChatHead(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		return c.JSON(ChatHeadResponse{
			ChatHead: pad.ChatHead,
		})
	}
}

// GetRevisionsCount godoc
// @Summary Get revisions count
// @Description Returns the number of revisions of this pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} map[string]int
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/revisionsCount [get]
func GetRevisionsCount(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		return c.JSON(fiber.Map{
			"revisions": pad.Head,
		})
	}
}

// GetLastEdited godoc
// @Summary Get last edited timestamp
// @Description Returns the timestamp of when the pad was last edited
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} map[string]int64
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/lastEdited [get]
func GetLastEdited(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		var lastEdited int64 = 0
		if pad.UpdatedAt != nil {
			lastEdited = pad.UpdatedAt.UnixMilli()
		}

		return c.JSON(fiber.Map{
			"lastEdited": lastEdited,
		})
	}
}

// DeletePad godoc
// @Summary Delete a pad
// @Description Deletes a pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {string} string "OK"
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId} [delete]
func DeletePad(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Verify pad exists
		_, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Delete the pad using PadManager
		err = initStore.PadManager.RemovePad(padId)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.SendStatus(200)
	}
}

// GetHTML godoc
// @Summary Get pad HTML
// @Description Returns the pad content as HTML
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param rev query string false "Revision number"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/html [get]
func GetHTML(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		revStr := c.Query("rev")
		var rev *int
		if revStr != "" {
			revNum, err := utils.CheckValidRev(revStr)
			if err != nil {
				return c.Status(400).JSON(errors2.InvalidRevisionError)
			}
			if *revNum > pad.Head {
				return c.Status(400).JSON(errors2.RevisionHigherThanHeadError)
			}
			rev = revNum
		}

		// Get the HTML (using exporter if available)
		var text string
		if rev != nil {
			atext := pad.GetInternalRevisionAText(*rev)
			if atext == nil {
				return c.Status(500).JSON(errors2.InternalApiError)
			}
			text = atext.Text
		} else {
			text = pad.Text()
		}

		// Simple HTML conversion (basic)
		html := "<html><body>" + strings.ReplaceAll(text, "\n", "<br>") + "</body></html>"

		return c.JSON(fiber.Map{
			"html": html,
		})
	}
}

// CheckToken godoc
// @Summary Check API token
// @Description Returns ok when the current API token is valid
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {string} string "OK"
// @Security BearerAuth
// @Router /admin/api/checkToken [get]
func CheckToken() fiber.Handler {
	return func(c fiber.Ctx) error {
		// If we reach here, the token is valid (middleware already validated it)
		return c.SendStatus(200)
	}
}

// SetHTMLRequest represents the request to set HTML content
type SetHTMLRequest struct {
	HTML     string `json:"html"`
	AuthorId string `json:"authorId"`
}

// SetHTML godoc
// @Summary Set pad HTML
// @Description Sets the text of a pad based on HTML
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body SetHTMLRequest true "HTML content and Author ID"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/html [post]
func SetHTML(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request SetHTMLRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		if request.HTML == "" {
			return c.Status(400).JSON(errors2.NewInvalidParamError("html is required"))
		}

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Import HTML using the importer
		if err := initStore.Importer.SetPadHTML(pad, request.HTML, request.AuthorId); err != nil {
			return c.Status(400).JSON(errors2.NewInvalidParamError("HTML is malformed"))
		}

		// Update clients
		initStore.Handler.UpdatePadClients(pad)

		return c.SendStatus(200)
	}
}

// ChatMessageResponse represents a chat message in the response
type ChatMessageResponse struct {
	Text     string `json:"text"`
	AuthorID string `json:"authorID"`
	Time     int64  `json:"time"`
	UserName string `json:"userName"`
}

// ChatHistoryResponse represents the response with chat history
type ChatHistoryResponse struct {
	Messages []ChatMessageResponse `json:"messages"`
}

// GetChatHistory godoc
// @Summary Get chat history
// @Description Returns a part of or the whole chat history of this pad
// @Tags Chat
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param start query int false "Start index"
// @Param end query int false "End index"
// @Success 200 {object} ChatHistoryResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/chatHistory [get]
func GetChatHistory(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Parse start and end parameters
		startStr := c.Query("start")
		endStr := c.Query("end")

		var start, end int
		chatHead := pad.ChatHead

		if startStr != "" && endStr != "" {
			startNum, err := utils.CheckValidRev(startStr)
			if err != nil || *startNum < 0 {
				return c.Status(400).JSON(errors2.NewInvalidParamError("start must be a non-negative number"))
			}
			start = *startNum

			endNum, err := utils.CheckValidRev(endStr)
			if err != nil || *endNum < 0 {
				return c.Status(400).JSON(errors2.NewInvalidParamError("end must be a non-negative number"))
			}
			end = *endNum

			if start > end {
				return c.Status(400).JSON(errors2.NewInvalidParamError("start is higher than end"))
			}
			if start > chatHead {
				return c.Status(400).JSON(errors2.NewInvalidParamError("start is higher than the current chatHead"))
			}
			if end > chatHead {
				return c.Status(400).JSON(errors2.NewInvalidParamError("end is higher than the current chatHead"))
			}
		} else {
			start = 0
			end = chatHead
		}

		// Get chat messages
		messages, err := pad.GetChatMessages(start, end)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		// Convert to response format
		responseMessages := make([]ChatMessageResponse, 0)
		if messages != nil {
			for _, msg := range *messages {
				userName := ""
				if msg.DisplayName != nil {
					userName = *msg.DisplayName
				}
				authorID := ""
				if msg.AuthorId != nil {
					authorID = *msg.AuthorId
				}
				var timestamp int64 = 0
				if msg.Time != nil {
					timestamp = *msg.Time
				}
				responseMessages = append(responseMessages, ChatMessageResponse{
					Text:     msg.Message,
					AuthorID: authorID,
					Time:     timestamp,
					UserName: userName,
				})
			}
		}

		return c.JSON(ChatHistoryResponse{
			Messages: responseMessages,
		})
	}
}

// AppendChatMessageRequest represents the request to append a chat message
type AppendChatMessageRequest struct {
	Text     string `json:"text"`
	AuthorID string `json:"authorID"`
	Time     int64  `json:"time"`
}

// AppendChatMessage godoc
// @Summary Append a chat message
// @Description Creates a chat message for the pad
// @Tags Chat
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body AppendChatMessageRequest true "Chat message data"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/chat [post]
func AppendChatMessage(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request AppendChatMessageRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		if request.Text == "" {
			return c.Status(400).JSON(errors2.NewInvalidParamError("text is required"))
		}

		// Use current timestamp if not provided
		if request.Time == 0 {
			request.Time = time.Now().UnixMilli()
		}

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Append chat message
		_, err = pad.AppendChatMessage(&request.AuthorID, request.Time, request.Text)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.SendStatus(200)
	}
}

// SavedRevisionsCountResponse represents the response with saved revisions count
type SavedRevisionsCountResponse struct {
	SavedRevisions int `json:"savedRevisions"`
}

// GetSavedRevisionsCount godoc
// @Summary Get saved revisions count
// @Description Returns the number of saved revisions of this pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} SavedRevisionsCountResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/savedRevisionsCount [get]
func GetSavedRevisionsCount(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		return c.JSON(SavedRevisionsCountResponse{
			SavedRevisions: len(pad.SavedRevisions),
		})
	}
}

// SavedRevisionsListResponse represents the response with saved revisions list
type SavedRevisionsListResponse struct {
	SavedRevisions []int `json:"savedRevisions"`
}

// ListSavedRevisions godoc
// @Summary List saved revisions
// @Description Returns the list of saved revisions of this pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} SavedRevisionsListResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/savedRevisions [get]
func ListSavedRevisions(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Get saved revisions list
		savedRevisions := make([]int, len(pad.SavedRevisions))
		for i, rev := range pad.SavedRevisions {
			savedRevisions[i] = rev.RevNum
		}

		return c.JSON(SavedRevisionsListResponse{
			SavedRevisions: savedRevisions,
		})
	}
}

// SaveRevisionRequest represents the request to save a revision
type SaveRevisionRequest struct {
	Rev int `json:"rev"`
}

// SaveRevision godoc
// @Summary Save a revision
// @Description Saves the current revision of the pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body SaveRevisionRequest false "Revision number (optional, defaults to head)"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/saveRevision [post]
func SaveRevision(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request SaveRevisionRequest
		// Body is optional
		c.Bind().Body(&request)

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Validate revision if provided
		if request.Rev > pad.Head {
			return c.Status(400).JSON(errors2.RevisionHigherThanHeadError)
		}

		// Create author for API call
		apiAuthor, err := initStore.AuthorManager.CreateAuthor(nil)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		// Save the revision
		if err := pad.AddSavedRevision(apiAuthor.Id); err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.SendStatus(200)
	}
}

// CreatePadRequest represents the request to create a pad
type CreatePadRequest struct {
	Text     string `json:"text"`
	AuthorId string `json:"authorId"`
}

// CreatePad godoc
// @Summary Create a new pad
// @Description Creates a new pad with optional initial text
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body CreatePadRequest false "Initial text and author ID"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 409 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId} [post]
func CreatePad(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request CreatePadRequest
		// Body is optional
		c.Bind().Body(&request)

		// Check for invalid characters
		if strings.Contains(padId, "$") {
			return c.Status(400).JSON(errors2.NewInvalidParamError("createPad can't create group pads"))
		}
		if strings.ContainsAny(padId, "/?&#") {
			return c.Status(400).JSON(errors2.NewInvalidParamError("malformed padID: Remove special characters"))
		}

		// Check if pad already exists
		exists, err := initStore.PadManager.DoesPadExist(padId)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}
		if exists != nil && *exists {
			return c.Status(409).JSON(errors2.NewInvalidParamError("pad already exists"))
		}

		// Create the pad
		var textPtr *string
		if request.Text != "" {
			textPtr = &request.Text
		}
		var authorPtr *string
		if request.AuthorId != "" {
			authorPtr = &request.AuthorId
		}

		_, err = initStore.PadManager.GetPad(padId, textPtr, authorPtr)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.SendStatus(200)
	}
}

// AppendTextRequest represents the request to append text
type AppendTextRequest struct {
	Text     string `json:"text"`
	AuthorId string `json:"authorId"`
}

// AppendText godoc
// @Summary Append text to a pad
// @Description Appends text to the end of a pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body AppendTextRequest true "Text to append and Author ID"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/appendText [post]
func AppendText(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request AppendTextRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		if request.Text == "" {
			return c.Status(400).JSON(errors2.NewInvalidParamError("text is required"))
		}

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Get current text and append
		currentText := pad.Text()
		// Remove trailing newline, append new text, add newline back
		if len(currentText) > 0 && currentText[len(currentText)-1] == '\n' {
			currentText = currentText[:len(currentText)-1]
		}
		newText := currentText + request.Text

		var authorId *string
		if request.AuthorId != "" {
			authorId = &request.AuthorId
		}

		err = pad.SetText(newText, authorId)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		// Update clients
		initStore.Handler.UpdatePadClients(pad)

		return c.SendStatus(200)
	}
}

// ChangesetResponse represents the response with a changeset
type ChangesetResponse struct {
	Changeset string `json:"changeset"`
}

// GetRevisionChangesetOptional godoc
// @Summary Get revision changeset (optional rev)
// @Description Returns the changeset at a given revision, or last revision if rev is not provided
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param rev query string false "Revision number (optional, defaults to head)"
// @Success 200 {object} ChangesetResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/revisionChangeset [get]
func GetRevisionChangesetOptional(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		head := pad.Head
		var revNum int

		// Check if rev query param is provided
		revStr := c.Query("rev")
		if revStr != "" {
			revPtr, err := utils.CheckValidRev(revStr)
			if err != nil {
				return c.Status(400).JSON(errors2.InvalidRevisionError)
			}
			if *revPtr > head {
				return c.Status(400).JSON(errors2.RevisionHigherThanHeadError)
			}
			revNum = *revPtr
		} else {
			revNum = head
		}

		// Get the changeset
		revision, err := pad.GetRevision(revNum)
		if err != nil {
			return c.Status(404).JSON(errors2.RevisionNotFoundError)
		}

		return c.JSON(ChangesetResponse{
			Changeset: revision.Changeset,
		})
	}
}

// AllPadsResponse represents the response with all pad IDs
type AllPadsResponse struct {
	PadIDs []string `json:"padIDs"`
}

// ListAllPads godoc
// @Summary List all pads
// @Description Returns a list of all pad IDs
// @Tags Pads
// @Accept json
// @Produce json
// @Success 200 {object} AllPadsResponse
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads [get]
func ListAllPads(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		pads, err := initStore.Store.GetPadIds()
		if err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}
		if pads == nil {
			pads = &[]string{}
		}
		return c.JSON(AllPadsResponse{
			PadIDs: *pads,
		})
	}
}

// PadUser represents a user currently in a pad
type PadUser struct {
	ID      string `json:"id"`
	ColorID string `json:"colorId"`
	Name    string `json:"name"`
}

// PadUsersResponse represents the response with pad users
type PadUsersResponse struct {
	PadUsers []PadUser `json:"padUsers"`
}

// PadUsersCountResponse represents the response with pad users count
type PadUsersCountResponse struct {
	PadUsersCount int `json:"padUsersCount"`
}

// GetPadUsers godoc
// @Summary Get users currently in a pad
// @Description Returns a list of users currently editing a pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} PadUsersResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/users [get]
func GetPadUsers(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Verify pad exists
		_, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Get users from session store
		users := initStore.Handler.GetPadUsers(padId)

		padUsers := make([]PadUser, 0)
		for _, user := range users {
			// Get author info for color and name
			authorInfo, err := initStore.AuthorManager.GetAuthor(user.AuthorId)
			colorId := ""
			name := ""
			if err == nil && authorInfo != nil {
				colorId = authorInfo.ColorId
				if authorInfo.Name != nil {
					name = *authorInfo.Name
				}
			}
			padUsers = append(padUsers, PadUser{
				ID:      user.AuthorId,
				ColorID: colorId,
				Name:    name,
			})
		}

		return c.JSON(PadUsersResponse{
			PadUsers: padUsers,
		})
	}
}

// GetPadUsersCount godoc
// @Summary Get count of users in a pad
// @Description Returns the number of users currently editing a pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} PadUsersCountResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/usersCount [get]
func GetPadUsersCount(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Verify pad exists
		_, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Get users count from session store
		count := initStore.Handler.GetPadUsersCount(padId)

		return c.JSON(PadUsersCountResponse{
			PadUsersCount: count,
		})
	}
}
