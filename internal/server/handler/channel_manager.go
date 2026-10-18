package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"infineon/internal/channelmanager"
)

// ChannelMgr is the channel manager instance (set during init)
var ChannelMgr *channelmanager.ChannelManager

// SetChannelManager sets the global channel manager instance
func SetChannelManager(cm *channelmanager.ChannelManager) {
	ChannelMgr = cm
}

// channelMgrOrUnavailable resolves the process-wide channel manager. When the
// active storage backend has no raw SQL handle the manager is never wired, so
// handlers answer 503 rather than dereferencing a nil pointer.
func channelMgrOrUnavailable(w http.ResponseWriter) *channelmanager.ChannelManager {
	if ChannelMgr == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "Channel manager is unavailable on this storage backend",
		})
		return nil
	}
	return ChannelMgr
}

// ListChannels returns all OTA channel connections
func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	propertyID := getPropertyID(r)
	ctx := r.Context()

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	channels, err := cm.ListChannels(ctx, propertyID)
	if err != nil {
		log.Printf("[CHANNEL] ListChannels error: %v (propertyID=%s)", err, propertyID)
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to list channels: " + err.Error()})
		return
	}

	respondJSON(w, 200, map[string]interface{}{
		"channels": channels,
		"total":    len(channels),
	})
}

// SyncRates pushes rates to all enabled channels
func (h *Handler) SyncRates(w http.ResponseWriter, r *http.Request) {
	propertyID := getPropertyID(r)
	ctx := r.Context()

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	resp, err := cm.SyncRatesToAll(ctx, propertyID)
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": err.Error()})
		return
	}

	respondJSON(w, 200, resp)
}

// SyncAvailability pushes availability to all enabled channels
func (h *Handler) SyncAvailability(w http.ResponseWriter, r *http.Request) {
	propertyID := getPropertyID(r)
	ctx := r.Context()

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	resp, err := cm.SyncAvailabilityToAll(ctx, propertyID)
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": err.Error()})
		return
	}

	respondJSON(w, 200, resp)
}

// PullChannelBookings imports bookings from all enabled channels
func (h *Handler) PullChannelBookings(w http.ResponseWriter, r *http.Request) {
	propertyID := getPropertyID(r)
	ctx := r.Context()

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	resp, err := cm.PullBookingsFromAll(ctx, propertyID)
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": err.Error()})
		return
	}

	respondJSON(w, 200, resp)
}

// ChannelPerformance returns per-channel performance metrics
func (h *Handler) ChannelPerformance(w http.ResponseWriter, r *http.Request) {
	propertyID := getPropertyID(r)
	ctx := r.Context()

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	perf := cm.GetChannelPerformance(ctx, propertyID)
	respondJSON(w, 200, perf)
}

// ChannelSyncLog returns sync history for a channel
func (h *Handler) ChannelSyncLog(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		respondJSON(w, 400, map[string]interface{}{"error": "channel_id required"})
		return
	}
	ctx := r.Context()

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	logs := cm.GetSyncLog(ctx, channelID, 50)
	respondJSON(w, 200, map[string]interface{}{"logs": logs})
}

// ChannelBookings returns bookings from a specific channel
func (h *Handler) ChannelBookings(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channel_id")
	status := r.URL.Query().Get("status")
	ctx := r.Context()

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	bookings := cm.GetChannelBookings(ctx, channelID, status)
	respondJSON(w, 200, map[string]interface{}{"bookings": bookings})
}

// UpdateChannelSettings updates a channel's configuration
func (h *Handler) UpdateChannelSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ChannelID  string  `json:"channel_id"`
		Commission float64 `json:"commission"`
		Markup     float64 `json:"markup"`
		MarkupType string  `json:"markup_type"`
		MinStay    int     `json:"min_stay"`
		MaxStay    int     `json:"max_stay"`
		Enabled    bool    `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]interface{}{"error": "Invalid request body"})
		return
	}

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	err := cm.UpdateChannelSettings(ctx, req.ChannelID, req.Commission, req.Markup, req.MarkupType, req.MinStay, req.MaxStay, req.Enabled)
	if err != nil {
		respondJSON(w, 500, map[string]interface{}{"error": "Failed to update channel"})
		return
	}

	respondJSON(w, 200, map[string]interface{}{"message": "Channel settings updated"})
}

// ChannelDashboard returns a unified channel management dashboard
func (h *Handler) ChannelDashboard(w http.ResponseWriter, r *http.Request) {
	propertyID := getPropertyID(r)
	ctx := r.Context()

	cm := channelMgrOrUnavailable(w)
	if cm == nil {
		return
	}
	channels, _ := cm.ListChannels(ctx, propertyID)
	perf := cm.GetChannelPerformance(ctx, propertyID)

	connected := 0
	for _, ch := range channels {
		if ch.Status == "connected" && ch.Enabled {
			connected++
		}
	}

	respondJSON(w, 200, map[string]interface{}{
		"summary": map[string]interface{}{
			"total_channels":     len(channels),
			"connected_channels": connected,
			"enabled_channels":   connected,
			"total_revenue":      perf["total_revenue"],
		},
		"channels":    channels,
		"performance": perf,
	})
}
