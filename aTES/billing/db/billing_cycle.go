package db

import (
	"fmt"

	"github.com/google/uuid"
	pq "github.com/lib/pq"
	"gorm.io/gorm"
)

type BillingCycle struct {
	gorm.Model
	PublicID       uuid.UUID
	DaySum         int
	TransactionLog pq.StringArray `gorm:"type:text[]"`
}

func (c *Connection) GetBillingCycle(id string) (*BillingCycle, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse id failed: %w", err)
	}
	var bc BillingCycle
	res := c.Where(&BillingCycle{PublicID: uid}).First(&bc)
	if res.Error != nil {
		return nil, fmt.Errorf("get billing_cycle failed: %w", err)
	}
	return &bc, nil
}

func (c *Connection) GetAllBillingCycles() ([]BillingCycle, error) {
	allBillingCycles := []BillingCycle{}
	res := c.Find(&allBillingCycles)
	if res.Error != nil {
		return nil, fmt.Errorf("get all billing_cycle failed: %s", res.Error)
	}
	return allBillingCycles, nil
}

type UpdateBillingCycleReq struct {
	DaySum         *int
	TransactionLog pq.StringArray
}

func (c *Connection) UpdateBillingCycle(id string, req UpdateBillingCycleReq) (*BillingCycle, error) {
	t, err := c.GetBillingCycle(id)
	if err != nil {
		return nil, err
	}
	if req.DaySum != nil {
		t.DaySum = *req.DaySum
	}
	if req.TransactionLog != nil {
		t.TransactionLog = req.TransactionLog
	}
	return t, c.SaveBillingCycle(t)
}

func (c *Connection) SaveBillingCycle(t *BillingCycle) error {
	res := c.Save(t)
	if res.Error != nil {
		return fmt.Errorf("billing_cycle save failed: %s", res.Error)
	}
	return nil
}

func (c *Connection) CreateBillingCycle(bc *BillingCycle) error {
	lookup := BillingTask{PublicID: bc.PublicID}
	res := c.Find(&lookup)
	if res.Error != nil {
		return fmt.Errorf("get task failed: %s", res.Error)
	}
	if lookup.ID == 0 {
		res := c.Create(bc)
		if res.Error != nil {
			return fmt.Errorf("task create failed: %s", res.Error)
		}
	}
	return nil
}
