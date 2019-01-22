## Build stage
FROM golang:1.9.4 as builder

# Set the working directory to the app directory
WORKDIR /go/src/captureorderfd

# Install godeps
RUN go get -u -v github.com/astaxie/beego
RUN go get -u -v github.com/beego/bee
RUN go get -d github.com/Microsoft/ApplicationInsights-Go/appinsights
RUN go get -u -v gopkg.in/mgo.v2
RUN go get -u -v github.com/Azure/go-autorest/autorest/utils
RUN go get -u -v github.com/streadway/amqp
RUN go get -u -v pack.ag/amqp
RUN go get gopkg.in/matryer/try.v1

# Copy the application files
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o captureorderfd .

## App stage
FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/src/captureorderfd .

# Define environment variables

# PLEASE DO NOT OVERRIDE UNLESS INSTRUCTED BY PROCTORS
ENV CHALLENGEAPPINSIGHTS_KEY=0e90ab6f-79ee-466b-a1e7-fe469a0767da

# Challenge Logging
ENV TEAMNAME=

# Mongo/Cosmos
ENV MONGOHOST=
ENV MONGOUSER=
ENV MONGOPASSWORD=

# Expose the application on port 8080
EXPOSE 8080

# Set the entry point of the container to the bee command that runs the
# application and watches for changes
CMD ["./captureorderfd", "run"]