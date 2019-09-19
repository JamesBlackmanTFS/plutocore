FROM golang:1.11
RUN go get github.com/Shopify/sarama
RUN go get github.com/gorilla/websocket
RUN go get github.com/gorilla/mux
RUN go get github.com/jinzhu/gorm
RUN go get github.com/jinzhu/gorm/dialects/sqlite
RUN go get github.com/jinzhu/gorm/dialects/postgres
RUN go get github.com/satori/go.uuid
RUN go get github.com/dangkaka/go-kafka-avro

# docker build -t plutocore:latest . -f Dockerfile
