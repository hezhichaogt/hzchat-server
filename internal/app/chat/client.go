/*
Package chat contains the core logic for handling real-time chat rooms, user connections, and message broadcasting.

This file defines the Client struct, representing an active WebSocket connection. It manages the client's
lifecycle, message communication loops (ReadPump and WritePump), and interaction with the Room.
*/
package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"hzchat/internal/app/user"
	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/logx"
)

const (
	// timeout duration for writing to the WebSocket connection.
	writeWait = 10 * time.Second

	// maximum time allowed for the server to wait for a Pong message from the client.
	pongWait = 60 * time.Second

	// frequency at which the server sends a Ping message.
	pingPeriod = (pongWait * 9) / 10

	// maximum allowed size (in bytes) of a message sent by the client.
	maxMessageSize = 8192

	// maximum allowed size (in bytes) for text message content.
	MaxContentBytes = 5000

	// MaxAttachmentsCount defines the maximum number of attachments allowed per message.
	MaxAttachmentsCount = 3

	// WsCloseCodeSessionKicked is a custom WebSocket Close Code (4000-4999 range)
	// used to signal the client that the session was replaced by a new connection.
	WsCloseCodeSessionKicked = 4001

	// TokenRefreshWindow defines how much time before the token expires we should attempt to refresh it.
	TokenRefreshWindow = 2 * time.Minute
)

// Client struct represents an active WebSocket connection and its associated user.
type Client struct {
	// the chat room the client currently belongs to.
	room *Room

	// underlying WebSocket connection object.
	conn *websocket.Conn

	// associated client user.
	user user.User

	// tokenExpiry records the expiration time of the current JWT used by the client.
	tokenExpiry time.Time

	// a buffered channel used to queue messages waiting to be sent to the client.
	send chan []byte

	// structured logger with client and room context.
	logger zerolog.Logger
}

// NewClient constructs and returns a new Client instance.
func NewClient(room *Room, wsConn *websocket.Conn, user user.User, expiry time.Time) *Client {
	clientLogger := logx.Logger().With().
		Str("client_id", user.ID).
		Str("room_code", room.Code).
		Logger()

	client := &Client{
		room:        room,
		conn:        wsConn,
		user:        user,
		tokenExpiry: expiry,
		send:        make(chan []byte, 256),
		logger:      clientLogger,
	}

	return client
}

// ReadPump handles reading messages from the WebSocket connection.
// It handles heartbeats (Pong), message parsing, and performs cleanup upon connection closure.
func (c *Client) ReadPump() {
	defer c.cleanupOnDisconnect()

	c.conn.SetReadLimit(maxMessageSize)

	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		c.logger.Error().Err(err).Msg("Failed to set read deadline")
		return
	}

	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Info().Err(err).Msg("Error reading message (Client close/going away)")
			}
			break
		}

		c.processInboundMessage(messageBytes)
	}
}

// cleanupOnDisconnect handles the necessary cleanup steps when the client's ReadPump terminates.
func (c *Client) cleanupOnDisconnect() {
	c.logger.Info().Msg("Client connection cleanup starting.")

	// notify the room to unregister the client
	select {
	case c.room.unregister <- c:
	default:
		c.logger.Warn().Msg("Room unregister channel blocked. Connection cleanup still proceeding.")
	}

	// close the connection
	if err := c.conn.Close(); err != nil {
		c.logger.Error().Err(err).Msg("Client connection close error")
	}
}

// processInboundMessage handles raw byte messages received from the client.
func (c *Client) processInboundMessage(messageBytes []byte) {
	var inboundMsg struct {
		Type    MessageType     `json:"type"`
		Payload json.RawMessage `json:"payload,omitempty"`
		TempID  string          `json:"tempID,omitempty"`
	}

	if err := json.Unmarshal(messageBytes, &inboundMsg); err != nil {
		c.logger.Warn().Err(err).
			Bytes("message_bytes", messageBytes).
			Msg("Client sent invalid JSON")
		return
	}

	switch inboundMsg.Type {
	case TypeText:
		c.handleText(inboundMsg.Payload, inboundMsg.TempID)

	case TypeAttachments:
		c.handleAttachments(inboundMsg.Payload, inboundMsg.TempID)

	default:
		c.logger.Warn().Str("msg_type", string(inboundMsg.Type)).Msg("Client sent unsupported message type")
	}
}

