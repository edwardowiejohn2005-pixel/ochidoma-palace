package models

import (
	"time"

	"github.com/google/uuid"
)

type Article struct {
	ID          uuid.UUID  `json:"id"`
	Section     string     `json:"section"` // heritage | history
	CategoryID  *int       `json:"category_id,omitempty"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Description *string    `json:"description,omitempty"`
	Body        string     `json:"body"`
	Sources     *string    `json:"sources,omitempty"`
	AuthorID    *uuid.UUID `json:"author_id,omitempty"`
	Status      string     `json:"status"`
	IsDemo      bool       `json:"is_demo_content"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Food struct {
	ID                    uuid.UUID `json:"id"`
	Name                  string    `json:"name"`
	LocalName             *string   `json:"local_name,omitempty"`
	Slug                  string    `json:"slug"`
	Description           *string   `json:"description,omitempty"`
	Ingredients           *string   `json:"ingredients,omitempty"`
	Preparation           *string   `json:"preparation,omitempty"`
	CulturalSignificance  *string   `json:"cultural_significance,omitempty"`
	Region                *string   `json:"region,omitempty"`
	Status                string    `json:"status"`
	IsDemo                bool      `json:"is_demo_content"`
	CreatedAt             time.Time `json:"created_at"`
}

type Event struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description *string    `json:"description,omitempty"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
	Location    *string    `json:"location,omitempty"`
	Organizer   *string    `json:"organizer,omitempty"`
	Category    *string    `json:"category,omitempty"`
	Status      string     `json:"status"` // upcoming | ongoing | completed
	IsDemo      bool       `json:"is_demo_content"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Announcement struct {
	ID              uuid.UUID  `json:"id"`
	ReferenceNumber *string    `json:"reference_number,omitempty"`
	Title           string     `json:"title"`
	Category        *string    `json:"category,omitempty"`
	Content         string     `json:"content"`
	Status          string     `json:"status"`
	IsOfficial      bool       `json:"is_official"`
	IsDemo          bool       `json:"is_demo_content"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}
