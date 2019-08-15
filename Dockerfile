FROM golang:1.11
RUN go get github.com/Shopify/sarama
RUN go get github.com/gorilla/websocket
RUN go get github.com/gorilla/mux
RUN go get github.com/jinzhu/gorm
RUN go get github.com/jinzhu/gorm/dialects/sqlite

# docker build -t plutocore:latest . -f Dockerfile
