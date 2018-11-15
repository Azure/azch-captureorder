package models

import (
	"crypto/tls"
	"net"
	"net/url"
	"context"
	"fmt"

	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
    "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	amqp10 "pack.ag/amqp"
	"gopkg.in/matryer/try.v1"
)

// Order represents the order json
type Order struct {
	ID           			bson.ObjectId		`json:"id" bson:"_id,omitempty"`
	OrderID           string  				`json:"orderId"`
	EmailAddress      string  				`json:"emailAddress"`
	Product           string  				`json:"product"`
	Total             float64 				`json:"total"`
	Status            string  				`json:"status"`
}

// Environment variables
var mongoHost = os.Getenv("MONGOHOST")
var mongoUsername = os.Getenv("MONGOUSER")
var mongoPassword = os.Getenv("MONGOPASSWORD")
var mongoSSL = false 
var mongoPort = ""
var amqpURL = os.Getenv("AMQPURL")
var teamName = os.Getenv("TEAMNAME")
var mongoPoolLimit = 25

// MongoDB variables
var mongoDBSession *mgo.Session
var mongoDBSessionError error

// MongoDB database and collection names
var mongoDatabaseName = "akschallenge"
var mongoCollectionName = "orders"
var mongoCollectionShardKey = "orderid"

// AMQP 1.0 variables
var amqp10Client *amqp10.Client
var amqp10Session *amqp10.Session
var amqpSender *amqp10.Sender
var serivceBusName string

// Application Insights telemetry clients
var ChallengeTelemetryClient appinsights.TelemetryClient
var CustomTelemetryClient appinsights.TelemetryClient

// For tracking and code branching purposes
var isCosmosDb = strings.Contains(mongoHost, "documents.azure.com")
var db string // CosmosDB or MongoDB?

// TrackInitialOrder send telemetry data for initial order, to track challenge
func TrackInitialOrder(order Order) {
	eventTelemetry := appinsights.NewEventTelemetry("Initial order")
	eventTelemetry.Properties["team"] = teamName
	eventTelemetry.Properties["sequence"] = "0"
	eventTelemetry.Properties["type"] = "http"
	eventTelemetry.Properties["service"] = "CaptureOrder"
	eventTelemetry.Properties["orderId"] = order.OrderID
	ChallengeTelemetryClient.Track(eventTelemetry)
	if CustomTelemetryClient != nil {
		CustomTelemetryClient.Track(eventTelemetry)
	}
}

// AddOrderToMongoDB Adds the order to MongoDB/CosmosDB
func AddOrderToMongoDB(order Order) (Order, error) {
	success := false
	startTime := time.Now()

	// Use the existing mongoDBSessionCopy
	mongoDBSessionCopy := mongoDBSession.Copy()
	defer mongoDBSessionCopy.Close()

	order.ID = bson.NewObjectId()
	order.OrderID = order.ID.Hex()
	order.Status = "Open"

	log.Print("Inserting into MongoDB URL: ", mongoHost, " CosmosDB: ", isCosmosDb)
	log.Println(order)

	// insert Document in collection
	mongoDBCollection := mongoDBSessionCopy.DB(mongoDatabaseName).C(mongoCollectionName)
	mongoDBSessionError = mongoDBCollection.Insert(order)

	if mongoDBSessionError != nil {
		// If the team provided an Application Insights key, let's track that exception
		if CustomTelemetryClient != nil {
			CustomTelemetryClient.TrackException(mongoDBSessionError)
		}
		log.Println("Problem inserting data: ", mongoDBSessionError)
	} else {
		log.Println("Inserted order:", order.OrderID)
		success = true
	}

	endTime := time.Now()

	if success {
		// Track the event for the challenge purposes
		eventTelemetry := appinsights.NewEventTelemetry("CaptureOrder to " + db)
		eventTelemetry.Properties["team"] = teamName
		eventTelemetry.Properties["sequence"] = "1"
		eventTelemetry.Properties["type"] = db
		eventTelemetry.Properties["service"] = "CaptureOrder"
		eventTelemetry.Properties["orderId"] = order.OrderID
		ChallengeTelemetryClient.Track(eventTelemetry)
	}
	
	// Track the dependency, if the team provided an Application Insights key, let's track that dependency
	if CustomTelemetryClient != nil {
		if isCosmosDb {
			dependency := appinsights.NewRemoteDependencyTelemetry(
				"CosmosDB",
				"MongoDB",
				mongoHost,
				success)
			dependency.Data = "Insert order"		

			if mongoDBSessionError != nil {
				dependency.ResultCode = mongoDBSessionError.Error()
			}
				
			dependency.MarkTime(startTime, endTime)
			CustomTelemetryClient.Track(dependency)	
		} else {
			dependency := appinsights.NewRemoteDependencyTelemetry(
				"MongoDB",
				"MongoDB",
				mongoHost,
				success)
			dependency.Data = "Insert order"	

			if mongoDBSessionError != nil {
				dependency.ResultCode = mongoDBSessionError.Error()
			}

			dependency.MarkTime(startTime, endTime)
			CustomTelemetryClient.Track(dependency)		
		}
	}

	return order, mongoDBSessionError
}

