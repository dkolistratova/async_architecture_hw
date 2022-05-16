package db

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Task struct {
	gorm.Model
	PublicID    uuid.UUID `json:"public_id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	Status      Status    `json:"status"`
	Description string    `json:"description"`
	Title       string    `json:"title"`
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

func (c *Connection) GetTask(id string) (*Task, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse id failed: %w", err)
	}

	var task Task
	res := c.Where(&Task{PublicID: uid}).First(&task)
	if res.Error != nil {
		return nil, fmt.Errorf("get task failed: %w", err)
	}

	return &task, nil
}

func (c *Connection) GetAllTasks() ([]Task, error) {
	allTasks := []Task{}
	res := c.Find(&allTasks)
	if res.Error != nil {
		return nil, fmt.Errorf("get all tasks failed: %s", res.Error)
	}
	return allTasks, nil
}

func (c *Connection) GetAllDoneTasks() ([]Task, error) {
	all := []Task{}
	res := c.Where("status = ?", Status_Done).Find(&all)
	if res.Error != nil {
		return nil, fmt.Errorf("get done tasks by id failed: %s", res.Error)
	}
	return all, nil
}

func (c *Connection) UpdateTask(id, descr, status string) (*Task, error) {
	t, err := c.GetTask(id)
	if err != nil {
		return nil, err
	}

	if status != "" {
		t.Status.UnmarshalText(status)
	}

	if descr != "" {
		t.Description = descr
	}

	return t, c.SaveTask(t)
}

func (c *Connection) SaveTask(t *Task) error {
	res := c.Save(t)
	if res.Error != nil {
		return fmt.Errorf("task save failed: %s", res.Error)
	}
	return nil
}
