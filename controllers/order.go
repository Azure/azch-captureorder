package controllers

import (
	"captureorderfd/models"
	"encoding/json"
	"os"
	"time"
	"fmt"
	"strconv"
	"github.com/astaxie/beego"
	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
)


var customInsightsKey = os.Getenv("APPINSIGHTS_KEY")
var challengeInsightsKey = os.Getenv("CHALLENGEAPPINSIGHTS_KEY")
var teamName = os.Getenv("TEAMNAME")

// Application Insights telemetry clients
var challengeTelemetryClient appinsights.TelemetryClient
var customTelemetryClient appinsights.TelemetryClient

// Operations about object
type OrderController struct {
	beego.Controller
}

func init() {
	// Init App Insights
	challengeTelemetryClient = appinsights.NewTelemetryClient(challengeInsightsKey)
	challengeTelemetryClient.Context().Tags.Cloud().SetRole("captureorder")

	if customInsightsKey != "" {
		customTelemetryClient = appinsights.NewTelemetryClient(customInsightsKey)

		// Set role instance name globally -- this is usually the
		// name of the service submitting the telemetry
		customTelemetryClient.Context().Tags.Cloud().SetRole("captureorder")
	}

	appinsights.NewDiagnosticsMessageListener(func(msg string) error {
		fmt.Printf("[%s] %s\n", time.Now().Format(time.UnixDate), msg)
		return nil
	})
}

// @Title Capture Order
// @Description Capture order POST
// @Param	body	body 	models.Order true		"body for order content"
// @Success 200 {string} models.Order.ID
// @Failure 403 body is empty
// @router / [post]
func (this *OrderController) Post() {

	var ob models.Order
	json.Unmarshal(this.Ctx.Input.RequestBody, &ob)

	// Inject telemetry clients
	models.CustomTelemetryClient = customTelemetryClient;
	models.ChallengeTelemetryClient = challengeTelemetryClient;
	
	// Track the request
	requestStartTime := time.Now()

	// Add the order to MongoDB
	orderID, err := models.AddOrderToMongoDB(ob)
	var orderAddedToMongoDb = false
	var orderAddedToAMQP = false

	if err == nil {
		orderAddedToMongoDb = true

		// Add the order to AMQP
		orderAddedToAMQP = models.AddOrderToAMQP(orderID)

		fmt.Printf("[%s] orderid: %s mongo: %b amqp: %b\n", time.Now().Format(time.UnixDate), orderID, orderAddedToMongoDb, orderAddedToAMQP)
		trackRequest(requestStartTime, time.Now(), orderAddedToMongoDb && orderAddedToAMQP, "POST", "captureorder.svc/orders/v1")

		// return
		this.Data["json"] = map[string]string{"orderId": orderID}
	} else {
		this.Data["json"] = map[string]string{"error": "order not added to MongoDB. Check logs: " + err.Error()}
		this.Ctx.Output.SetStatus(500)

		fmt.Printf("[%s] orderid: %s mongo: %b amqp: %b\n", time.Now().Format(time.UnixDate), orderID, orderAddedToMongoDb, orderAddedToAMQP)
		trackRequest(requestStartTime, time.Now(), false, "POST", "captureorder.svc/orders/v1")
	}
	

	this.ServeJSON()
}

// @Title Capture Order
// @Description Capture order GET
// @Success 200 {string} count of orders in the database
// @Failure 403 body is empty
// @router / [post]
func (this *OrderController) Get() {

	var ob models.Order
	json.Unmarshal(this.Ctx.Input.RequestBody, &ob)

	// Inject telemetry clients
	models.CustomTelemetryClient = customTelemetryClient;
	models.ChallengeTelemetryClient = challengeTelemetryClient;
	
	// Track the request
	requestStartTime := time.Now()

	// Get number of orders in MongoDB
	orderCount, err := models.GetNumberOfOrdersInDB()
	var orderCountQueried = false

	if err == nil {
		orderCountQueried = true
		trackRequest(requestStartTime, time.Now(), orderCountQueried, "GET", "captureorder.svc/orders/v1")

		// return
		this.Data["json"] = map[string]string{"orderCount": strconv.Itoa(orderCount), "timestamp": time.Now().String()}
	} else {
		this.Data["json"] = map[string]string{"error": "couldn't query order count. Check logs: " + err.Error()}
		this.Ctx.Output.SetStatus(500)
		trackRequest(requestStartTime, time.Now(), false, "GET", "captureorder.svc/orders/v1")
	}
	
	this.ServeJSON()
}

func trackRequest(requestStartTime time.Time, requestEndTime time.Time, requestSuccess bool, method string, endpoint string) {
	var responseCode = "200"
	if requestSuccess != true {
		responseCode = "500"
	} 
	requestTelemetry := appinsights.NewRequestTelemetry(method, endpoint, 0, responseCode)
	requestTelemetry.MarkTime(requestStartTime, requestEndTime)
	requestTelemetry.Properties["team"] = teamName
	requestTelemetry.Properties["service"] = "CaptureOrder"
	requestTelemetry.Name = "CaptureOrder"

	challengeTelemetryClient.Track(requestTelemetry)
	if customTelemetryClient != nil {
		customTelemetryClient.Track(requestTelemetry)
	}
}