// AddOrderToAMQP Adds the order to AMQP (Service Bus Queue)
func AddOrderToAMQP(order Order)  bool{
	if amqpURL != "" {
		return addOrderToAMQP10(order)
	} else {
		log.Println("Skipping inserting to Service Bus because it isn't configured yet.")
		return true
	}
}

//// BEGIN: NON EXPORTED FUNCTIONS
func init() {

	rand.Seed(time.Now().UnixNano())

	// Validate environment variables
	validateVariable(mongoHost, "MONGOHOST")
	validateVariable(mongoUsername, "MONGOUSERNAME")
	validateVariable(mongoPassword, "MONGOPASSWORD")
	validateVariable(amqpURL, "AMQPURL")
	validateVariable(teamName, "TEAMNAME")

	var mongoPoolLimitEnv = os.Getenv("MONGOPOOL_LIMIT")
	if mongoPoolLimitEnv != "" {
		if limit, err := strconv.Atoi(mongoPoolLimitEnv); err == nil {
			mongoPoolLimit = limit
		}
	}
	log.Printf("MongoDB pool limit set to %v. You can override by setting the MONGOPOOL_LIMIT environment variable." , mongoPoolLimit)

	// Initialize the MongoDB client
	initMongo()

	// Initialize the AMQP client if AMQPURL is passed
	if amqpURL != "" {
		initAMQP()
	}
}

// Logs out value of a variable
func validateVariable(value string, envName string) {
	if len(value) == 0 {
		log.Printf("The environment variable %s has not been set", envName)
	} else {
		log.Printf("The environment variable %s is %s", envName, value)
	}
}

func initMongoDial() (success bool, mErr error) {
	if isCosmosDb {
		log.Println("Using CosmosDB")
		db = "CosmosDB"
		mongoSSL = true
		mongoPort = ":10255"

	} else {
		log.Println("Using MongoDB")
		db = "MongoDB"
		mongoSSL = false
		mongoPort = ""
	}

	// Parse the connection string to extract components because the MongoDB driver is peculiar
	var dialInfo *mgo.DialInfo
	
	mongoDatabase := mongoDatabaseName // can be anything

	log.Printf("\tUsername: %s", mongoUsername)
	log.Printf("\tPassword: %s", mongoPassword)
	log.Printf("\tHost: %s", mongoHost)
	log.Printf("\tPort: %s", mongoPort)
	log.Printf("\tDatabase: %s", mongoDatabase)
	log.Printf("\tSSL: %t", mongoSSL)

	if mongoSSL {
		dialInfo = &mgo.DialInfo{
			Addrs:    []string{mongoHost+mongoPort},
			Timeout:  10 * time.Second,
			Database: mongoDatabase, // It can be anything
			Username: mongoUsername, // Username
			Password: mongoPassword, // Password
			DialServer: func(addr *mgo.ServerAddr) (net.Conn, error) {
				return tls.Dial("tcp", addr.String(), &tls.Config{})
			},
		}
	} else {
		dialInfo = &mgo.DialInfo{
			Addrs:    []string{mongoHost+mongoPort},
			Timeout:  10 * time.Second,
			Database: mongoDatabase, // It can be anything
			Username: mongoUsername, // Username
			Password: mongoPassword, // Password
		}
	}

	// Create a mongoDBSession which maintains a pool of socket connections
	// to our MongoDB.
	success = false
	startTime := time.Now()

	log.Println("Attempting to connect to MongoDB")
	mongoDBSession, mongoDBSessionError = mgo.DialWithInfo(dialInfo)
	if mongoDBSessionError != nil {
		log.Println(fmt.Sprintf("Can't connect to mongo at [%s], go error: ", mongoHost+mongoPort), mongoDBSessionError)
		trackException(mongoDBSessionError)
		mErr = mongoDBSessionError
	} else {
		success = true
		log.Println("\tConnected")
	}

	mongoDBSession.SetMode(mgo.Monotonic, true)

	// Limit connection pool to avoid running into Request Rate Too Large on CosmosDB
	mongoDBSession.SetPoolLimit(mongoPoolLimit)

	endTime := time.Now()

	// Track the dependency, if the team provided an Application Insights key, let's track that dependency
	if CustomTelemetryClient != nil {
		if isCosmosDb {
			dependency := appinsights.NewRemoteDependencyTelemetry(
				"CosmosDB",
				"MongoDB",
				mongoHost,
				success)
				dependency.Data = "Create session"

			if mongoDBSessionError != nil {
				dependency.ResultCode = mongoDBSessionError.Error()
			}

			dependency.MarkTime(startTime, endTime)
			CustomTelemetryClient.TrackException(mongoDBSessionError)
			CustomTelemetryClient.Track(dependency)
		} else {
			dependency := appinsights.NewRemoteDependencyTelemetry(
				"MongoDB",
				"MongoDB",
				mongoHost,
				success)
				dependency.Data = "Create session"

			if mongoDBSessionError != nil {
				dependency.ResultCode = mongoDBSessionError.Error()
			}

			dependency.MarkTime(startTime, endTime)
			CustomTelemetryClient.TrackException(mongoDBSessionError)
			CustomTelemetryClient.Track(dependency)
		}
	}
	return
}

