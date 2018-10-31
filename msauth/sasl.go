// Package msauth is used to generate a Microsoft SASL signed token
// to be used across various services provided by Microsoft.
package msauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Signer helps in creating a valid Microsoft SASL signed token.
type Signer interface {
	Sign(uri string, expiry string) string
}

type signer struct {
	namespace string
	saKey     string
	saValue   []byte
	url       string
}

const (
	serviceBusURL = "https://%s.servicebus.windows.net:443/"
)

// New creates a new auth builder from the given parameters. Their meaning can be found in the MSDN docs at:
//  https://docs.microsoft.com/en-us/rest/api/servicebus/Introduction
//  https://docs.microsoft.com/en-us/azure/service-bus-messaging/service-bus-sas
func New(namespace string, sharedAccessKeyName string, sharedAccessKeyValue string) Signer {
	return &signer{
		namespace: namespace,
		saKey:     sharedAccessKeyName,
		saValue:   []byte(sharedAccessKeyValue),
		url:       fmt.Sprintf(serviceBusURL, namespace),
	}
}

// Sign returns the value of the Microsoft token that could be used for various Azure services:
// - as authorization header for requests to Azure Service Bus
// - during the CBS handshake when connecting to the Event Hub using AMQP 1.0
// The parameter "expiry" could be generated using "SignatureExpiry"
//
// It's translated from the Python client:
// https://github.com/Azure/azure-sdk-for-python/blob/master/azure-servicebus/azure/servicebus/servicebusservice.py
func (s *signer) Sign(uri string, expiry string) string {
	u := signatureURI(uri)
	sts := stringToSign(u, expiry)
	sig := s.signString(sts)
	return fmt.Sprintf("SharedAccessSignature sig=%s&se=%s&skn=%s&sr=%s", sig, expiry, s.saKey, u)
}

// SignatureExpiry returns the expiry for the shared access signature for the next request.
//
// It's translated from the Python client:
// https://github.com/Azure/azure-sdk-for-python/blob/master/azure-servicebus/azure/servicebus/servicebusservice.py
func SignatureExpiry(from time.Time, interval time.Duration) string {
	t := from.Add(interval).Round(time.Second).Unix()
	return strconv.Itoa(int(t))
}

// signatureURI returns the canonical URI according to Azure specs.
//
// It's translated from the Python client:
// https://github.com/Azure/azure-sdk-for-python/blob/master/azure-servicebus/azure/servicebus/servicebusservice.py
func signatureURI(uri string) string {
	return strings.ToLower(url.QueryEscape(uri)) //Python's urllib.quote and Go's url.QueryEscape behave differently. This might work, or it might not...like everything else to do with authentication in Azure.
}

// stringToSign returns the string to sign.
//
// It's translated from the Python client:
// https://github.com/Azure/azure-sdk-for-python/blob/master/azure-servicebus/azure/servicebus/servicebusservice.py
func stringToSign(uri string, expiry string) string {
	return uri + "\n" + expiry
}

//signString returns the HMAC signed string.
//
//It's translated from the Python client:
//https://github.com/Azure/azure-sdk-for-python/blob/master/azure-servicebus/azure/servicebus/_common_conversion.py
func (s *signer) signString(str string) string {
	h := hmac.New(sha256.New, s.saValue)
	h.Write([]byte(str))
	encodedSig := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return url.QueryEscape(encodedSig)
}
