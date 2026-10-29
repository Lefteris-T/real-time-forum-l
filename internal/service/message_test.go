package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"forum/internal/model"
	"forum/internal/validation"
)

type messageConversationFake struct {
	conversation model.Conversation
	err          error
	users        [2]int64
}

func (f *messageConversationFake) FindOrCreate(first, second int64) (model.Conversation, error) {
	f.users = [2]int64{first, second}
	return f.conversation, f.err
}

type messageWriterFake struct {
	message model.Message
	err     error
	args    [3]any
	called  bool
	image   model.MessageImage
}

func (f *messageWriterFake) CreateWithImage(conversationID, senderID int64, body string, image model.MessageImage) (model.Message, error) {
	f.called = true
	f.args = [3]any{conversationID, senderID, body}
	f.image = image
	f.message.ImageMIME = image.MIMEType
	return f.message, f.err
}

func (f *messageWriterFake) Create(conversationID, senderID int64, body string) (model.Message, error) {
	f.called = true
	f.args = [3]any{conversationID, senderID, body}
	return f.message, f.err
}

type messagePublisherFake struct {
	users   []int64
	types   []string
	payload []json.RawMessage
}

func (f *messagePublisherFake) PublishMessage(userID int64, eventType string, payload json.RawMessage) error {
	f.users = append(f.users, userID)
	f.types = append(f.types, eventType)
	f.payload = append(f.payload, append(json.RawMessage(nil), payload...))
	return nil
}

func TestMessageServicePersistsBeforePublishingToBothUsers(t *testing.T) {
	createdAt := time.Date(2026, 10, 29, 17, 0, 0, 0, time.UTC)
	conversations := &messageConversationFake{conversation: model.Conversation{ID: 21}}
	messages := &messageWriterFake{message: model.Message{ID: 8, ConversationID: 21, SenderID: 4, SenderUsername: "sender", Body: "hello", CreatedAt: createdAt}}
	events := &messagePublisherFake{}
	service := NewMessageService(conversations, messages, events)
	message, err := service.Send(4, 9, "  hello  ")
	if err != nil {
		t.Fatalf("Send(): %v", err)
	}
	if message.ID != 8 || conversations.users != [2]int64{4, 9} || !messages.called || messages.args != [3]any{int64(21), int64(4), "hello"} {
		t.Fatalf("message and dependency calls = %+v, %+v, %+v", message, conversations.users, messages.args)
	}
	if len(events.users) != 2 || events.users[0] != 9 || events.users[1] != 4 || events.types[0] != "message.new" || events.types[1] != "message.new" {
		t.Fatalf("published users/types = %v/%v", events.users, events.types)
	}
	var payload messageEventPayload
	if err := json.Unmarshal(events.payload[0], &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	if payload.Message.ID != 8 || payload.Message.Sender != "sender" || payload.Message.CreatedAt != createdAt.Format(time.RFC3339) {
		t.Fatalf("event payload = %+v", payload)
	}
}

func TestMessageServiceValidatesBeforePersistenceAndKeepsOfflineSendDurable(t *testing.T) {
	conversations := &messageConversationFake{conversation: model.Conversation{ID: 21}}
	messages := &messageWriterFake{message: model.Message{ID: 3, ConversationID: 21}}
	service := NewMessageService(conversations, messages, nil)
	for _, input := range []struct {
		sender, recipient int64
		body              string
		want              error
	}{
		{sender: 1, recipient: 1, body: "hello", want: ErrInvalidMessageRecipient},
		{sender: 1, recipient: 2, body: "  \n", want: ErrInvalidMessageBody},
		{sender: 1, recipient: 2, body: string(make([]rune, MaxPrivateMessageLength+1)), want: ErrInvalidMessageBody},
	} {
		if _, err := service.Send(input.sender, input.recipient, input.body); !errors.Is(err, input.want) {
			t.Fatalf("Send(%d, %d, body length %d) error = %v, want %v", input.sender, input.recipient, len(input.body), err, input.want)
		}
	}
	if messages.called {
		t.Fatal("invalid message reached persistence")
	}

	service = NewMessageService(conversations, messages, nil)
	if _, err := service.Send(1, 2, "offline recipient"); err != nil {
		t.Fatalf("Send() without live event publisher: %v", err)
	}
	if !messages.called {
		t.Fatal("message was not persisted when no recipient connection exists")
	}
}

func TestMessageServiceValidatesAndPublishesPrivateImageURL(t *testing.T) {
	imageBuffer := new(bytes.Buffer)
	imageData := image.NewRGBA(image.Rect(0, 0, 1, 1))
	imageData.Set(0, 0, color.RGBA{R: 20, G: 40, B: 60, A: 255})
	if err := png.Encode(imageBuffer, imageData); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	conversations := &messageConversationFake{conversation: model.Conversation{ID: 31}}
	messages := &messageWriterFake{message: model.Message{ID: 17, ConversationID: 31, SenderID: 2, SenderUsername: "sam", CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}}
	events := &messagePublisherFake{}
	service := NewMessageService(conversations, messages, events)
	message, err := service.SendWithImage(2, 3, "", imageBuffer.Bytes())
	if err != nil {
		t.Fatalf("SendWithImage(): %v", err)
	}
	if message.ImageMIME != "image/png" || messages.image.MIMEType != "image/png" || !bytes.Equal(messages.image.Data, imageBuffer.Bytes()) {
		t.Fatalf("stored image = %+v, message MIME %q", messages.image, message.ImageMIME)
	}
	var payload messageEventPayload
	if err := json.Unmarshal(events.payload[0], &payload); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if payload.Message.ImageURL != "/api/messages/17/image" {
		t.Fatalf("event image URL = %q", payload.Message.ImageURL)
	}
	if _, err := service.SendWithImage(2, 3, "", []byte("not an image")); !errors.Is(err, validation.ErrInvalidMessageImage) {
		t.Fatalf("invalid image error = %v", err)
	}
}
