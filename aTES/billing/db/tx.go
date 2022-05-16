package db

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model
	PublicID    uuid.UUID `json:"public_id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	Cost        int       `json:"cost"`
	Type        TxType    `json:"type"`
	Status      TxStatus  `json:"status"`
	Description string    `json:"description"`
}

type TxType int

const (
	TxType_Unknown     TxType = 0
	TxType_Add         TxType = 1
	TxType_Withdraw    TxType = 2
	TxType_MakePayment TxType = 3
)

func (t TxType) String() string {
	switch t {
	case TxType_Add:
		return "add money"
	case TxType_Withdraw:
		return "withdraw money"
	case TxType_MakePayment:
		return "make payment"
	default:
		return "underfined"
	}
}

type TxStatus int

const (
	TxStatus_NotDone TxStatus = 0
	TxStatus_Success TxStatus = 1
	TxStatus_Failed  TxStatus = 2
)

func (status TxStatus) String() string {
	switch status {
	case TxStatus_Failed:
		return "tx filed"
	case TxStatus_Success:
		return "tx succeed"
	default:
		return "not done"
	}
}

func (c *Connection) GetTransaction(id string) (*Transaction, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse id failed: %w", err)
	}

	var tx Transaction
	res := c.Where(&Transaction{PublicID: uid}).First(&tx)
	if res.Error != nil {
		return nil, fmt.Errorf("get tx failed: %w", err)
	}

	return &tx, nil
}

func (c *Connection) GetAllTransactions() ([]Transaction, error) {
	allTransactions := []Transaction{}
	res := c.Find(&allTransactions)
	if res.Error != nil {
		return nil, fmt.Errorf("get all tasks failed: %s", res.Error)
	}
	return allTransactions, nil
}

type UpdateTransactionReq struct {
	Cost        *int
	Type        *TxType
	Status      *TxStatus
	Description string
}

func (c *Connection) UpdateTransaction(id string, req UpdateTransactionReq) (*Transaction, error) {
	t, err := c.GetTransaction(id)
	if err != nil {
		return nil, err
	}

	if req.Cost != nil {
		t.Cost = *req.Cost
	}

	if req.Type != nil {
		t.Type = *req.Type
	}

	if req.Status != nil {
		t.Status = *req.Status
	}

	if req.Description != "" {
		t.Description = req.Description
	}

	return t, c.SaveTransaction(t)
}

func (c *Connection) SaveTransaction(t *Transaction) error {
	res := c.Save(t)
	if res.Error != nil {
		return fmt.Errorf("tx save failed: %s", res.Error)
	}
	return nil
}

func (c *Connection) GetAllUserTransactions(uids []string) ([]Transaction, error) {
	allTransactions := []Transaction{}
	res := c.Where("public_id IN ?", uids).Find(&allTransactions)
	if res.Error != nil {
		return nil, fmt.Errorf("get all txes by id failed: %s", res.Error)
	}
	return allTransactions, nil
}

func (c *Connection) CreateTransaction(tx *Transaction) error {
	lookup := Transaction{PublicID: tx.PublicID}
	res := c.Find(&lookup)
	if res.Error != nil {
		return fmt.Errorf("get Transaction failed: %s", res.Error)
	}
	if lookup.ID == 0 {
		res := c.Create(tx)
		if res.Error != nil {
			return fmt.Errorf("Transaction create failed: %s", res.Error)
		}
	}
	return nil
}
