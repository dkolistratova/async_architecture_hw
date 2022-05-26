package producer

import (
	"encoding/json"
	"fmt"
	"os"
	"tasktracker/db"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/dkolistratova/eventschemaregistry"
	"github.com/google/uuid"
)

const (
	TaskEventsTopic = "tasks"
	TaskCUDsTopic   = "tasks-stream"

	TaskCreatedEvt = "TaskCreated"
	TaskUpdatedEvt = "TaskUpdated"
	TaskDeletedEvt = "TaskDeleted"

	TaskAssignedEvt  = "TaskAssigned"
	TaskCompletedEvt = "TaskCompleted"
)

type Producer struct {
	*kafka.Producer
	deliveryCh chan kafka.Event
	validator  *eventschemaregistry.Validator
}

func NewProducer() *Producer {
	conf := kafka.ConfigMap{
		"bootstrap.servers": "broker:29092",
		"client.id":         "localhost:3002",
		"acks":              "all",
	}

	pr, err := kafka.NewProducer(&conf)
	if err != nil {
		fmt.Println("NewProducer failed", err)
		os.Exit(1)
	}

	return &Producer{
		Producer:   pr,
		deliveryCh: make(chan kafka.Event, 10000),
		validator:  eventschemaregistry.NewValidator("/app/event_schema_registry/schemas"),
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

var validatorByEvtName = map[string]string{
	TaskAssignedEvt:  "tasks.assigned",
	TaskCompletedEvt: "tasks.completed",
	TaskCreatedEvt:   "tasks.created",
	TaskDeletedEvt:   "tasks.deleted",
	TaskUpdatedEvt:   "tasks.updated",
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
		fmt.Println("produceTaskEvt err", err)
		return err
	}
	fmt.Println("produceTaskEvt Validate")
	if err := p.validator.Validate(msg, validatorByEvtName[evtName], 1); err != nil {
		fmt.Println("produceTaskEvt validation failed", err)
		return err
	}
	fmt.Println("produceTaskEvt Produce")
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

func (p *Producer) TaskCompletedMsg(t db.Task) error {
	return p.produceTaskEvt(t, TaskEventsTopic, TaskCompletedEvt)
}

func (p *Producer) TaskAssignedMsg(t db.Task) error {
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
