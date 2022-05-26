package consumer

import (
	"encoding/json"
	"fmt"
	"os"
	"tasktracker/db"
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
)

type Consumer struct {
	DBConn              db.Connection
	RoleChangesNotifyer chan uuid.UUID
}

func NewConsumer(dbConn db.Connection, roleChangesNotifyer chan uuid.UUID) *Consumer {
	return &Consumer{
		DBConn:              dbConn,
		RoleChangesNotifyer: roleChangesNotifyer,
	}
}

func (consumer *Consumer) Run() {
	conf := kafka.ConfigMap{
		"bootstrap.servers":     "broker:29092",
		"broker.address.family": "v4",
		"group.id":              1,
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

			return c.DBConn.CreateAccount(&db.JiraAccount{
				PublicID: uid,
				Email:    data.Data.Email,
				FullName: data.Data.FullName,
			})
		}

		acc.Email = data.Data.Email
		acc.FullName = data.Data.FullName
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

		return c.DBConn.CreateAccount(&db.JiraAccount{
			PublicID: uid,
			Email:    data.Data.Email,
			FullName: data.Data.FullName,
		})
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