// handleText processes incoming text messages from the client.
func (c *Client) handleText(payloadBytes json.RawMessage, tempID string) {
	var textPayload TextPayload
	if err := json.Unmarshal(payloadBytes, &textPayload); err != nil {
		c.logger.Warn().Err(err).Msg("Client sent invalid TEXT payload")
		return
	}

	if len(textPayload.Content) > MaxContentBytes {
		c.SendError(errs.NewError(errs.ErrMessageContentTooLong))
		return
	}

	broadcastMsg, err := NewMessage(TypeText, c.room.Code, c.user, textPayload)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to create new text message for broadcast")
		return
	}

	c.sendConfirmation(tempID, broadcastMsg)
	c.room.broadcast <- broadcastMsg
}

// handleAttachments processes incoming attachment messages from the client.
func (c *Client) handleAttachments(payloadBytes json.RawMessage, tempID string) {
	var attachmentsPayload AttachmentsPayload
	if err := json.Unmarshal(payloadBytes, &attachmentsPayload); err != nil {
		c.logger.Warn().Err(err).Msg("Client sent invalid ATTACHMENTS payload")
		return
	}

	if count := len(attachmentsPayload.Attachments); count == 0 || count > MaxAttachmentsCount {
		c.SendError(errs.NewError(errs.ErrAttachmentCountInvalid, MaxAttachmentsCount))
		return
	}

	if len(attachmentsPayload.Description) > MaxContentBytes {
		c.SendError(errs.NewError(errs.ErrMessageContentTooLong))
		return
	}

	expectedKeyPrefix := fmt.Sprintf("%s/", c.room.Code)

	for i := range attachmentsPayload.Attachments {
		a := &attachmentsPayload.Attachments[i]

		if !strings.HasPrefix(a.Key, expectedKeyPrefix) {
			c.SendError(errs.NewError(errs.ErrAttachmentKeyInvalid))
			return
		}

		if err := ValidateFileType(a.Name, a.MimeType); err != nil {
			c.SendError(err)
			return
		}

		a.Meta = nil
	}

	broadcastMsg, err := NewMessage(TypeAttachments, c.room.Code, c.user, attachmentsPayload)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to create new attachments message for broadcast")
		return
	}

	c.sendConfirmation(tempID, broadcastMsg)
	c.room.broadcast <- broadcastMsg
}

// WritePump handles writing messages from the Client.send channel to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()

		// ensure the connection is closed on exit
		if err := c.conn.Close(); err != nil {
			c.logger.Error().Err(err).Msg("Client connection close error in WritePump")
		}
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !c.writeQueuedMessage(message, ok) {
				return
			}

		case <-ticker.C:
			if !c.writePingMessage() {
				return
			}

			c.checkAndRefreshToken()
		}
	}
}

// writeQueuedMessage handles messages pulled from the send channel, writing them to the WebSocket.
// Returns true if the WritePump loop should continue, false if it should terminate.
func (c *Client) writeQueuedMessage(message []byte, ok bool) bool {
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		c.logger.Error().Err(err).Msg("Failed to set write deadline")
		return false
	}

	if !ok {
		if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
			c.logger.Error().Err(err).Msg("Error writing close message")
		}
		return false
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
		c.logger.Error().Err(err).Msg("Error writing message")
		return false
	}

	return true
}

// writePingMessage sends a periodic WebSocket Ping message to maintain the connection heartbeat.
// Returns false if the WritePump loop should terminate due to write failure.
func (c *Client) writePingMessage() bool {
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		c.logger.Error().Err(err).Msg("Failed to set write deadline on ping")
		return false
	}

	if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		c.logger.Error().Err(err).Msg("Error writing ping")
		return false
	}

	return true
}

// checkAndRefreshToken checks if the current JWT is close to expiry and generates a new one if necessary.
func (c *Client) checkAndRefreshToken() {
	if time.Now().After(c.tokenExpiry.Add(-TokenRefreshWindow)) {
		c.logger.Info().
			Time("current_expiry", c.tokenExpiry).
			Dur("refresh_window", TokenRefreshWindow).
			Msg("JWT token is nearing expiry, attempting refresh.")

		// Recreate the Payload using current client/room data
		payload := &jwt.Payload{
			ID:       c.user.ID,
			Code:     c.room.Code,
			UserType: c.user.UserType,
		}

		secretKey := c.room.JWTSecret

		// Generate the new token
		tokenString, err := jwt.GenerateToken(payload, secretKey, jwt.RoomAccessExpiration)
		if err != nil {
			c.logger.Error().Err(err).Msg("Failed to generate new token. Aborting refresh.")
			return
		}

		// Calculate the new token expiry time
		newExpiry := time.Now().Add(jwt.RoomAccessExpiration)

		// Update Client state and send the update message
		if err := c.SendTokenUpdateMessage(tokenString); err != nil {
			c.logger.Error().Err(err).Msg("Failed to send token update to client.")
			return
		}

		// Update the client's internal expiry record
		c.tokenExpiry = newExpiry
	}
}