// Initialize the MongoDB client
func initMongo() {

	success, err := initMongoDial()
	if !success {
		os.Exit(1)
	}

	mongoDBSessionCopy := mongoDBSession.Copy()
	defer mongoDBSessionCopy.Close()

	// SetSafe changes the mongoDBSessionCopy safety mode.
	// If the safe parameter is nil, the mongoDBSessionCopy is put in unsafe mode, and writes become fire-and-forget,
	// without error checking. The unsafe mode is faster since operations won't hold on waiting for a confirmation.
	// http://godoc.org/labix.org/v2/mgo#Session.SetMode.
	mongoDBSessionCopy.SetSafe(nil)

	// Create a sharded collection and retrieve it
	result := bson.M{}
	err = mongoDBSessionCopy.DB(mongoDatabaseName).Run(
		bson.D{
			{
				"shardCollection",
				fmt.Sprintf("%s.%s", mongoDatabaseName, mongoCollectionName),
			},
			{
				"key",
				bson.M{
					mongoCollectionShardKey: "hashed",
				},
			},
		}, &result)

	if err != nil {
		trackException(err)
		// The collection is most likely created and already sharded. I couldn't find a more elegant way to check this.
		log.Println("Could not create/re-create sharded MongoDB collection. Either collection is already sharded or sharding is not supported. You can ignore this error: ", err)
	} else {
		log.Println("Created MongoDB collection: ")
		log.Println(result)
	}
}

// Initalize AMQP by figuring out where we are running
func initAMQP() {
	url, err := url.Parse(amqpURL)
	if err != nil {
		// If the team provided an Application Insights key, let's track that exception
		if CustomTelemetryClient != nil {
			CustomTelemetryClient.TrackException(err)
		}
		log.Fatal(fmt.Sprintf("Problem parsing AMQP Host %s. Make sure you URL Encoded your policy/password.",url), err)
	}


	log.Println("Using Service Bus")

	// Parse the eventHubName (last part of the url)
	serivceBusName = url.Path
	initAMQP10()
	
	log.Println("\tAMQP URL: " + amqpURL)
	log.Println("** READY TO TAKE ORDERS **")
}

