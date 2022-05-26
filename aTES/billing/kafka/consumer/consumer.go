package consumer

import (
	"billing/db"
	"billing/kafka/producer"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
)

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
	TaskCompletedEvt = "TaskCompleted"
)

type Consumer struct {
	DBConn              db.Connection
	RoleChangesNotifyer chan uuid.UUID
	Producer            producer.Producer
}

func NewConsumer(dbConn db.Connection, roleChangesNotifyer chan uuid.UUID, p producer.Producer) *Consumer {
	return &Consumer{
		DBConn:              dbConn,
		RoleChangesNotifyer: roleChangesNotifyer,
		Producer:            p,
	}
}

func (consumer *Consumer) Run() {
	conf := kafka.ConfigMap{
		"bootstrap.servers":     "broker:29092",
		"broker.address.family": "v4",
		"group.id":              2,
		"session.timeout.ms":    6000,
		"auto.offset.reset":     "earliest",
	}
	c, err := kafka.NewConsumer(&conf)
	if err != nil {
		fmt.Println("consumer failed", err)
		os.Exit(1)
	}

	defer c.Close()

	topicProcessors := map[string]func(msg *kafka.Message) error{
		AccountEventsTopic: consumer.processAccountEvts,
		AccountCUDsTopic:   consumer.processAccountsCUDs,
		TaskCUDsTopic:      consumer.processTaskCUDs,
		TaskEventsTopic:    consumer.processTaskEvts,
	}

	var topics []string
	for k := range topicProcessors {
		topics = append(topics, k)
	}

	if err := c.SubscribeTopics(topics, nil); err != nil {
		fmt.Println(err)
	}

	for {
		ev, err := c.ReadMessage(time.Second * 10)
		if err != nil {
			continue
		}
		fmt.Printf("Consumed event from topic %s: key = %-10s value = %s\n",
			*ev.TopicPartition.Topic, string(ev.Key), string(ev.Value))
		if processor, ok := topicProcessors[*ev.TopicPartition.Topic]; ok {
			go func() {
				fmt.Println(processor(ev))
			}()
		}
	}
}

func (c *Consumer) processAccountEvts(msg *kafka.Message) error {
	var payload Event
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return err
	}

	switch payload.EventName {
	case AccountRoleChangedEvt:
		var data RoleChangePayload
		if err := json.Unmarshal([]byte(msg.Value), &data); err != nil {
			return err
		}

		acc, err := c.DBConn.GetAccount(data.Data.PublicID)
		if err != nil {
			return err
		}

		defer func() {
			if c.RoleChangesNotifyer != nil {
				c.RoleChangesNotifyer <- acc.PublicID
			}
		}()

		acc.Role.UnmarshalText(data.Data.Role)
		return c.DBConn.SaveAccount(acc)
	default:
		fmt.Println("ignoring unknown event", payload.EventName)
	}

	return nil
}

func (c *Consumer) processAccountsCUDs(msg *kafka.Message) error {
	var payload Event
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return err
	}

	switch payload.EventName {
	case AccountUpdatedEvt:
		var data AccountUpdatePayload
		if err := json.Unmarshal([]byte(msg.Value), &data); err != nil {
			return err
		}

		acc, err := c.DBConn.GetAccount(data.Data.PublicID)
		if err != nil {
			return err
		}

		if acc.ID == 0 {
			uid, err := uuid.Parse(data.Data.PublicID)
			if err != nil {
				return err
			}

			return c.DBConn.CreateAccount(&db.BillingAccount{
				PublicID: uid,
				Email:    data.Data.Email,
			})
		}

		acc.Email = data.Data.Email
		return c.DBConn.SaveAccount(acc)
	case AccountCreatedEvt:
		var data AccountUpdatePayload
		if err := json.Unmarshal([]byte(msg.Value), &data); err != nil {
			return err
		}

		uid, err := uuid.Parse(data.Data.PublicID)
		if err != nil {
			return err
		}

		return c.DBConn.CreateAccount(&db.BillingAccount{
			PublicID: uid,
			Email:    data.Data.Email,
		})
	default:
		fmt.Println("ignoring unknown event", payload.EventName)
	}

	return nil
}

