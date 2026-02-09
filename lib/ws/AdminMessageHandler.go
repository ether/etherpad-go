package ws

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	adminutils "github.com/ether/etherpad-go/admin/src/utils"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/models/revision"
	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/settings"
	libutils "github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type AdminMessageHandler struct {
	store             db.DataStore
	hub               *Hub
	hook              *hooks.Hook
	padManager        *pad.Manager
	padMessageHandler *PadMessageHandler
	Logger            *zap.SugaredLogger
	App               *fiber.App
}

func NewAdminMessageHandler(store db.DataStore, h *hooks.Hook, m *pad.Manager, padMessHandler *PadMessageHandler, logger *zap.SugaredLogger, hub *Hub, app *fiber.App) AdminMessageHandler {
	return AdminMessageHandler{
		store:             store,
		hook:              h,
		padManager:        m,
		padMessageHandler: padMessHandler,
		Logger:            logger,
		hub:               hub,
		App:               app,
	}
}

func (h AdminMessageHandler) HandleMessage(message admin.EventMessage, retrievedSettings *settings.Settings, c *Client) {
	switch message.Event {
	case "load":
		{
			resp := make([]interface{}, 2)
			resp[0] = "settings"
			resp[1] = map[string]interface{}{
				"results": retrievedSettings,
			}
			responseBytes, err := json.Marshal(resp)
			if err != nil {
				return
			}
			c.SafeSend(responseBytes)
		}
	case "checkUpdates":
		{
			latestVersion, err := h.store.GetServerVersion()
			if err != nil {
				h.Logger.Errorf("Error getting server version from database: %s", err.Error())
				return
			}

			if latestVersion == nil {
				h.Logger.Errorf("No server version found in database")
				return
			}

			currentVersion := retrievedSettings.GitVersion
			updateAvailable := libutils.IsUpdateAvailable(currentVersion, latestVersion.Version)

			result := admin.UpdateCheckResult{
				CurrentVersion:  currentVersion,
				LatestVersion:   latestVersion.Version,
				UpdateAvailable: updateAvailable,
			}

			resp := make([]interface{}, 2)
			resp[0] = "results:checkUpdates"
			resp[1] = result

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				h.Logger.Errorf("Error marshalling update check response: %s", err.Error())
				return
			}
			c.SafeSend(responseBytes)
		}
	case "createPad":
		{
			var padCreateData admin.PadCreateData
			if err := json.Unmarshal(message.Data, &padCreateData); err != nil {
				println("Error unmarshalling padCreate data:", err.Error())
			}
			padExists, err := h.padManager.DoesPadExist(padCreateData.PadName)
			if err != nil {
				h.Logger.Errorf("Error checking if Pad exists: %s", err.Error())
				return
			}
			if *padExists {
				h.Logger.Warnf("Pad %s already exists", padCreateData.PadName)
				errorMessage := admin.ErrorMessage{
					Error: "Pad already exists",
				}
				var resp = make([]interface{}, 2)
				resp[0] = "results:createPad"
				resp[1] = errorMessage
				responseBytes, err := json.Marshal(resp)
				if err != nil {
					println("Error marshalling response:", err.Error())
					return
				}

				c.SafeSend(responseBytes)
			} else {
				_, err := h.padManager.GetPad(padCreateData.PadName, nil, nil)
				if err != nil {
					h.Logger.Warnf("Error creating pad %s: %s", padCreateData.PadName, err.Error())
					return
				}
				h.Logger.Infof("Pad %s created successfully via admin interface", padCreateData.PadName)

				var resp = make([]interface{}, 2)
				resp[0] = "results:createPad"
				resp[1] = admin.SuccessMessage{
					Success: "Pad created " + padCreateData.PadName,
				}

				responseBytes, err := json.Marshal(resp)
				if err != nil {
					println("Error marshalling response:", err.Error())
					return
				}
				c.SafeSend(responseBytes)
			}

		}
	case "padLoad":
		{
			var padLoadData admin.PadLoadData
			if err := json.Unmarshal(message.Data, &padLoadData); err != nil {
				println("Error unmarshalling padLoad data:", err.Error())
				return
			}
			dbPads, err := h.store.QueryPad(padLoadData.Offset, padLoadData.Limit, padLoadData.SortBy, padLoadData.Ascending, padLoadData.Pattern)
			if err != nil {
				println("Error querying pads:", err.Error())
				return
			}

			var padDtos admin.PadDefinition

			padDtos.Total = dbPads.TotalPads
			padDtos.Results = make([]admin.PadDBSearch, 0)
			for _, dbPad := range dbPads.Pads {
				padDtos.Results = append(padDtos.Results, admin.PadDBSearch{
					PadName:        dbPad.Padname,
					RevisionNumber: dbPad.RevisionNumber,
					LastEdited:     dbPad.LastEdited,
					UserCount:      len(h.padMessageHandler.GetRoomSockets(dbPad.Padname)),
				})
			}

			resp := make([]interface{}, 2)
			resp[0] = "results:padLoad"
			resp[1] = padDtos

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}
			c.SafeSend(responseBytes)
		}
	case "getInstalled":
		{

			var epPlugin = []admin.InstalledPluginDefinition{
				{
					Name:         "etherpad",
					Description:  "The core Etherpad application",
					Version:      retrievedSettings.GitVersion,
					FrontendPath: "/plugins/etherpad",
					BackendPath:  "/lib/plugins/etherpad",

					Enabled: true,
				},
			}

			for _, plugin := range plugins.RegisteredPlugins {
				epPlugin = append(epPlugin, admin.InstalledPluginDefinition{
					Name:         plugin.Name(),
					Description:  plugin.Description(),
					Version:      retrievedSettings.GitVersion,
					Enabled:      plugin.IsEnabled(),
					FrontendPath: "/plugins/" + plugin.Name(),
					BackendPath:  "/lib/plugins/" + plugin.Name(),
				})
			}

			slices.SortFunc(epPlugin, func(a, b admin.InstalledPluginDefinition) int {
				return strings.Compare(a.Name, b.Name)
			})
			resp := make([]interface{}, 2)
			resp[0] = "results:installed"
			resp[1] = map[string]interface{}{
				"installed": epPlugin,
			}

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}
			c.SafeSend(responseBytes)
		}
	case "shout":
		{
			var adminMessage admin.ShoutMessageRequest
			if err := json.Unmarshal(message.Data, &adminMessage); err != nil {
				println("Error unmarshalling shout data:", err.Error())
				return
			}
			padShoutData := admin.ShoutMessageResponse{
				Type: "COLLABROOM",
				Data: struct {
					Type    string             `json:"type"`
					Payload admin.ShoutMessage `json:"payload"`
				}{Type: "result:shout", Payload: admin.ShoutMessage{
					Timestamp: time.Now().UnixMilli(),
					Message:   adminMessage,
				}},
			}
			var resp = make([]interface{}, 2)
			resp[0] = "result:shout"
			resp[1] = padShoutData
			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}

			h.hub.ClientsRWMutex.RLock()
			for key := range h.hub.Clients {
				key.SafeSend(responseBytes)
			}
			h.hub.ClientsRWMutex.RUnlock()

		}
	case "deletePad":
		{
			var padDeleteData admin.PadDeleteData
			if err := json.Unmarshal(message.Data, &padDeleteData); err != nil {
				println("Error unmarshalling padDelete data:", err.Error())
				return
			}

			if err := h.padMessageHandler.DeletePad(padDeleteData); err != nil {
				h.Logger.Warnf("Error deleting pad: %s", err.Error())
				return
			}

			h.Logger.Infof("Pad %s deleted successfully via admin interface", padDeleteData)

			var resp = make([]interface{}, 2)
			resp[0] = "results:deletePad"
			resp[1] = padDeleteData

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}
			c.SafeSend(responseBytes)
		}
	case "cleanupPadRevisions":
		{
			if !retrievedSettings.Cleanup.Enabled {
				h.Logger.Warnf("Cleanup is not enabled in settings")
				return
			}
			var padDeleteData admin.PadCleanupData
			if err := json.Unmarshal(message.Data, &padDeleteData); err != nil {
				println("Error unmarshalling padDelete data:", err.Error())
				return
			}

			padExists, err := h.padManager.DoesPadExist(padDeleteData)
			if err != nil {
				h.Logger.Errorf("Error checking if Pad exists: %s", err.Error())
				return
			}
			if !*padExists {
				h.Logger.Warnf("Pad %s does not exist", padDeleteData)
				return
			}
			if err := h.DeleteRevisions(padDeleteData, retrievedSettings.Cleanup.KeepRevisions); err != nil {
				h.Logger.Warnf("Error cleaning up revisions for pad %s: %s", padDeleteData, err.Error())
				return
			}
			h.Logger.Infof("Revisions for pad %s cleaned up successfully via admin interface", padDeleteData)

			var resp = make([]interface{}, 2)
			resp[0] = "results:cleanupPadRevisions"
			resp[1] = padDeleteData

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
			}
			c.SafeSend(responseBytes)
		}
	case "search":
		{
			pluginDef := admin.SeachchPluginDefinition{
				Results: make([]admin.PluginSearchDefinition, 0),
				Query: struct {
					Offset     int    `json:"offset"`
					Limit      int    `json:"limit"`
					SortBy     string `json:"sortBy"`
					SortDir    string `json:"sortDir"`
					SearchTerm string `json:"searchTerm"`
				}{Offset: 0, Limit: 99999, SortBy: "name", SortDir: "asc", SearchTerm: ""},
			}

			resp := make([]interface{}, 2)
			resp[0] = "results:search"
			resp[1] = pluginDef

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}
			c.SafeSend(responseBytes)
		}
	case "getStats":
		{
			totalUsers := len(h.padMessageHandler.SessionStore.sessions)
			totalUserMessage := admin.Stats{
				TotalUsers: totalUsers,
			}
			resp := make([]interface{}, 2)
			resp[0] = "results:stats"
			resp[1] = totalUserMessage
			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}
			c.SafeSend(responseBytes)
		}
	default:
		// Unknown event
		println("Unknown admin event:", message.Event)
	}
}

