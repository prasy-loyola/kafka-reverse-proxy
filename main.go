package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func info(format string, args ...any) {
	log.Printf("INFO: "+format, args...)
}

func debug(format string, args ...any) {
	log.Printf("DEBUG: "+format, args...)
}

func logerror(format string, args ...any) {
	log.Printf("ERROR: "+format, args...)
}

type endpoint struct {
	name    string
	servers string
	topic   string
}

func (e endpoint) getProducerConfig() *kafka.ConfigMap {
	num := rand.Int()
	return &kafka.ConfigMap{
		"bootstrap.servers": e.servers,
		"client.id":         fmt.Sprintf("%s-producer-%d", e.name, num),
		"acks":              "all",
	}
}

func (e endpoint) getConsumerConfig() *kafka.ConfigMap {
	num := rand.Int()
	return &kafka.ConfigMap{
		"bootstrap.servers": e.servers,
		"client.id":         fmt.Sprintf("%s-consumer-%d", e.name, num),
		"group.id":          fmt.Sprintf("%s-group-%d", e.name, num),
		"auto.offset.reset": kafka.OffsetEnd.String(),
	}
}

type Config struct {
	backends map[string]string
	mapping  map[string]string
}

func forward(source endpoint, dest endpoint, channel chan kafka.Event) error {

	consumer, err := kafka.NewConsumer(source.getConsumerConfig())

	if err != nil {
		return err
	}
	producer, err := kafka.NewProducer(dest.getProducerConfig())
	if err != nil {
		return err
	}
	info("source: %+v, consumer: %+v", source)
	topics_info, err := consumer.GetMetadata(nil, true, 100)
	info("topics info %+v", topics_info)
	info("dest: %+v, producer: %+v", dest, producer)

	consumer.Subscribe(source.topic, nil)
	info("subscribing to %s", source.topic)

	run := true
	for run {
		e := consumer.Poll(1000)
		switch ev := e.(type) {
		case *kafka.Message:
			ev.TopicPartition.Topic = &dest.topic
			info("%v", *ev)
			producer.Produce(ev, channel)
		case *kafka.Error:
			logerror("Couldn't forward message from %s:%s->%s:%s", source.name, source.topic, dest.name, dest.topic)
			run = false
		default:
		}
	}

	return nil
}

func main() {
	config := Config{
		backends: map[string]string{
			"kafka2": "localhost:9095",
			"kafka1": "localhost:9093",
		},
		mapping: map[string]string{
			"kafka1:push-topic": "kafka2:push-topic",
			"kafka2:pull-topic": "kafka1:pull-topic",
		},
	}

	wait_chan := make(chan kafka.Event)

	for k, v := range config.mapping {

		sourcecfg := strings.Split(k, ":")
		destcfg := strings.Split(v, ":")

		source := endpoint{
			name:    sourcecfg[0],
			topic:   sourcecfg[1],
			servers: config.backends[sourcecfg[0]],
		}

		dest := endpoint{
			name:    destcfg[0],
			topic:   destcfg[1],
			servers: config.backends[destcfg[0]],
		}
		go forward(source, dest, wait_chan)
	}

	for {
		_ = <-wait_chan
	}
}
