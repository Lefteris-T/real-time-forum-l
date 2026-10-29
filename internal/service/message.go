package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"forum/internal/model"
	"forum/internal/validation"
)

const MaxPrivateMessageLength = 2000

var (
	ErrInvalidMessageRecipient = errors.New("message recipient is invalid")
	ErrInvalidMessageBody      = errors.New("message body is invalid")
)

type MessageConversationFinder interface {
	FindOrCreate(userA, userB int64) (model.Conversation, error)
}

type MessageWriter interface {
	Create(conversationID, senderID int64, body string) (model.Message, error)
}

type ImageMessageWriter interface {
	CreateWithImage(conversationID, senderID int64, body string, image model.MessageImage) (model.Message, error)
}

type MessageEventPublisher interface {
	PublishMessage(userID int64, eventType string, payload json.RawMessage) error
}

type messageEvent struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	SenderID       int64  `json:"sender_id"`
	Sender         string `json:"sender"`
	Body           string `json:"body"`
	CreatedAt      string `json:"created_at"`
	ImageURL       string `json:"image_url,omitempty"`
}

type messageEventPayload struct {
	Message messageEvent `json:"message"`
}

// MessageService validates, persists, then best-effort publishes a private
// message. A disconnected recipient can retrieve it from history later.
type MessageService struct {
	conversations MessageConversationFinder
	messages      MessageWriter
	events        MessageEventPublisher
}

func NewMessageService(conversations MessageConversationFinder, messages MessageWriter, events MessageEventPublisher) *MessageService {
	return &MessageService{conversations: conversations, messages: messages, events: events}
}

func (s *MessageService) Send(senderID, recipientID int64, body string) (model.Message, error) {
	return s.SendWithImage(senderID, recipientID, body, nil)
}

func (s *MessageService) SendWithImage(senderID, recipientID int64, body string, imageData []byte) (model.Message, error) {
	if senderID <= 0 {
		return model.Message{}, ErrAuthenticationRequired
	}
	if recipientID <= 0 || recipientID == senderID {
		return model.Message{}, ErrInvalidMessageRecipient
	}
	body = strings.TrimSpace(body)
	if !utf8.ValidString(body) || utf8.RuneCountInString(body) > MaxPrivateMessageLength || (utf8.RuneCountInString(body) == 0 && len(imageData) == 0) {
		return model.Message{}, ErrInvalidMessageBody
	}
	var image model.MessageImage
	if len(imageData) > 0 {
		validatedImage, err := validation.ValidatePrivateMessageImage(imageData)
		if err != nil {
			return model.Message{}, err
		}
		image = validatedImage
	}
	conversation, err := s.conversations.FindOrCreate(senderID, recipientID)
	if err != nil {
		return model.Message{}, err
	}
	var message model.Message
	if len(imageData) > 0 {
		imageWriter, ok := s.messages.(ImageMessageWriter)
		if !ok {
			return model.Message{}, errors.New("message image storage is unavailable")
		}
		message, err = imageWriter.CreateWithImage(conversation.ID, senderID, body, image)
	} else {
		message, err = s.messages.Create(conversation.ID, senderID, body)
	}
	if err != nil {
		return model.Message{}, err
	}
	eventMessage := messageEvent{
		ID: message.ID, ConversationID: message.ConversationID, SenderID: message.SenderID,
		Sender: message.SenderUsername, Body: message.Body, CreatedAt: message.CreatedAt.Format(time.RFC3339),
	}
	if message.ImageMIME != "" {
		eventMessage.ImageURL = "/api/messages/" + strconv.FormatInt(message.ID, 10) + "/image"
	}
	messagePayload, _ := json.Marshal(messageEventPayload{Message: eventMessage})
	if s.events != nil {
		// Delivery is intentionally best effort: persistence already succeeded.
		_ = s.events.PublishMessage(recipientID, "message.new", messagePayload)
		_ = s.events.PublishMessage(senderID, "message.new", messagePayload)
	}
	return message, nil
}
