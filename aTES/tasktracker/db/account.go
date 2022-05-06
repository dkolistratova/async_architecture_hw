package db

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JiraAccount struct {
	gorm.Model
	PublicID uuid.UUID `json:"public_id"`
	Role     *Role     `json:"role"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
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

func (c *Connection) GetAccount(id string) (*JiraAccount, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse id failed: %w", err)
	}

	acc := JiraAccount{PublicID: uid}
	res := c.Find(&acc)
	fmt.Println(acc)
	if res.Error != nil {
		return nil, fmt.Errorf("get acc failed: %w", err)
	}

	return &acc, nil
}

func (c *Connection) GetAllAccounts() ([]JiraAccount, error) {
	allAccs := []JiraAccount{}
	res := c.Find(&allAccs)
	fmt.Println(allAccs)
	if res.Error != nil {
		return nil, fmt.Errorf("get all accs failed: %s", res.Error)
	}
	return allAccs, nil
}

func (c *Connection) UpdateAccount(id, role, email string) (*JiraAccount, error) {
	acc, err := c.GetAccount(id)
	if err != nil {
		return nil, err
	}

	if role != "" {
		acc.Role.UnmarshalText(role)
	}

	if email != "" {
		acc.Email = email
	}

	return acc, c.SaveAccount(acc)
}

func (c *Connection) SaveAccount(acc *JiraAccount) error {
	log.Println("saving acc", acc)
	res := c.Save(acc)
	if res.Error != nil {
		return fmt.Errorf("acc save failed: %s", res.Error)
	}
	return nil
}

func (c *Connection) CreateAccount(acc *JiraAccount) error {
	accLookup := JiraAccount{PublicID: acc.PublicID}
	res := c.Find(&acc)
	if res.Error != nil {
		return fmt.Errorf("get acc failed: %s", res.Error)
	}
	if accLookup.ID == 0 {
		res := c.Create(acc)
		if res.Error != nil {
			return fmt.Errorf("acc create failed: %s", res.Error)
		}
	}
	return nil
}
