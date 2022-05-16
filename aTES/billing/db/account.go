package db

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	pq "github.com/lib/pq"
	"gorm.io/gorm"
)

type BillingAccount struct {
	gorm.Model
	PublicID       uuid.UUID
	Role           *Role
	Balance        int
	Email          string
	TransactionLog pq.StringArray `gorm:"type:text[]"`
}

type Role int

const (
	Role_Worker    Role = 0
	Role_Manager   Role = 1
	Role_Admin     Role = 2
	Role_Accounter Role = 3
)

func (r Role) String() string {
	switch r {
	case Role_Manager:
		return "manager"
	case Role_Admin:
		return "admin"
	case Role_Accounter:
		return "accounter"
	default:
		return "worker"
	}
}

func (r *Role) UnmarshalJSON(s []byte) error {
	r.UnmarshalText(string(s))
	return nil
}

func (r *Role) UnmarshalText(s string) {
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.TrimSpace(s)
	switch text := s; text {
	case "manager":
		*r = Role_Manager
	case "accounter":
		*r = Role_Accounter
	case "admin":
		*r = Role_Admin
	default:
		*r = Role_Worker
	}
}

func (c *Connection) GetAccount(id string) (*BillingAccount, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse id failed: %w", err)
	}

	var acc BillingAccount
	res := c.Where(&BillingAccount{PublicID: uid}).First(&acc)
	if res.Error != nil {
		if strings.Contains(res.Error.Error(), "record not found") {
			return &acc, nil
		}
		return nil, fmt.Errorf("get acc failed: %w", err)
	}

	return &acc, nil
}

func (c *Connection) GetAllAccounts() ([]BillingAccount, error) {
	allAccs := []BillingAccount{}
	res := c.Find(&allAccs)
	if res.Error != nil {
		return nil, fmt.Errorf("get all accs failed: %s", res.Error)
	}
	return allAccs, nil
}

type UpdateBillingAccountReq struct {
	Role           string
	Balance        *int
	Email          *string
	TransactionLog pq.StringArray
}

func (c *Connection) UpdateAccount(id string, req UpdateBillingAccountReq) (*BillingAccount, error) {
	acc, err := c.GetAccount(id)
	if err != nil {
		return nil, err
	}

	if req.Role != "" {
		acc.Role.UnmarshalText(req.Role)
	}

	if req.Balance != nil {
		acc.Balance = *req.Balance
	}

	if req.Email != nil {
		acc.Email = *req.Email
	}

	if req.TransactionLog != nil {
		acc.TransactionLog = req.TransactionLog
	}

	return acc, c.SaveAccount(acc)
}

func (c *Connection) SaveAccount(acc *BillingAccount) error {
	res := c.Save(acc)
	if res.Error != nil {
		return fmt.Errorf("acc save failed: %s", res.Error)
	}
	return nil
}

func (c *Connection) CreateAccount(acc *BillingAccount) error {
	lookup, err := c.GetAccount(acc.PublicID.String())
	if err != nil {
		return err
	}
	if lookup.ID == 0 {
		res := c.Create(acc)
		if res.Error != nil {
			return fmt.Errorf("acc create failed: %s", res.Error)
		}
	}
	return nil
}
