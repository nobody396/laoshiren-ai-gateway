package service

import "time"

type IncidentCandidateState string

const (
	IncidentCandidateOpen      IncidentCandidateState = "open"
	IncidentCandidateConfirmed IncidentCandidateState = "confirmed"
	IncidentCandidateDismissed IncidentCandidateState = "dismissed"
	IncidentCandidateRecovered IncidentCandidateState = "recovered"
)

type IncidentControlSettings struct {
	Enabled       bool `json:"enabled"`
	PublicEnabled bool `json:"public_enabled"`
}

type AdminIncidentCandidateProduct struct {
	ProductCode            string        `json:"product_code"`
	ProductName            string        `json:"product_name"`
	LatestStatus           ServiceStatus `json:"latest_status"`
	FirstObservedAt        time.Time     `json:"first_observed_at"`
	LastObservedAt         time.Time     `json:"last_observed_at"`
	FirstCustomerFailureAt *time.Time    `json:"first_customer_failure_at,omitempty"`
}

type AdminIncidentCandidate struct {
	ID                  int64                           `json:"id"`
	State               IncidentCandidateState          `json:"state"`
	FirstObservedAt     time.Time                       `json:"first_observed_at"`
	LastObservedAt      time.Time                       `json:"last_observed_at"`
	ConfirmedIncidentID *int64                          `json:"confirmed_incident_id,omitempty"`
	DismissedReason     string                          `json:"dismissed_reason,omitempty"`
	Products            []AdminIncidentCandidateProduct `json:"products"`
}

type AdminIncidentSegment struct {
	ID                 int64      `json:"id"`
	StartedAt          time.Time  `json:"started_at"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	StartObservationID int64      `json:"start_observation_id,omitempty"`
	EndObservationID   int64      `json:"end_observation_id,omitempty"`
	DurationSeconds    int64      `json:"duration_seconds"`
}

type AdminIncidentProduct struct {
	ID                    int64                  `json:"id"`
	ProductCode           string                 `json:"product_code"`
	ProductName           string                 `json:"product_name"`
	CurrentStatus         ServiceStatus          `json:"current_status"`
	AffectedAt            time.Time              `json:"affected_at"`
	MonitoringSince       *time.Time             `json:"monitoring_since,omitempty"`
	RecoveredAt           *time.Time             `json:"recovered_at,omitempty"`
	LastCustomerFailureAt *time.Time             `json:"last_customer_failure_at,omitempty"`
	CompensableSeconds    int64                  `json:"compensable_seconds"`
	Segments              []AdminIncidentSegment `json:"segments"`
}

type AdminIncidentUpdate struct {
	ID              int64         `json:"id"`
	Phase           IncidentPhase `json:"phase"`
	Kind            string        `json:"kind"`
	InternalMessage string        `json:"internal_message"`
	CreatedByUserID *int64        `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
}

type AdminIncidentPublicUpdate struct {
	ID                int64         `json:"id"`
	Phase             IncidentPhase `json:"phase"`
	Message           string        `json:"message"`
	PublishedByUserID *int64        `json:"published_by_user_id,omitempty"`
	PublishedAt       time.Time     `json:"published_at"`
}

type AdminIncident struct {
	ID                      int64                       `json:"id"`
	PublicID                string                      `json:"public_id"`
	Phase                   IncidentPhase               `json:"phase"`
	Title                   string                      `json:"title"`
	InternalSummary         string                      `json:"internal_summary"`
	ObservationStartedAt    time.Time                   `json:"observation_started_at"`
	ObservationEndedAt      *time.Time                  `json:"observation_ended_at,omitempty"`
	CustomerImpactStartedAt *time.Time                  `json:"customer_impact_started_at,omitempty"`
	CustomerImpactEndedAt   *time.Time                  `json:"customer_impact_ended_at,omitempty"`
	MonitoringSince         *time.Time                  `json:"monitoring_since,omitempty"`
	ResolvedAt              *time.Time                  `json:"resolved_at,omitempty"`
	Version                 int64                       `json:"version"`
	EvidenceGap             bool                        `json:"evidence_gap"`
	Products                []AdminIncidentProduct      `json:"products"`
	Updates                 []AdminIncidentUpdate       `json:"updates"`
	PublicTimeline          []AdminIncidentPublicUpdate `json:"public_timeline"`
	CreatedAt               time.Time                   `json:"created_at"`
	UpdatedAt               time.Time                   `json:"updated_at"`
}

type AdminIncidentSnapshot struct {
	Settings    IncidentControlSettings  `json:"settings"`
	Candidates  []AdminIncidentCandidate `json:"candidates"`
	Incidents   []AdminIncident          `json:"incidents"`
	GeneratedAt time.Time                `json:"generated_at"`
}

type PublicIncidentUpdate struct {
	Phase       IncidentPhase `json:"phase"`
	Message     string        `json:"message"`
	PublishedAt time.Time     `json:"published_at"`
}

type PublicIncident struct {
	ID               string                 `json:"id"`
	Phase            IncidentPhase          `json:"phase"`
	StartedAt        time.Time              `json:"started_at"`
	ResolvedAt       *time.Time             `json:"resolved_at,omitempty"`
	AffectedProducts []string               `json:"affected_products"`
	Timeline         []PublicIncidentUpdate `json:"timeline"`
}

type PublicIncidentSnapshot struct {
	Enabled     bool             `json:"enabled"`
	GeneratedAt time.Time        `json:"generated_at"`
	Incidents   []PublicIncident `json:"incidents"`
}

type IncidentTransitionCommand struct {
	IncidentID  int64
	Target      IncidentPhase
	Message     string
	ActorUserID int64
	At          time.Time
}

type IncidentConfirmCommand struct {
	CandidateID     int64
	Title           string
	InternalSummary string
	ActorUserID     int64
	At              time.Time
}

type IncidentDismissCommand struct {
	CandidateID int64
	Reason      string
	ActorUserID int64
	At          time.Time
}

type IncidentUpdateCommand struct {
	IncidentID  int64
	Message     string
	ActorUserID int64
}

type IncidentPublishCommand struct {
	IncidentID  int64
	Message     string
	ActorUserID int64
}

type IncidentEvidenceGapCommand struct {
	IncidentID  int64
	Reason      string
	ActorUserID int64
}
