package models

import "time"

type EmailLog struct {
	ID        string      `json:"id" bson:"id"`
	Recipient string      `json:"recipient" bson:"recipient"`
	Subject   string      `json:"subject" bson:"subject"`
	Message   string      `json:"message" bson:"message"`
	SentAt    time.Time   `json:"sent_at" bson:"sent_at"`
	Active    *bool		  `json:"active"`
	AttachmentPath string `json:"attachment_path" bson:"attachment_path"`
}
