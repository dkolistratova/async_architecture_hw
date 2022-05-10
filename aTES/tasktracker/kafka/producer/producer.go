package producer

import (
	"encoding/json"
	"log"
	"os"
	"tasktracker/db"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
)

const (
	TaskEventsTopic = "tasks"
	TaskCUDsTopic   = "tasks-stream"

	TaskCreatedEvt = "TaskCreated"
	TaskUpdatedEvt = "TaskUpdated"
	TaskDeletedEvt = "TaskDeleted"

	TaskAssignedEvt  = "TaskAssigned"
	TaskCompletedEvt = "TaskCompeleted"
)

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
						log.Printf("Failed to deliver message: %v\n", ev.TopicPartition)
					} else {
						log.Printf("Successfully produced record to topic %s partition [%d] @ offset %v\n",
							*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset)
					}
				}
			}
		}
	}()
}

func (p *Producer) produceTaskEvt(t db.Task, topic, evtName string) error {
	msg, err := json.Marshal(Event{
		EventID:      uuid.NewString(),
		EventVersion: 1,
		EventTime:    time.Now().String(),
		EventName:    evtName,
		Data:         t,
	})
	if err != nil {
		return err
	}
	return p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg},
		p.deliveryCh,
	)
}

func (p *Producer) TaskCreatedMsg(t db.Task) error {
	return p.produceTaskEvt(t, TaskCUDsTopic, TaskCreatedEvt)
}

func (p *Producer) TaskUpdatedMsg(t db.Task) error {
	return p.produceTaskEvt(t, TaskCUDsTopic, TaskUpdatedEvt)
}

func (p *Producer) TaskDeletedMsg(t db.Task) error {
	return p.produceTaskEvt(t, TaskCUDsTopic, TaskDeletedEvt)
}

func (p *Producer) TaskCompleted(t db.Task) error {
	return p.produceTaskEvt(t, TaskEventsTopic, TaskCompletedEvt)
}

func (p *Producer) TaskAssigned(t db.Task) error {
	return p.produceTaskEvt(t, TaskEventsTopic, TaskAssignedEvt)
}

type Event struct {
	EventID      string      `json:"event_id"`
	EventVersion int64       `json:"event_version"`
	EventTime    string      `json:"event_time"`
	Producer     string      `json:"producer"`
	EventName    string      `json:"event_name"`
	Data         interface{} `json:"data"`
}
