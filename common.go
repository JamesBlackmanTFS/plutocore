package shared

import (
	"fmt"
	"os"
	"time"

	"github.com/Shopify/sarama"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

//StartConsume initiates consumption of topic
func StartConsume(topic string, serviceConfig *ServiceConfig, f func(*sarama.ConsumerMessage, *Room, *gorm.DB)) {

	serviceConfig.ConsumerCount++
	/* define channels */
	doneCh := make(chan struct{})

	/* Consume Client Events */
	master, consumer, offset := OpenConsumer(topic)
	// initClients()
	go func(signals *chan os.Signal, sw *chan struct{}, f func(*sarama.ConsumerMessage, *Room, *gorm.DB), rm *Room, db *gorm.DB) {
		if offset == 0 {
			*sw <- struct{}{}
		}
		for {
			select {

			case err := <-consumer.Errors():
				fmt.Println(err)
				fmt.Println("ERROR CONSUMING TOPIC ", topic)

			case msg := <-consumer.Messages():
				// fmt.Println(topic, " - ", msg.Offset+1, " of ", offset)
				// rm.Forward <- msg.Key
				f(msg, rm, db)
				// *shared.MessageCountStart++
				// fmt.Printf("(%v) = %v\n", string(msg.Key), string(msg.Value))

				if msg.Offset+1 == offset {
					/* If we have consumed up to the offset send on the startWeb channel */
					fmt.Println("Finished consumption of ", topic, " - ", msg.Offset+1, " of ", offset)
					*sw <- struct{}{}
				}
			case <-*signals:
				fmt.Println("Interrupt is detected in StartConsume", topic)
				doneCh <- struct{}{}
				os.Exit(0)
			}
		}
	}(&serviceConfig.Signals, &serviceConfig.StartWeb, f, serviceConfig.Socket, serviceConfig.DB)

	<-doneCh /* Wait until interrupt */
	fmt.Println("Closed ", topic, " consumer")
	// consumer.Close()
	master.Close()
	time.Sleep(1 * time.Second)

}
