package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	padModel "github.com/ether/etherpad-go/lib/models/pad"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/updater"
	"github.com/ether/etherpad-go/lib/utils"
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
	updater           *updater.Updater
}

func NewAdminMessageHandler(store db.DataStore, h *hooks.Hook, m *pad.Manager, padMessHandler *PadMessageHandler, logger *zap.SugaredLogger, hub *Hub, app *fiber.App, upd *updater.Updater) AdminMessageHandler {
	return AdminMessageHandler{
		store:             store,
		hook:              h,
		padManager:        m,
		padMessageHandler: padMessHandler,
		Logger:            logger,
		hub:               hub,
		App:               app,
		updater:           upd,
	}
}

func (h AdminMessageHandler) HandleMessage(message admin.EventMessage, retrievedSettings *settings.Settings, c *Client) {
	switch message.Event {
	case "load":
		{
			// The settings.json text as it is on disk (comments, ${ENV}
			// placeholders and all) is what the editor shows and saves.
			// The parsed settings used to be sent and saved instead, which
			// replaced the file by every resolved value: secrets from the
			// environment ended up in the file, comments and placeholders
			// were gone. The live values go along redacted.
			resp := make([]interface{}, 2)
			resp[0] = "settings"
			if !retrievedSettings.ShowSettingsInAdminPage {
				resp[1] = map[string]interface{}{"results": "NOT_ALLOWED"}
			} else {
				raw, err := os.ReadFile(settingsFilePath(retrievedSettings))
				if err != nil && !os.IsNotExist(err) {
					h.Logger.Errorf("Reading the settings file: %v", err)
				}
				text := string(raw)
				if err != nil {
					text = "{\n}\n"
				}
				resp[1] = map[string]interface{}{
					"results":  text,
					"resolved": retrievedSettings.ResolvedForAdmin(),
					"flags":    map[string]interface{}{"readOnly": retrievedSettings.Clustered()},
					"readOnly": retrievedSettings.Clustered(),
				}
			}
			responseBytes, err := json.Marshal(resp)
			if err != nil {
				return
			}
			c.SafeSend(responseBytes)
		}
	case "checkUpdates":
		{
			var result admin.UpdateCheckResult
			if h.updater != nil {
				status := h.updater.CheckNowSync()
				latestVersion := status.CurrentVersion
				if status.Latest != nil {
					latestVersion = status.Latest.Version
				}
				result = admin.UpdateCheckResult{
					CurrentVersion:  status.CurrentVersion,
					LatestVersion:   latestVersion,
					UpdateAvailable: status.UpdateAvailable,
				}
			} else {
				// Fallback for contexts without a wired updater (tests, updater
				// disabled): report against the persisted server version.
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
				result = admin.UpdateCheckResult{
					CurrentVersion:  currentVersion,
					LatestVersion:   latestVersion.Version,
					UpdateAvailable: libutils.IsUpdateAvailable(currentVersion, latestVersion.Version),
				}
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
	case "applyUpdate":
		{
			if h.updater == nil {
				h.Logger.Error("Updater not available")
				return
			}
			resp := make([]interface{}, 2)
			resp[0] = "results:applyUpdate"
			if err := h.updater.ApplyNow(); err != nil {
				h.Logger.Warnf("Manual update apply rejected: %s", err.Error())
				resp[1] = map[string]any{"ok": false, "error": err.Error()}
			} else {
				// On success the process applies the binary and exits 75 to
				// restart; this acknowledgement may not reach the client.
				resp[1] = map[string]any{"ok": true}
			}
			if responseBytes, err := json.Marshal(resp); err == nil {
				c.SafeSend(responseBytes)
			}
		}
	case "getUpdateStatus":
		{
			if h.updater == nil {
				h.Logger.Error("Updater not available")
				return
			}
			resp := make([]interface{}, 2)
			resp[0] = "results:updateStatus"
			resp[1] = h.updater.Status()
			if responseBytes, err := json.Marshal(resp); err == nil {
				c.SafeSend(responseBytes)
			}
		}
	case "acknowledgeUpdate":
		{
			if h.updater == nil {
				h.Logger.Error("Updater not available")
				return
			}
			resp := make([]interface{}, 2)
			resp[0] = "results:acknowledgeUpdate"
			if err := h.updater.Acknowledge(); err != nil {
				resp[1] = map[string]any{"ok": false, "error": err.Error()}
			} else {
				resp[1] = map[string]any{"ok": true}
			}
			if responseBytes, err := json.Marshal(resp); err == nil {
				c.SafeSend(responseBytes)
			}
		}
	case "createPad":
		{
			var padCreateData admin.PadCreateData
			if err := json.Unmarshal(message.Data, &padCreateData); err != nil {
				h.Logger.Warn("Error unmarshalling padCreate data:", err.Error())
			}
			unlock, err := h.padManager.LockPad(padCreateData.PadName)
			if err != nil {
				h.Logger.Errorf("Error creating pad %s: %s", padCreateData.PadName, err.Error())
				return
			}
			defer unlock()
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
					h.Logger.Warn("Error marshalling response:", err.Error())
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
					h.Logger.Warn("Error marshalling response:", err.Error())
					return
				}
				c.SafeSend(responseBytes)
			}

		}
	case "padLoad":
		{
			var padLoadData admin.PadLoadData
			if err := json.Unmarshal(message.Data, &padLoadData); err != nil {
				h.Logger.Warn("Error unmarshalling padLoad data:", err.Error())
				return
			}
			dbPads, err := h.store.QueryPad(padLoadData.Offset, padLoadData.Limit, padLoadData.SortBy, padLoadData.Ascending, padLoadData.Pattern)
			if err != nil {
				h.Logger.Warn("Error querying pads:", err.Error())
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
				h.Logger.Warn("Error marshalling response:", err.Error())
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
				h.Logger.Warn("Error marshalling response:", err.Error())
				return
			}
			c.SafeSend(responseBytes)
		}
	case "shout":
		{
			var adminMessage admin.ShoutMessageRequest
			if err := json.Unmarshal(message.Data, &adminMessage); err != nil {
				h.Logger.Warn("Error unmarshalling shout data:", err.Error())
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
				h.Logger.Warn("Error marshalling response:", err.Error())
				return
			}

			h.padMessageHandler.BroadcastToAllClients(responseBytes)

		}
	case "deletePad":
		{
			var padDeleteData admin.PadDeleteData
			if err := json.Unmarshal(message.Data, &padDeleteData); err != nil {
				h.Logger.Warn("Error unmarshalling padDelete data:", err.Error())
				return
			}

			unlock, err := h.padManager.LockPad(string(padDeleteData))
			if err == nil {
				err = h.padMessageHandler.DeletePad(padDeleteData)
				unlock()
			}
			if err != nil {
				h.Logger.Warnf("Error deleting pad: %s", err.Error())
				return
			}

			h.Logger.Infof("Pad %s deleted successfully via admin interface", padDeleteData)

			var resp = make([]interface{}, 2)
			resp[0] = "results:deletePad"
			resp[1] = padDeleteData

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				h.Logger.Warn("Error marshalling response:", err.Error())
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
				h.Logger.Warn("Error unmarshalling padDelete data:", err.Error())
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
			unlock, err := h.padManager.LockPad(string(padDeleteData))
			if err == nil {
				err = h.DeleteRevisions(padDeleteData, retrievedSettings.Cleanup.KeepRevisions)
				unlock()
			}
			if err != nil {
				h.Logger.Warnf("Error cleaning up revisions for pad %s: %s", padDeleteData, err.Error())
				return
			}
			h.Logger.Infof("Revisions for pad %s cleaned up successfully via admin interface", padDeleteData)

			var resp = make([]interface{}, 2)
			resp[0] = "results:cleanupPadRevisions"
			resp[1] = padDeleteData

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				h.Logger.Warn("Error marshalling response:", err.Error())
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
				h.Logger.Warn("Error marshalling response:", err.Error())
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
				h.Logger.Warn("Error marshalling response:", err.Error())
				return
			}
			c.SafeSend(responseBytes)
		}
	case "saveSettings":
		{
			if retrievedSettings.Clustered() {
				// A settings.json written here would only change this
				// instance and be lost when its pod is replaced.
				resp := []interface{}{"results:saveSettings", map[string]interface{}{
					"error": "Settings are read-only in a cluster. Change settings.json or the environment of all instances and roll them out.",
				}}
				responseBytes, _ := json.Marshal(resp)
				c.SafeSend(responseBytes)
				return
			}
			if !retrievedSettings.ShowSettingsInAdminPage {
				responseBytes, _ := json.Marshal([]interface{}{"results:saveSettings", map[string]interface{}{"error": "NOT_ALLOWED"}})
				c.SafeSend(responseBytes)
				return
			}
			var settingsJSON string
			if err := json.Unmarshal(message.Data, &settingsJSON); err != nil {
				settingsJSON = string(message.Data)
			}
			// Written byte for byte (comments stay), but only if it parses:
			// a typo must not leave a server that no longer starts.
			if _, err := settings.PrepareSettingsJSON([]byte(settingsJSON)); err != nil {
				responseBytes, _ := json.Marshal([]interface{}{"results:saveSettings", map[string]interface{}{
					"error": "The settings are not valid JSON, nothing was saved: " + err.Error()}})
				c.SafeSend(responseBytes)
				return
			}
			settingsPath := settingsFilePath(retrievedSettings)
			if err := writeFileAtomic(settingsPath, []byte(settingsJSON)); err != nil {
				h.Logger.Errorf("Error saving settings: %v", err)
				resp := make([]interface{}, 2)
				resp[0] = "results:saveSettings"
				resp[1] = map[string]interface{}{"error": err.Error()}
				responseBytes, _ := json.Marshal(resp)
				c.SafeSend(responseBytes)
				return
			}
			h.Logger.Info("Settings saved to " + settingsPath)
			resp := make([]interface{}, 2)
			resp[0] = "results:saveSettings"
			resp[1] = map[string]interface{}{"success": true}
			responseBytes, _ := json.Marshal(resp)
			c.SafeSend(responseBytes)
		}
	case "restartServer":
		{
			h.Logger.Info("Restart requested via admin UI")
			resp := make([]interface{}, 2)
			resp[0] = "results:restartServer"
			resp[1] = map[string]interface{}{"success": true}
			responseBytes, _ := json.Marshal(resp)
			c.SafeSend(responseBytes)
			go func() {
				time.Sleep(500 * time.Millisecond)
				os.Exit(0)
			}()
		}
	case "getConnections":
		{
			type connectionInfo struct {
				SessionID string `json:"sessionId"`
				PadID     string `json:"padId"`
				IP        string `json:"ip"`
				Type      string `json:"type"`
			}
			var connections []connectionInfo
			h.hub.ClientsRWMutex.RLock()
			for client := range h.hub.Clients {
				padId := ""
				ip := client.ClientIP
				connType := "admin"
				if sessionPad := h.padMessageHandler.SessionStore.padOf(client.SessionId); sessionPad != "" {
					padId = sessionPad
					connType = "pad"
				}
				connections = append(connections, connectionInfo{
					SessionID: client.SessionId,
					PadID:     padId,
					IP:        ip,
					Type:      connType,
				})
			}
			h.hub.ClientsRWMutex.RUnlock()
			resp := make([]interface{}, 2)
			resp[0] = "results:getConnections"
			resp[1] = map[string]interface{}{"connections": connections}
			responseBytes, _ := json.Marshal(resp)
			c.SafeSend(responseBytes)
		}
	case "getSystemInfo":
		{
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			resp := make([]interface{}, 2)
			resp[0] = "results:getSystemInfo"
			resp[1] = map[string]interface{}{
				"memAlloc":      memStats.Alloc,
				"memTotalAlloc": memStats.TotalAlloc,
				"memSys":        memStats.Sys,
				"numGoroutine":  runtime.NumGoroutine(),
				"numGC":         memStats.NumGC,
				"goVersion":     runtime.Version(),
				"numCPU":        runtime.NumCPU(),
				"dbType":        h.padMessageHandler.Logger.Desugar().Name(), // placeholder
			}
			responseBytes, _ := json.Marshal(resp)
			c.SafeSend(responseBytes)
		}
	case "kickUser":
		{
			var kickData struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.Unmarshal(message.Data, &kickData); err != nil {
				h.Logger.Warn("Error unmarshalling kickUser data:", err.Error())
				return
			}
			h.hub.ClientsRWMutex.RLock()
			var toKick []*Client
			for client := range h.hub.Clients {
				if client.SessionId == kickData.SessionID {
					toKick = append(toKick, client)
				}
			}
			h.hub.ClientsRWMutex.RUnlock()
			for _, client := range toKick {
				// Send disconnect message in the wire format ["message", {...}] and close
				kickMsg, _ := json.Marshal([]interface{}{"message", map[string]string{"disconnect": "kicked"}})
				client.SafeSend(kickMsg)
				client.Conn.Close()
			}
			resp := make([]interface{}, 2)
			resp[0] = "results:kickUser"
			resp[1] = map[string]interface{}{"success": true}
			responseBytes, _ := json.Marshal(resp)
			c.SafeSend(responseBytes)
		}
	case "searchPadContent":
		{
			var searchData struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(message.Data, &searchData); err != nil {
				h.Logger.Warn("Error unmarshalling searchPadContent:", err.Error())
				return
			}
			if searchData.Limit <= 0 || searchData.Limit > 50 {
				searchData.Limit = 20
			}
			padIds, err := h.store.GetPadIds()
			if err != nil || padIds == nil {
				resp := make([]interface{}, 2)
				resp[0] = "results:searchPadContent"
				resp[1] = map[string]interface{}{"results": []interface{}{}}
				responseBytes, _ := json.Marshal(resp)
				c.SafeSend(responseBytes)
				return
			}
			type searchResult struct {
				PadID   string `json:"padId"`
				Snippet string `json:"snippet"`
			}
			var results []searchResult
			query := strings.ToLower(searchData.Query)
			for _, padId := range *padIds {
				if len(results) >= searchData.Limit {
					break
				}
				pad, err := h.padManager.GetPad(padId, nil, nil)
				if err != nil || pad == nil {
					continue
				}
				text := pad.Text()
				if strings.Contains(strings.ToLower(text), query) {
					// Extract snippet around match
					idx := strings.Index(strings.ToLower(text), query)
					start := idx - 50
					if start < 0 {
						start = 0
					}
					end := idx + len(searchData.Query) + 50
					if end > len(text) {
						end = len(text)
					}
					snippet := text[start:end]
					results = append(results, searchResult{PadID: padId, Snippet: snippet})
				}
			}
			if results == nil {
				results = []searchResult{}
			}
			resp := make([]interface{}, 2)
			resp[0] = "results:searchPadContent"
			resp[1] = map[string]interface{}{"results": results}
			responseBytes, _ := json.Marshal(resp)
			c.SafeSend(responseBytes)
		}
	case "getPadContent":
		{
			var padName string
			if err := json.Unmarshal(message.Data, &padName); err != nil {
				padName = string(message.Data)
			}
			padName = strings.Trim(padName, "\"")
			pad, err := h.padManager.GetPad(padName, nil, nil)
			if err != nil || pad == nil {
				resp := make([]interface{}, 2)
				resp[0] = "results:getPadContent"
				resp[1] = map[string]interface{}{"padId": padName, "content": "", "error": "Pad not found"}
				responseBytes, _ := json.Marshal(resp)
				c.SafeSend(responseBytes)
				return
			}
			text := pad.Text()
			if len(text) > 2000 {
				text = text[:2000] + "..."
			}
			resp := make([]interface{}, 2)
			resp[0] = "results:getPadContent"
			resp[1] = map[string]interface{}{"padId": padName, "content": text}
			responseBytes, _ := json.Marshal(resp)
			c.SafeSend(responseBytes)
		}
	case "bulkDeletePads":
		{
			var bulkData struct {
				PadNames []string `json:"padNames"`
			}
			if err := json.Unmarshal(message.Data, &bulkData); err != nil {
				h.Logger.Warn("Error unmarshalling bulkDeletePads:", err.Error())
				return
			}
			deleted := 0
			for _, padName := range bulkData.PadNames {
				// Kick users first
				h.hub.ClientsRWMutex.RLock()
				for client := range h.hub.Clients {
					if padName != "" && h.padMessageHandler.SessionStore.padOf(client.SessionId) == padName {
						client.SendPadDelete()
					}
				}
				h.hub.ClientsRWMutex.RUnlock()

				unlock, err := h.padManager.LockPad(padName)
				if err == nil {
					err = h.padMessageHandler.DeletePad(admin.PadDeleteData(padName))
					unlock()
				}
				if err == nil {
					deleted++
				}
			}
			resp := make([]interface{}, 2)
			resp[0] = "results:bulkDeletePads"
			resp[1] = map[string]interface{}{"deleted": deleted}
			responseBytes, _ := json.Marshal(resp)
			c.SafeSend(responseBytes)
		}
	default:
		h.Logger.Warn("Unknown admin event:", message.Event)
	}
}

