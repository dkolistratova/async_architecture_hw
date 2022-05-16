package producer

import (
	"billing/db"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/dkolistratova/eventschemaregistry"
	"github.com/google/uuid"
)

const (
	TxEventsTopic = "transactions"
	TxCUDsTopic   = "transactions-stream"

	TxCreatedEvt = "TransactionCreated"
	TxUpdatedEvt = "TransactionUpdated"

	TxAppliedEvt     = "TransactionApplied"
	TxPaymentDoneEvt = "TransactionPaymentDone"
)

var validatorByEvtName = map[string]string{
	TxCreatedEvt:     "transactions.created",
	TxUpdatedEvt:     "transactions.updated",
	TxAppliedEvt:     "transactions.applied",
	TxPaymentDoneEvt: "transactions.paymentdone",
}

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

func (p *Producer) produceEvt(topic, evtName string, evt interface{}) error {
	msg, err := json.Marshal(Event{
		EventID:      uuid.NewString(),
		EventVersion: 1,
		EventTime:    time.Now().String(),
		EventName:    evtName,
		Data:         evt,
	})
	if err != nil {
		fmt.Println("produceTxEvt err", err)
		return err
	}
	if err := p.validator.Validate(msg, validatorByEvtName[evtName], 1); err != nil {
		fmt.Println("produceTxEvt validation failed", err)
		return err
	}
	return p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg},
		p.deliveryCh,
	)
}

type Event struct {
	EventID      string      `json:"event_id"`
	EventVersion int64       `json:"event_version"`
	EventTime    string      `json:"event_time"`
	Producer     string      `json:"producer"`
	EventName    string      `json:"event_name"`
	Data         interface{} `json:"data"`
}

func (p *Producer) TxCreatedMsg(t db.Transaction) error {
	return p.produceEvt(TxCUDsTopic, TxCreatedEvt, t)
}

func (p *Producer) TxUpdatedMsg(t db.Transaction) error {
	return p.produceEvt(TxCUDsTopic, TxUpdatedEvt, t)
}

func (p *Producer) TxAppliedMsg(t db.Transaction) error {
	return p.produceEvt(TxEventsTopic, TxAppliedEvt, t)
}

func (p *Producer) PaymentDoneMsg(t db.Transaction) error {
	return p.produceEvt(TxEventsTopic, TxPaymentDoneEvt, t)
}
