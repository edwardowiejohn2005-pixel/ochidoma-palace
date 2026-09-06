package models

import (
	"time"

	"github.com/google/uuid"
)

type Decree struct {
	ID             uuid.UUID `json:"id"`
	DecreeNumber   string    `json:"decree_number"`
	CurrentStatus  string    `json:"current_status"`
	IsDemoContent  bool      `json:"is_demo_content"`
	CreatedAt      time.Time `json:"created_at"`
}

type DecreeVersion struct {
	ID                  uuid.UUID  `json:"id"`
	DecreeID            uuid.UUID  `json:"decree_id"`
	VersionNumber       float64    `json:"version_number"`
	Title               string     `json:"title"`
	FullText            string     `json:"full_text"`
	IssuingAuthority    string     `json:"issuing_authority"`
	DateIssued          *time.Time `json:"date_issued,omitempty"`
	EffectiveDate       *time.Time `json:"effective_date,omitempty"`
	IntegrityHash       string     `json:"integrity_hash"`
	Status              string     `json:"status"`
	IsCorrection        bool       `json:"is_correction"`
	SupersedesVersionID *uuid.UUID `json:"supersedes_version_id,omitempty"`
	PublishedAt         *time.Time `json:"published_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}
