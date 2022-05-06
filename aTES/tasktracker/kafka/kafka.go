package kafka

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"tasktracker/db"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
)

type Consumer struct {
	DBConn      db.Connection
	RoleChanges chan uuid.UUID
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
		log.Println("consumer failed", err)
		os.Exit(1)
	}

	defer c.Close()

	topicProcessors := map[string]func(msg *kafka.Message) error{
		"accounts":        consumer.processAccountsBEvts,
		"accounts-stream": consumer.processAccountsCUDs,
	}

	var topics []string
	for k := range topicProcessors {
		topics = append(topics, k)
	}

	err = c.SubscribeTopics(topics, nil)
	if err != nil {
		log.Println(err)
	}
	// Set up a channel for handling Ctrl-C, etc
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case sig := <-sigchan:
			log.Printf("Caught signal %v: terminating\n", sig)
			return
		default:
			ev, err := c.ReadMessage(time.Second * 10)
			if err != nil {
				continue
			}
			fmt.Printf("Consumed event from topic %s: key = %-10s value = %s\n",
				*ev.TopicPartition.Topic, string(ev.Key), string(ev.Value))
			if foo, ok := topicProcessors[*ev.TopicPartition.Topic]; ok {
				go func() {
					log.Println(foo(ev))
				}()
			}
		}
	}
}

func (c *Consumer) processAccountsBEvts(msg *kafka.Message) error {
	var payload Event
	err := json.Unmarshal(msg.Value, &payload)
	if err != nil {
		return err
	}

	switch payload.EventName {
	case "AccountRoleChanged":
		var data RoleChangePayload
		err := json.Unmarshal([]byte(msg.Value), &data)
		if err != nil {
			return err
		}
		acc, err := c.DBConn.GetAccount(data.Data.PublicID)
		if err != nil {
			return err
		}
		defer func() {
			if c.RoleChanges != nil {
				c.RoleChanges <- acc.PublicID
			}
		}()

		acc.Role.UnmarshalText(data.Data.Role)
		fmt.Println(data.Data.Role, acc.Role)
		return c.DBConn.SaveAccount(acc)
	default:
		log.Println("ignoring unknown event", payload.EventName)
	}

	return nil
}

func (c *Consumer) processAccountsCUDs(msg *kafka.Message) error {
	var payload Event
	err := json.Unmarshal(msg.Value, &payload)
	if err != nil {
		return err
	}

	switch payload.EventName {
	case "AccountUpdated":
		var data AccountUpdatePayload
		err := json.Unmarshal([]byte(msg.Value), &data)
		if err != nil {
			return err
		}
		acc, err := c.DBConn.GetAccount(data.Data.PublicID)
		if err != nil {
			return err
		}
		acc.Email = data.Data.Email
		acc.FullName = data.Data.FullName
		return c.DBConn.SaveAccount(acc)
	case "AccountCreated":
		var data AccountUpdatePayload
		err := json.Unmarshal([]byte(msg.Value), &data)
		if err != nil {
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
		log.Println("ignoring unknown event", payload.EventName)
	}

	return nil
}

type Event struct {
	EventID      string      `json:"event_id"`
	EventVersion int64       `json:"event_version"`
	EventTime    string      `json:"event_time"`
	Producer     string      `json:"producer"`
	EventName    string      `json:"event_name"`
	Data         interface{} `json:"data"`
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

type Producer struct {
	*kafka.Producer
	deliveryCh chan kafka.Event
}

func NewProducer() *Producer {
	conf := kafka.ConfigMap{
		"bootstrap.servers": "broker:9092",
		"client.id":         "localhost:3002",
		"acks":              "all",
	}
	pr, err := kafka.NewProducer(&conf)
	if err != nil {
		log.Println("consumer failed", err)
		os.Exit(1)
	}

	return &Producer{
		Producer:   pr,
		deliveryCh: make(chan kafka.Event, 10000),
	}
}

func (p *Producer) Run() {
	defer p.Producer.Close()

	go func() {
		for {
			for e := range p.Events() {
				switch ev := e.(type) {
				case *kafka.Message:
					if ev.TopicPartition.Error != nil {
						fmt.Printf("Failed to deliver message: %v\n", ev.TopicPartition)
					} else {
						fmt.Printf("Successfully produced record to topic %s partition [%d] @ offset %v\n",
							*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset)
					}
				}
			}
		}
	}()
}

func (p *Producer) TaskCreatedMsg(t db.Task) error {
	msg, err := getTaskEvt(t, "TaskCreated")
	if err != nil {
		return err
	}
	topic := "tasks-stream"
	return p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg},
		p.deliveryCh,
	)
}

func (p *Producer) TaskUpdatedMsg(t db.Task) error {
	msg, err := getTaskEvt(t, "TaskUpdated")
	if err != nil {
		return err
	}
	topic := "tasks-stream"
	return p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg},
		p.deliveryCh,
	)
}

func (p *Producer) TaskCompleted(t db.Task) error {
	msg, err := getTaskEvt(t, "TasCompleted")
	if err != nil {
		return err
	}
	topic := "tasks"
	return p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg},
		p.deliveryCh,
	)
}

func (p *Producer) TaskAssigned(t db.Task) error {
	msg, err := getTaskEvt(t, "TasAssigned")
	if err != nil {
		return err
	}
	topic := "tasks"
	return p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg},
		p.deliveryCh,
	)
}

func getTaskEvt(t db.Task, evtName string) ([]byte, error) {
	return json.Marshal(Event{
		EventID:      uuid.NewString(),
		EventVersion: 1,
		EventTime:    time.Now().String(),
		EventName:    evtName,
		Data:         t,
	})
}
