package kafka

import "github.com/google/uuid"

const (
	AccountEventsTopic = "accounts"
	AccountCUDsTopic   = "accounts-stream"

	AccountCreatedEvt = "AccountCreated"
	AccountUpdatedEvt = "AccountUpdated"
	AccountDeletedEvt = "AccountDeleted"

	AccountRoleChangedEvt = "AccountRoleChanged"

	TaskEventsTopic = "tasks"
	TaskCUDsTopic   = "tasks-stream"

	TaskCreatedEvt = "TaskCreated"
	TaskUpdatedEvt = "TaskUpdated"
	TaskDeletedEvt = "TaskDeleted"

	TaskAssignedEvt  = "TaskAssigned"
	TaskCompletedEvt = "TaskCompeleted"
)

type Event struct {
	EventID      string `json:"event_id"`
	EventVersion int64  `json:"event_version"`
	EventTime    string `json:"event_time"`
	Producer     string `json:"producer"`
	EventName    string `json:"event_name"`
}

type AccountUpdatePayload struct {
	Data struct {
		PublicID string `json:"public_id"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Position string `json:"position"`
	} `json:"data"`
}

type RoleChangePayload struct {
	Data struct {
		PublicID string `json:"public_id"`
		Role     string `json:"role"`
	} `json:"data"`
}

type TaskUpdatePayload struct {
	Data struct {
		PublicID    string    `json:"public_id"`
		OwnerID     uuid.UUID `json:"owner_id"`
		Description string    `json:"description"`
		Status      int       `json:"status"`
	} `json:"data"`
}

type TaskAssignedPayload struct {
	Data struct {
		PublicID string    `json:"public_id"`
		OwnerID  uuid.UUID `json:"owner_id"`
	} `json:"data"`
}

type TaskCompletedPayload struct {
	Data struct {
		PublicID    string    `json:"public_id"`
		OwnerID     uuid.UUID `json:"owner_id"`
		Description string    `json:"description"`
	} `json:"data"`
}