// SendTokenUpdateMessage constructs and sends a TypeTokenUpdate message to the client.
func (c *Client) SendTokenUpdateMessage(newToken string) error {
	updatePayload := TokenUpdatePayload{
		Token: newToken,
	}

	updateMsg, err := NewMessage(
		TypeTokenUpdate,
		c.room.Code,
		SystemUser,
		updatePayload,
	)

	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to build TOKEN_UPDATE message.")
		return err
	}

	if err := c.sendMessage(updateMsg); err != nil {
		c.logger.Error().Err(err).Msg("Failed to send TOKEN_UPDATE message.")
		return err
	}
	return nil
}

// sendMessage marshals the data and attempts to send it to the client's send channel.
func (c *Client) sendMessage(data any) error {
	messageBytes, err := json.Marshal(data)
	if err != nil {
		c.logger.Error().Err(err).Msg("Error marshaling data for client")
		return err
	}

	select {
	case c.send <- messageBytes:
		return nil
	default:
		c.logger.Warn().Int("queue_len", len(c.send)).Msg("Client send channel full, dropping message")
		return fmt.Errorf("client send queue full")
	}
}

// SendError constructs and sends a TypeError message to the client.
func (c *Client) SendError(err error) {
	var code int
	var message string

	var customErr *errs.CustomError
	if errors.As(err, &customErr) {
		code = customErr.Code
		message = customErr.Message
	} else {
		code = errs.ErrUnknown
		message = fmt.Sprintf("Internal server error: %v", err)
	}

	errorPayload := ErrorPayload{
		Code:    code,
		Message: message,
	}

	errorMsg, msgErr := NewMessage(
		TypeError,
		c.room.Code,
		SystemUser,
		errorPayload,
	)

	if msgErr != nil {
		logx.Fatal(msgErr, "Failed to build error message in SendError")
		return
	}

	if err := c.sendMessage(errorMsg); err != nil {
		c.logger.Error().Err(err).Msg("Failed to queue error message")
	}
}

// SendInitData constructs and sends a TypeInitData message containing the initial room state information.
func (c *Client) SendInitData(payload InitDataPayload) error {
	initMsg, err := NewMessage(
		TypeInitData,
		c.room.Code,
		SystemUser,
		payload,
	)

	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to build INIT_DATA message.")
		return err
	}

	if err := c.sendMessage(initMsg); err != nil {
		c.logger.Error().Err(err).Msg("Failed to send INIT_DATA message.")
		return err
	}

	return nil
}

// sendConfirmation constructs and sends a TypeConfirm (ACK) message back to the sender.
func (c *Client) sendConfirmation(originalTempID string, authoritativeMsg Message) {
	if originalTempID == "" {
		return
	}

	ackPayload := struct {
		OriginalTempID string `json:"tempId"`
		MessageID      string `json:"id"`
		Timestamp      int64  `json:"timestamp"`
	}{
		OriginalTempID: originalTempID,
		MessageID:      authoritativeMsg.ID,
		Timestamp:      authoritativeMsg.Timestamp,
	}

	ackMsg, err := NewMessage(
		TypeConfirm,
		c.room.Code,
		c.user,
		ackPayload,
	)

	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to build ACK message in sendConfirmation")
		return
	}

	if err := c.sendMessage(ackMsg); err != nil {
		c.logger.Error().Err(err).Msg("Failed to queue ACK message")
	}
}

// Kick gracefully closes the client's connection by sending a custom WebSocket
// Close Frame (Code 4001) indicating that the session was replaced.
func (c *Client) Kick(reason string) {
	c.logger.Warn().
		Int("close_code", WsCloseCodeSessionKicked).
		Str("reason", reason).
		Msg("Sending WS Kick message and closing connection.")

	closeMessage := websocket.FormatCloseMessage(
		WsCloseCodeSessionKicked,
		reason,
	)

	c.conn.SetWriteDeadline(time.Now().Add(writeWait))

	if err := c.conn.WriteMessage(websocket.CloseMessage, closeMessage); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to send WS 4001 Close Message.")
	}

	select {
	case <-c.send:
	default:
		close(c.send)
	}
}