func (h AdminMessageHandler) DeleteRevisions(padId string, keepRevisions int) error {
	h.Logger.Debugf("Starting deletion of revisions for pad %s, keeping last %d revisions", padId, keepRevisions)

	retrievedPad, err := h.padManager.GetPad(padId, nil, nil)
	if err != nil {
		return err
	}

	if err := retrievedPad.Check(); err != nil {
		h.Logger.Errorf("Pad %s failed integrity check before revision deletion: %s", padId, err.Error())
		return err
	}

	if retrievedPad.Head <= keepRevisions {
		h.Logger.Infof("Pad %s has %d revisions, which is less than or equal to keepRevisions %d. No revisions will be deleted.", padId, retrievedPad.Head, keepRevisions)
		return nil
	}

	h.padMessageHandler.KickSessionsFromPad(padId)
	cleanupUntilRevision := retrievedPad.Head - keepRevisions
	h.Logger.Infof("Deleting revisions for pad %s until revision %d", padId, cleanupUntilRevision)
	compressedChangeset, err := h.padMessageHandler.ComposePadChangesets(retrievedPad, 0, cleanupUntilRevision+1)
	if err != nil {
		println("Error composing changeset:", err.Error())
		return err
	}

	// Save revisions to keep (we need to resave because of changed changesets due to compression)
	revisionsToKeep, err := retrievedPad.GetRevisions(0, retrievedPad.Head)
	if err != nil {
		println("Error getting revisions to keep:", err.Error())
		return err
	}
	currentRevsToKeep := make(map[int]db2.PadSingleRevision)
	for i := range *revisionsToKeep {
		currentRevsToKeep[(*revisionsToKeep)[i].RevNum] = (*revisionsToKeep)[i]
	}

	if err := retrievedPad.RemoveAllSavedRevisions(); err != nil {
		println("Error removing saved revisions:", err.Error())
		return err
	}

	padContent, err := h.padManager.GetPad(padId, nil, nil)
	if err != nil {
		return err
	}
	padContent.Head = keepRevisions
	if len(padContent.SavedRevisions) > 0 {
		newSavedRevisions := make([]revision.SavedRevision, 0)
		for i := 0; i < len(padContent.SavedRevisions); i++ {
			if padContent.SavedRevisions[i].RevNum > cleanupUntilRevision {
				padContent.SavedRevisions[i].RevNum = padContent.SavedRevisions[i].RevNum - cleanupUntilRevision
				newSavedRevisions = append(newSavedRevisions, padContent.SavedRevisions[i])
			}
		}
		padContent.SavedRevisions = newSavedRevisions
	}
	if err := padContent.Save(); err != nil {
		return errors.New("error saving pad after revision cleanup" + err.Error())
	}

	newAtext := changeset.MakeAText("\n", nil)
	pool := padContent.Pool
	optNewAtext, err := changeset.ApplyToAText(compressedChangeset, newAtext, pool)
	if err != nil {
		println("Error applying compressed changeset to atext:", err.Error())
		return err
	}
	newAtext = *optNewAtext

	createdRevision := adminutils.CreateRevision(compressedChangeset, currentRevsToKeep[cleanupUntilRevision].Timestamp, true, currentRevsToKeep[cleanupUntilRevision].AuthorId, newAtext, pool)

	if err := h.store.SaveRevision(padContent.Id, 0, createdRevision.Changeset, newAtext.ToDBAText(), pool.ToRevDB(), createdRevision.Meta.Author, createdRevision.Meta.Timestamp); err != nil {
		println("Error saving compressed revision:", err.Error())
		return err
	}
	for i := 0; i < keepRevisions; i++ {
		rev := i + cleanupUntilRevision + 1
		newRev := rev - cleanupUntilRevision

		currentRevisionDb, ok := currentRevsToKeep[rev]
		if !ok {
			println("Error: revision", rev, "not found in current revisions to keep")
			return errors.New("revision not found in current revisions to keep")
		}
		optNewAtext, err = changeset.ApplyToAText(currentRevisionDb.Changeset, newAtext, pool)
		if err != nil {
			println("Error applying changeset to atext for revision", rev, ":", err.Error())
			return err
		}
		newAtext = *optNewAtext

		createdRevision = adminutils.CreateRevision(currentRevisionDb.Changeset, currentRevisionDb.Timestamp, true, currentRevisionDb.AuthorId, newAtext, pool)
		if err := h.store.SaveRevision(padContent.Id, newRev, createdRevision.Changeset, newAtext.ToDBAText(), pool.ToRevDB(), createdRevision.Meta.Author, createdRevision.Meta.Timestamp); err != nil {
			println("Error saving revision after deleting:", err.Error())
			return err
		}
	}

	h.padManager.UnloadPad(padId)
	retrievedPad, err = h.padManager.GetPad(padId, nil, nil)
	if err != nil {
		return err
	}
	if err := retrievedPad.Check(); err != nil {
		h.Logger.Errorf("Pad %s failed integrity check after revision deletion: %s", padId, err.Error())
		return err
	}
	return nil
}
