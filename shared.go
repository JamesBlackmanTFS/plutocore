package shared

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/Shopify/sarama"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

type ServiceConfig struct {
	Port          string
	ConsumerCount int
	StartWeb      chan struct{}
	Signals       chan os.Signal
	Socket        *Room
	DB            *gorm.DB
}
type Basemethods struct {
	Count  func() int
	Search func(string, int, int) (string, int)
	Delete func(string)
}

var (
	brokerList = kingpin.Flag("brokerList", "List of brokers to connect").Default("pluto_kafka:9092").Strings()
	maxRetry   = kingpin.Flag("maxRetry", "Retry limit").Default("5").Int()
	partition  = kingpin.Flag("partition", "Partition number").Default("0").String()
	offsetType = kingpin.Flag("offsetType", "Offset Type (OffsetNewest | OffsetOldest)").Default("-1").Int()
	//MessageCountStart holds consumption offset
	MessageCountStart = kingpin.Flag("messageCountStart", "Message counter start from:").Int64()
)

// NewUUID generates a random UUID according to RFC 4122
func NewUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

//OpenConsumer allows other libraries to use this
func OpenConsumer(topic string) (sarama.Consumer, sarama.PartitionConsumer, int64) {

	kingpin.Parse()
	fmt.Println("Consuming topic ", topic, " from broker ", *brokerList)
	config := sarama.NewConfig()
	//config.Version = sarama.V0_11_0_2
	config.Version = sarama.MaxVersion
	config.Consumer.Return.Errors = true
	brokers := *brokerList

	client, err := sarama.NewClient(brokers, config)
	offset, err := client.GetOffset(topic, 0, sarama.OffsetNewest)
	fmt.Printf(topic+" OFFSET = %v\n", offset)

	master, err := sarama.NewConsumer(brokers, config)

	if err != nil {
		panic(err)
	}
	// defer func() {
	// 	if err := master.Close(); err != nil {
	// 		panic(err)
	// 	}
	// }()

	consumer, err := master.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		fmt.Println("I PANIC 2")
		panic(err)
	}
	return master, consumer, offset
}

//EnableCors allows cross domain (shouldn't be required after nginx proxy)
func EnableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Expose-Headers", "List-Length")
}

//StreamEvent pushes event onto kafka
func StreamEvent(topic string, key string, jsonData []byte) error {
	kingpin.Parse()
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = *maxRetry
	config.Producer.Return.Successes = true
	config.Version = sarama.MaxVersion

	producer, err := sarama.NewSyncProducer(*brokerList, config)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			panic(err)
		}
	}()
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(jsonData),
		Key:   sarama.StringEncoder(key),
	}
	// fmt.Printf("\nMessage is [%v]\n", msg)
	_, _, err = producer.SendMessage(msg)
	if err != nil {
		return (err)
	}
	return nil
	// fmt.Printf("Message is stored in topic(%s)/partition(%d)/offset(%d)\n", topic, partition, offset)
}

func defaultRoute(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello")
}

//MainStartHTTPServer - handles initialization of all http servers
func MainStartHTTPServer(port string, signals *chan os.Signal, rt *mux.Router, f *Basemethods, rm *Room) {
	fmt.Println("STARTING WEB SERVICE ON PORT", port)

	rt.HandleFunc("/socket", func(w http.ResponseWriter, r *http.Request) {
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		socket, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal("ServeHTTP:", err)
			return
		}
		client := &client{
			socket: socket,
			send:   make(chan []byte, messageBufferSize),
			room:   rm,
		}
		rm.join <- client
		defer func() { rm.leave <- client }()
		go client.write()
		client.read()
	})
	rt.HandleFunc("/", defaultRoute)
	rt.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strconv.Itoa(f.Count()))
	})
	rt.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		EnableCors(&w)
		queryValues := r.URL.Query()
		id := queryValues.Get("id")
		f.Delete(id)
		io.WriteString(w, "Deleted item")
	})
	rt.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		EnableCors(&w)

		queryValues := r.URL.Query()
		skip := queryValues.Get("skip")
		searchterm := strings.ToLower(queryValues.Get("searchterm"))
		pagesize := queryValues.Get("pagesize")

		skipInt, errInt := strconv.Atoi(skip)
		if errInt != nil {
			fmt.Println("OOOPS")
			skipInt = 0
		}
		pageSizeInt, errPageSize := strconv.Atoi(pagesize)
		if errPageSize != nil {
			fmt.Println("OOOPS")
			pageSizeInt = 5
		}
		w.Header().Set("Content-Type", "application/json")
		jsonData, listLength := f.Search(searchterm, skipInt, pageSizeInt)
		w.Header().Set("List-Length", strconv.Itoa(listLength))
		io.WriteString(w, string(jsonData))
	})
	srv := &http.Server{
		Addr:    port,
		Handler: rt,
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %s", err)
		}
	}()
	select {
	case <-*signals:
		fmt.Println("server going to shut down")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			fmt.Println(err)
		}
	}
}

//WaitForConsumers - blocks until all consumers are consumed to offset
func WaitForConsumers(serviceConfig *ServiceConfig) {
	consumersReady := false
	for consumersReady == false {
		select {
		case <-serviceConfig.StartWeb:
			serviceConfig.ConsumerCount--
			if serviceConfig.ConsumerCount == 0 {
				fmt.Println("ALL CONSUMERS UP TO DATE")
				consumersReady = true
				// defer close(serviceConfig.StartWeb)
			}
		}
	}
}

//CreateServiceConfig - Config
func CreateServiceConfig(port string, logMode bool) *ServiceConfig {
	s := ServiceConfig{
		Port:          port,
		ConsumerCount: 0,
		StartWeb:      make(chan struct{}),
		Signals:       make(chan os.Signal, 1),
	}
	s.Socket = NewRoom()
	go s.Socket.Run()

	var err error
	// os.Remove("./foo.db")
	s.DB, err = gorm.Open("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		log.Fatal(err)
	}

	//Experiment with this ...
	s.DB.DB().SetMaxOpenConns(1)
	s.DB.LogMode(logMode)

	signal.Notify(s.Signals, os.Interrupt)
	return &s
}