func initAMQP10() {		
	// Try to establish the connection to AMQP
	// with retry logic
	err := try.Do(func(attempt int) (bool, error) {
		var err error
		
		log.Println("Attempting to connect to ServiceBus")
		amqp10Client, err = amqp10.Dial(amqpURL)
		if err != nil {
			trackException(err)
		}

		// Open a session if we managed to get an amqpClient
		log.Println("\tConnected to Service Bus")	
		if amqp10Client != nil {
			log.Println("\tCreating a new AMQP session")
			amqp10Session, err = amqp10Client.NewSession()	
			if err != nil {
				trackException(err)
				log.Fatal("\t\tError creating AMQP session: ", err)
			}
		}

		// Create a sender
		log.Println("\tCreating AMQP sender")
		amqpSender, err = amqp10Session.NewSender(
			amqp10.LinkTargetAddress(serivceBusName),
		)
		if err != nil {
			// If the team provided an Application Insights key, let's track that exception
			if CustomTelemetryClient != nil {
				CustomTelemetryClient.TrackException(err)
			}
			log.Fatal("\t\tError creating sender link: ", err)
		}

		if err != nil {
			log.Println("Error connecting to Service Bus instance. Will retry in 5 seconds:", err)
		  	time.Sleep(5 * time.Second) // wait
		}
		return attempt < 3, err
	  })
	
	  // If we still can't connect
	if err != nil {
		log.Println("Couldn't connect to Service Bus after 3 retries:", err)
	}
}

// addOrderToAMQP10 Adds the order to AMQP 1.0 (sends to the Default ConsumerGroup)
func addOrderToAMQP10(order Order) bool {
	var success bool
	
	if amqp10Client == nil {
		log.Println("Skipping AMQP. It is either not configured or improperly configured")
		success = true
	} else {
		// Only run this part if AMQP is configured
		success := false
		var err error
		startTime := time.Now()
		body := fmt.Sprintf("{\"order\": \"%s\", \"source\": \"%s\"}", order.OrderID, teamName)

		// Get an empty context
		amqp10Context := context.Background()

		log.Printf("AMQP URL: %s, Target: %s", amqpURL, serivceBusName)

		// Prepare the context to timeout in 5 seconds
		amqp10Context, cancel := context.WithTimeout(amqp10Context, 5*time.Second)

		// Send with retry logic (in case we get a amqp.DetachError)
		err = try.Do(func(attempt int) (bool, error) {
			var err error

			log.Println("Attempting to send the AMQP message: ", body)
			err = amqpSender.Send(amqp10Context, amqp10.NewMessage([]byte(body)))
			if err != nil {
				switch t := err.(type) {
				default:
					log.Println("Encountered an error sending AMQP. Will not retry: ", err)						
					// If the team provided an Application Insights key, let's track that exception
					trackException(err)
					// This is an unhandled error, don't retry
					return false, err
				case *amqp10.DetachError:
					log.Println("Service Bus detached. Will reconnect and retry: " , t, err)
					initAMQP10()
			   }
			}
			return attempt < 3, err
		})

		// Now check after possible retries if the message was sent
		success = (err == nil)

		// Cancel the context and close the sender
		cancel()
		//sender.Close()
		
		endTime := time.Now()

		if success {
			// Track the event for the challenge purposes
			eventTelemetry := appinsights.NewEventTelemetry("SendOrder to ServiceBus")
			eventTelemetry.Properties["team"] = teamName
			eventTelemetry.Properties["sequence"] = "2"
			eventTelemetry.Properties["type"] = "servicebus"
			eventTelemetry.Properties["service"] = "CaptureOrder"
			eventTelemetry.Properties["orderId"] = order.OrderID
			ChallengeTelemetryClient.Track(eventTelemetry)
		}

		// Track the dependency, if the team provided an Application Insights key, let's track that dependency
		if CustomTelemetryClient != nil {
			dependency := appinsights.NewRemoteDependencyTelemetry(
				"ServiceBus",
				"AMQP",
				amqpURL,
				success)
			dependency.Data = "Send message"

			if err != nil {
				dependency.ResultCode = err.Error()
			}

			dependency.MarkTime(startTime, endTime)
			CustomTelemetryClient.Track(dependency)
		}
		log.Printf("Sent to AMQP 1.0 (ServiceBus) - %t, %s: %s", success, amqpURL, body)
	}
	return success
}

func trackException(err error) {
	if err != nil {
		log.Println(err)
		if ChallengeTelemetryClient != nil {
			ChallengeTelemetryClient.TrackException(err)
		}
		if CustomTelemetryClient != nil {
			CustomTelemetryClient.TrackException(err)
		}
	}
}

// random: Generates a random number
func random(min int, max int) int {
	return rand.Intn(max-min) + min
}

//// END: NON EXPORTED FUNCTIONS