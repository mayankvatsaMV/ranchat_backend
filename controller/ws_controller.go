// ws_controller.go — WebSocket upgrade handler for ACTIVE MATCH sessions ONLY.
//
// Flow:
//   1. User receives matchId from long-polling POST /matches/search
//   2. User connects to GET /ws/match/:matchId?token=<jwt>
//   3. Backend validates JWT + confirms user belongs to this match
//   4. User is registered into the in-memory match room
//   5. Realtime events (friend requests, etc.) flow bidirectionally
//
// Clients send JSON: { "type": "SEND_FRIEND_REQUEST" }
// Server pushes JSON: { "type": "FRIEND_REQUEST_RECEIVED", "data": {...} }
package controller

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"ranchat_backend/models"
	"ranchat_backend/repository"
	"ranchat_backend/service"
	"ranchat_backend/utils"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ConnectMatchWS handles GET /ws/match/:matchId?token=<jwt>
func ConnectMatchWS(c *gin.Context) {
	matchID := c.Param("matchId")

	// 1. Validate JWT from query param (WebSocket handshake can't set headers easily)
	token := c.Query("token")
	if token == "" {
		utils.SendError(c, http.StatusUnauthorized, "token query param required")
		return
	}
	claims, err := utils.ParseToken(token)
	if err != nil {
		utils.SendError(c, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	userID := claims.UserID

	// 2. Confirm the user actually belongs to this match
	match := repository.GetMatchByUser(userID)
	if match == nil || match.MatchID != matchID {
		utils.SendError(c, http.StatusForbidden, "you are not part of this match")
		return
	}

	// 3. Upgrade HTTP → WebSocket
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] upgrade error for user=%s match=%s: %v", userID, matchID, err)
		return
	}

	// 4. Register into the in-memory match room
	client := repository.JoinRoom(matchID, userID, conn)
	log.Printf("[WS] user=%s joined match room=%s", userID, matchID)

	// 5. Start write pump in background
	go client.WritePump()

	// 6. Read pump — blocks until disconnect
	client.ReadPump(handleWSMessage, func(uid, mid string) {
		log.Printf("[WS] user=%s disconnected from match room=%s", uid, mid)
		repository.LeaveRoom(mid, uid)

		// Notify partner that this user has disconnected from the room
		partner := repository.GetPartner(mid, uid)
		if partner != nil {
			repository.SendToClient(partner, models.WSMessage{
				Type: models.WSEventPartnerDisconnected,
			})
		}
	})
}

// handleWSMessage dispatches incoming client events to the right service function.
func handleWSMessage(client *repository.WSClient, msgType string, raw []byte) {
	ctx := utils.BackgroundContext()

	switch msgType {
	case models.WSClientEventSendFriendRequest:
		req, _, err := service.SendFriendRequest(ctx, client.UserID, client.MatchID)
		if err != nil {
			repository.SendToClient(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": err.Error()},
			})
			return
		}
		// Echo back to sender as confirmation
		repository.SendToClient(client, models.WSMessage{
			Type: "FRIEND_REQUEST_SENT",
			Data: models.WSFriendRequestPayload{
				RequestID: req.ID.Hex(),
				FromID:    client.UserID,
				MatchID:   client.MatchID,
			},
		})

	case models.WSClientEventAcceptFriendRequest:
		var payload struct {
			RequestID string `json:"requestId"`
		}
		if err := utils.ParseJSON(raw, &payload); err != nil || payload.RequestID == "" {
			repository.SendToClient(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": "requestId is required"},
			})
			return
		}
		_, friendship, _, err := service.ResolveFriendRequest(ctx, client.UserID, payload.RequestID, true)
		if err != nil {
			repository.SendToClient(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": err.Error()},
			})
			return
		}
		fid := ""
		if friendship != nil {
			fid = friendship.ID.Hex()
		}
		repository.SendToClient(client, models.WSMessage{
			Type: models.WSEventFriendRequestAccepted,
			Data: models.WSAcceptPayload{RequestID: payload.RequestID, FriendshipID: fid},
		})

	case models.WSClientEventRejectFriendRequest:
		var payload struct {
			RequestID string `json:"requestId"`
		}
		if err := utils.ParseJSON(raw, &payload); err != nil || payload.RequestID == "" {
			repository.SendToClient(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": "requestId is required"},
			})
			return
		}
		if _, _, _, err := service.ResolveFriendRequest(ctx, client.UserID, payload.RequestID, false); err != nil {
			repository.SendToClient(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": err.Error()},
			})
			return
		}
		repository.SendToClient(client, models.WSMessage{
			Type: models.WSEventFriendRequestRejected,
		})

	default:
		// Unknown event type — silently ignore
		log.Printf("[WS] unknown message type=%s from user=%s", msgType, client.UserID)
	}
}
