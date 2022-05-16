package db

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BillingTask struct {
	gorm.Model
	PublicID    uuid.UUID `json:"public_id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	AssignCost  int       `json:"assign_cost"`
	DoneCost    int       `json:"done_cost"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	CloseDay    time.Time `json:"close_day"`
}

type Status int

const (
	Status_NotDefined Status = 0
	Status_InProgress Status = 1
	Status_Done       Status = 3
)

func (status Status) String() string {
	switch status {
	case Status_InProgress:
		return "in progress"
	case Status_Done:
		return "done"
	default:
		return "not defined"
	}
}

func (status *Status) UnmarshalText(s string) {
	switch text := s; text {
	case "in progress":
		*status = Status_InProgress
	case "done":
		*status = Status_Done
	default:
		*status = Status_NotDefined
	}
}

func (c *Connection) GetBillingTask(id string) (*BillingTask, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse id failed: %w", err)
	}

	var task BillingTask
	res := c.Where(&BillingTask{PublicID: uid}).First(&task)
	if res.Error != nil {
		return nil, fmt.Errorf("get task failed: %w", err)
	}

	return &task, nil
}

func (c *Connection) GetAllBillingTasks() ([]BillingTask, error) {
	allBillingTasks := []BillingTask{}
	res := c.Find(&allBillingTasks)
	if res.Error != nil {
		return nil, fmt.Errorf("get all tasks failed: %s", res.Error)
	}
	return allBillingTasks, nil
}

type UpdateBillingTaskReq struct {
	OwnerID     *uuid.UUID
	AssignCost  *int
	DoneCost    *int
	Description string
	Status      *Status
}

func (c *Connection) UpdateBillingTask(id string, req UpdateBillingTaskReq) (*BillingTask, error) {
	t, err := c.GetBillingTask(id)
	if err != nil {
		return nil, err
	}

	if req.DoneCost != nil {
		t.DoneCost = *req.DoneCost
	}

	if req.AssignCost != nil {
		t.AssignCost = *req.AssignCost
	}

	if req.Status != nil {
		t.Status = *req.Status
	}

	if req.Description != "" {
		t.Description = req.Description
	}

	return t, c.SaveBillingTask(t)
}

func (c *Connection) SaveBillingTask(t *BillingTask) error {
	res := c.Save(t)
	if res.Error != nil {
		return fmt.Errorf("task save failed: %s", res.Error)
	}
	return nil
}

func (c *Connection) CreateBillingTask(t *BillingTask) error {
	lookup := BillingTask{PublicID: t.PublicID}
	res := c.Find(&lookup)
	if res.Error != nil {
		return fmt.Errorf("get task failed: %s", res.Error)
	}
	if lookup.ID == 0 {
		res := c.Create(t)
		if res.Error != nil {
			return fmt.Errorf("task create failed: %s", res.Error)
		}
	}
	return nil
}

const (
	maxAssignCost = 20
	minAssignCost = 10
	maxDoneCost   = 40
	minDoneCost   = 20
)

func (t *BillingTask) SetCosts() {
	t.AssignCost = rand.Intn(maxAssignCost-minAssignCost) + minAssignCost
	t.DoneCost = rand.Intn(maxDoneCost-minDoneCost) + minDoneCost
}

func (c *Connection) GetDoneBillingTasks() ([]BillingTask, error) {
	all := []BillingTask{}
	res := c.Where("status = ?", Status_Done).Find(&all)
	if res.Error != nil {
		return nil, fmt.Errorf("get done btasks by id failed: %s", res.Error)
	}
	return all, nil
}