func (h AdminMessageHandler) DeleteRevisions(padId string, keepRevisions int) error {
	h.Logger.Debugf("Starting deletion of revisions for pad %s, keeping last %d revisions", padId, keepRevisions)

	retrievedPad, err := h.padManager.GetPad(padId, nil, nil)
	if err == nil {
		err = retrievedPad.Check()
	}
	if err != nil {
		missing, mErr := h.missingRevisions(padId)
		if mErr == nil && len(missing) > 0 {
			latest, rErr := h.latestText(padId)
			if rErr != nil {
				return fmt.Errorf("pad %s is missing revision(s) %v: %v. Nothing changed", padId, missing, rErr)
			}
			if keepRevisions == 0 {
				// Full compaction needs no history: it repairs the pad
				// from its latest text.
				return h.compactDamagedPad(padId, missing, latest)
			}
			return fmt.Errorf("pad %s is missing revision(s) %v, so its history cannot be shortened. "+
				"The pad's text is safe (it can be rebuilt from revision %d on); a full compaction "+
				"(keepRevisions 0) rebuilds the pad from it without the history", padId, missing, latest.from)
		}
		h.Logger.Errorf("Pad %s failed integrity check before revision deletion: %s", padId, err.Error())
		return err
	}

	if retrievedPad.Head <= keepRevisions {
		h.Logger.Infof("Pad %s has %d revisions, which is less than or equal to keepRevisions %d. No revisions will be deleted.", padId, retrievedPad.Head, keepRevisions)
		return nil
	}

	cleanupUntilRevision := retrievedPad.Head - keepRevisions
	h.Logger.Infof("Deleting revisions for pad %s until revision %d", padId, cleanupUntilRevision)
	compressedChangeset, err := h.padMessageHandler.ComposePadChangesets(retrievedPad, 0, cleanupUntilRevision+1)
	if err != nil {
		h.Logger.Warn("Error composing changeset:", err.Error())
		return err
	}
	oldRevisions, err := retrievedPad.GetRevisions(0, retrievedPad.Head)
	if err != nil {
		return err
	}
	byNum := make(map[int]db2.PadSingleRevision, len(*oldRevisions))
	for _, r := range *oldRevisions {
		byNum[r.RevNum] = r
	}

	// Build the new history in memory first and check that it ends at the
	// pad's current text. It is then swapped in with one transaction: the
	// old code deleted every revision before writing the new ones, so a
	// failed write lost the pad's latest text.
	pool := retrievedPad.Pool
	poolDB := pool.ToRevDB()
	atext := changeset.MakeAText("\n", nil)
	newRevisions := make([]db2.PadSingleRevision, 0, keepRevisions+1)
	appendRev := func(cs string, src db2.PadSingleRevision) error {
		next, err := changeset.ApplyToAText(cs, atext, pool)
		if err != nil {
			return err
		}
		atext = *next
		newRevisions = append(newRevisions, db2.PadSingleRevision{
			PadId: padId, RevNum: len(newRevisions), Changeset: cs, AText: atext.ToDBAText(),
			AuthorId: src.AuthorId, Timestamp: src.Timestamp, Pool: &poolDB,
		})
		return nil
	}
	base, ok := byNum[cleanupUntilRevision]
	if !ok {
		return fmt.Errorf("revision %d of pad %s is missing", cleanupUntilRevision, padId)
	}
	if err := appendRev(compressedChangeset, base); err != nil {
		return err
	}
	for rev := cleanupUntilRevision + 1; rev <= retrievedPad.Head; rev++ {
		src, ok := byNum[rev]
		if !ok {
			return fmt.Errorf("revision %d of pad %s is missing", rev, padId)
		}
		if err := appendRev(src.Changeset, src); err != nil {
			return fmt.Errorf("revision %d: %w", rev, err)
		}
	}
	if atext.Text != retrievedPad.AText.Text || atext.Attribs != retrievedPad.AText.Attribs {
		return fmt.Errorf("compacted history of pad %s does not end at its text; nothing changed", padId)
	}

	savedRevisions := make([]db2.SavedRevision, 0)
	for _, sr := range retrievedPad.SavedRevisions {
		if sr.RevNum > cleanupUntilRevision {
			savedRevisions = append(savedRevisions, db2.SavedRevision{
				RevNum: sr.RevNum - cleanupUntilRevision, SavedBy: sr.SavedBy, Timestamp: sr.Timestamp, Label: sr.Label, Id: sr.Id,
			})
		}
	}
	row := db2.PadDB{
		Head: keepRevisions, SavedRevisions: savedRevisions, ChatHead: retrievedPad.ChatHead, Pool: poolDB,
		ATextText: atext.Text, ATextAttribs: atext.Attribs, PublicStatus: retrievedPad.PublicStatus,
		DocumentType: retrievedPad.DocumentType, ReadOnlyId: retrievedPad.ReadonlyId,
	}

	h.padMessageHandler.KickSessionsFromPad(padId)
	if err := h.store.ReplaceRevisions(padId, row, newRevisions); err != nil {
		h.padManager.UnloadPad(padId)
		h.Logger.Warnf("Compacting pad %s failed, the pad is unchanged: %v", padId, err)
		return err
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

// missingRevisions lists the revision numbers below the highest stored one
// that are not stored.
func (h AdminMessageHandler) missingRevisions(padId string) ([]int, error) {
	v, err := h.store.GetPadVersion(padId)
	if err != nil {
		return nil, err
	}
	if v.Rev < 0 {
		return nil, nil
	}
	// One by one: reading a range refuses a range with holes.
	var missing []int
	for r := 0; r <= v.Rev; r++ {
		if _, err := h.store.GetRevision(padId, r); err != nil {
			missing = append(missing, r)
		}
	}
	return missing, nil
}

type recoveredText struct {
	atext      apool.AText
	pool       apool.APool
	from, head int
	authorId   *string
	timestamp  int64
}

func poolFromRev(src db2.RevPool) apool.APool {
	pool := apool.NewAPool()
	for k, a := range src.NumToAttrib {
		num, err := strconv.Atoi(k)
		if err != nil || len(a) < 2 {
			continue
		}
		attr := apool.Attribute{Key: a[0], Value: a[1]}
		pool.NumToAttrib[num] = attr
		pool.AttribToNum[attr] = num
	}
	pool.NextNum = src.NextNum
	return pool
}

// latestText rebuilds the pad's latest text despite holes in its history:
// from the newest revision that stores its text (key revisions do), with
// the revisions after it. It fails when one of those is missing, since the
// latest text then cannot be known.
func (h AdminMessageHandler) latestText(padId string) (*recoveredText, error) {
	v, err := h.store.GetPadVersion(padId)
	if err != nil {
		return nil, err
	}
	var base *db2.PadSingleRevision
	for r := v.Rev; r >= 0; r-- {
		rev, err := h.store.GetRevision(padId, r)
		if err == nil && rev.AText.Text != "" {
			base = rev
			break
		}
	}
	if base == nil {
		return nil, errors.New("no revision stores the pad's text")
	}
	row, err := h.store.GetPad(padId)
	if err != nil {
		return nil, err
	}
	pool := poolFromRev(row.Pool)
	out := &recoveredText{atext: apool.FromDBAText(base.AText), pool: pool, from: base.RevNum, head: base.RevNum,
		authorId: base.AuthorId, timestamp: base.Timestamp}
	for r := base.RevNum + 1; r <= v.Rev; r++ {
		rev, err := h.store.GetRevision(padId, r)
		if err != nil {
			return nil, fmt.Errorf("revision %d after the last stored text (revision %d) is missing, so the latest text cannot be rebuilt", r, base.RevNum)
		}
		next, err := changeset.ApplyToAText(rev.Changeset, out.atext, pool)
		if err != nil {
			return nil, fmt.Errorf("revision %d: %w", r, err)
		}
		out.atext, out.head, out.authorId, out.timestamp = *next, r, rev.AuthorId, rev.Timestamp
	}
	return out, nil
}

// compactDamagedPad replaces the history of a pad with holes by one
// revision holding its latest text.
func (h AdminMessageHandler) compactDamagedPad(padId string, missing []int, latest *recoveredText) error {
	row, err := h.store.GetPad(padId)
	if err != nil {
		return err
	}
	pool := latest.pool
	text := latest.atext.Text
	ops, err := changeset.DeserializeOps(latest.atext.Attribs)
	if err != nil {
		return err
	}
	bodyLen := utils.RuneCount(text) - 1
	builder := changeset.NewBuilder(1)
	pos := 0
	for _, op := range *ops {
		start, end := pos, min(pos+op.Chars, bodyLen)
		pos += op.Chars
		if start >= end {
			continue
		}
		attribs := changeset.FromString(op.Attribs, &pool)
		if a := attribs.Get("author"); a == nil || *a == "" {
			attribs.Set("author", padModel.SystemAuthorId)
		}
		builder.Insert(utils.RuneSlice(text, start, end), changeset.AttribsString(attribs.String()), nil)
	}
	cs := builder.ToString()
	atext, err := changeset.ApplyToAText(cs, changeset.MakeAText("\n", nil), pool)
	if err != nil {
		return err
	}
	if atext.Text != text {
		return fmt.Errorf("pad %s: could not rebuild the latest text; nothing changed", padId)
	}
	poolDB := pool.ToRevDB()
	rev := db2.PadSingleRevision{PadId: padId, RevNum: 0, Changeset: cs, AText: atext.ToDBAText(),
		AuthorId: latest.authorId, Timestamp: latest.timestamp, Pool: &poolDB}
	newRow := db2.PadDB{Head: 0, SavedRevisions: []db2.SavedRevision{}, ChatHead: row.ChatHead, Pool: poolDB,
		ATextText: atext.Text, ATextAttribs: atext.Attribs, PublicStatus: row.PublicStatus,
		DocumentType: row.DocumentType, ReadOnlyId: row.ReadOnlyId}
	h.padMessageHandler.KickSessionsFromPad(padId)
	h.padManager.UnloadPad(padId)
	if err := h.store.ReplaceRevisions(padId, newRow, []db2.PadSingleRevision{rev}); err != nil {
		return err
	}
	h.padManager.UnloadPad(padId)
	h.Logger.Warnf("Pad %s was missing revision(s) %v; its history was replaced by its latest text", padId, missing)
	repaired, err := h.padManager.GetPad(padId, nil, nil)
	if err != nil {
		return err
	}
	return repaired.Check()
}

func settingsFilePath(s *settings.Settings) string {
	if s.Root != "" {
		return s.Root + "/settings.json"
	}
	return "settings.json"
}

// writeFileAtomic replaces path in one step: a crash while writing leaves the
// old file, never a truncated one.
func writeFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
