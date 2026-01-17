package context

import "time"

// Message represents a single conversational turn.
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Conversation holds ordered messages.
type Conversation struct {
	Messages []Message `json:"messages"`
}

func (c *Conversation) Add(role, content string) {
	c.Messages = append(c.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().UTC(),
	})
}

func (c *Conversation) History() []Message {
	return c.Messages
}
