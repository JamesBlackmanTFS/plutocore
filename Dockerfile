FROM golang:1.11
RUN go get github.com/Shopify/sarama
RUN go get github.com/gorilla/websocket
RUN go get github.com/gorilla/mux
RUN go get github.com/jinzhu/gorm
RUN go get github.com/jinzhu/gorm/dialects/sqlite
RUN go get github.com/matryer/goblueprints/chapter1/trace
RUN go get gopkg.in/alecthomas/kingpin.v2
# docker build -t plutoimage:1.0 . -f Dockerfile