func (c *Consumer) processTaskCUDs(msg *kafka.Message) error {
	var payload Event
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return err
	}

	switch payload.EventName {
	case TaskUpdatedEvt:
		var data TaskUpdatePayload
		if err := json.Unmarshal([]byte(msg.Value), &data); err != nil {
			return err
		}

		task, err := c.DBConn.GetBillingTask(data.Data.PublicID)
		if err != nil {
			return err
		}

		task.Description = data.Data.Description
		return c.DBConn.SaveBillingTask(task)
	case TaskCreatedEvt:
		var data TaskUpdatePayload
		if err := json.Unmarshal([]byte(msg.Value), &data); err != nil {
			return err
		}

		uid, err := uuid.Parse(data.Data.PublicID)
		if err != nil {
			return err
		}

		t := &db.BillingTask{
			PublicID:    uid,
			OwnerID:     data.Data.OwnerID,
			Status:      db.Status(data.Data.Status),
			Description: data.Data.Description,
		}

		t.SetCosts()

		return c.DBConn.CreateBillingTask(t)
	default:
		fmt.Println("ignoring unknown event", payload.EventName)
	}

	return nil
}

func (c *Consumer) processTaskEvts(msg *kafka.Message) error {
	var payload Event
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return err
	}

	switch payload.EventName {
	case TaskAssignedEvt:
		var data TaskAssignedPayload
		if err := json.Unmarshal([]byte(msg.Value), &data); err != nil {
			return err
		}

		task, err := c.DBConn.GetBillingTask(data.Data.PublicID)
		if err != nil {
			return err
		}

		task.OwnerID = data.Data.OwnerID
		if err := c.DBConn.SaveBillingTask(task); err != nil {
			return err
		}

		tx := &db.Transaction{
			PublicID:    uuid.New(),
			OwnerID:     task.OwnerID,
			Cost:        task.AssignCost,
			Type:        db.TxType_Withdraw,
			Description: task.Description,
		}
		if err := c.DBConn.CreateTransaction(tx); err != nil {
			return err
		}

		if err := c.Producer.TxCreatedMsg(*tx); err != nil {
			log.Println("failext to publish!", err)
		}

		return c.processTX(tx)
	case TaskCompletedEvt:
		var data TaskCompletedPayload
		if err := json.Unmarshal([]byte(msg.Value), &data); err != nil {
			return err
		}

		task, err := c.DBConn.GetBillingTask(data.Data.PublicID)
		if err != nil {
			return err
		}

		task.Status = db.Status_Done
		task.CloseDay = time.Now().Truncate(time.Hour * 24)
		if err := c.DBConn.SaveBillingTask(task); err != nil {
			return err
		}

		tx := &db.Transaction{
			PublicID:    uuid.New(),
			OwnerID:     task.OwnerID,
			Cost:        task.DoneCost,
			Type:        db.TxType_Add,
			Description: task.Description,
		}
		if err := c.DBConn.CreateTransaction(tx); err != nil {
			return err
		}
		if err := c.Producer.TxCreatedMsg(*tx); err != nil {
			log.Println("failext to publish!", err)
		}
		return c.processTX(tx)
	default:
		fmt.Println("ignoring unknown event", payload.EventName)
	}

	return nil
}

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

func (c *Consumer) processTX(tx *db.Transaction) error {
	bcs, err := c.DBConn.GetAllBillingCycles()
	if err != nil {
		return err
	}
	bc := bcs[0]

	switch tx.Type {
	case db.TxType_Add, db.TxType_Withdraw:
		owner, err := c.DBConn.GetAccount(tx.OwnerID.String())
		if err != nil {
			return err
		}

		if tx.Type == db.TxType_Withdraw {
			owner.Balance -= tx.Cost
			bc.DaySum -= tx.Cost
		} else {
			owner.Balance += tx.Cost
			bc.DaySum += tx.Cost
		}

		owner.TransactionLog = append(owner.TransactionLog, tx.PublicID.String())
		if err := c.DBConn.SaveAccount(owner); err != nil {
			return err
		}
	case db.TxType_MakePayment:
		users, err := c.DBConn.GetAllAccounts()
		if err != nil {
			return err
		}

		for _, user := range users {
			user.TransactionLog = append(user.TransactionLog, tx.PublicID.String())
			if err := c.DBConn.SaveAccount(&user); err != nil {
				return err
			}
		}
	}

	bc.TransactionLog = append(bc.TransactionLog, tx.PublicID.String())
	if err := c.DBConn.SaveBillingCycle(&bc); err != nil {
		return err
	}

	if err := c.Producer.TxAppliedMsg(*tx); err != nil {
		log.Println("failext to publish!", err)
	}

	return nil
}
