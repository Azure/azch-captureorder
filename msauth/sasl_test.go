package msauth

import (
	"fmt"
	"testing"
	"time"
)

const (
	testFooURI           = "foo://bar:baz/uri"
	encodedFooURI        = "foo%3a%2f%2fbar%3abaz%2furi"
	encoded1970ExpiryStr = "300"
)

func TestSignatureURI(t *testing.T) {
	signatureURI := signatureURI(testFooURI)

	if signatureURI != encodedFooURI {
		t.Error(fmt.Printf("The encoded signature URI '%s' is not the expected one!", signatureURI))
	}
}

func TestSignatureExpiry(t *testing.T) {
	epoch1970 := int64(0)
	time1970 := time.Unix(epoch1970, 0)
	intervalInSeconds := 300 * time.Second

	signatureExpiry := SignatureExpiry(time1970, intervalInSeconds)

	if signatureExpiry != encoded1970ExpiryStr {
		t.Error(fmt.Printf("The output signature expiry '%s' is not the expected one!", signatureExpiry))
	}
}

func TestStringToSign(t *testing.T) {
	fooURI := "fooURI"
	fooEpochStr := "12345"
	expectedSignedString := "fooURI\n12345"

	stringToSign := stringToSign(fooURI, fooEpochStr)

	if stringToSign != expectedSignedString {
		t.Error(fmt.Printf("The output string to sign '%s' is not the expected one!", stringToSign))
	}
}

func TestSASToken(t *testing.T) {
	testEhNamespace := "fooNamespace"
	testEhSasUsername := "fooSasUsername"
	testEhSasPassword := "fooSasPassword"
	sasSigner := New(testEhNamespace, testEhSasUsername, testEhSasPassword)

	// general purpose "foo" scenario
	fooAuthToken := sasSigner.Sign(encodedFooURI, encoded1970ExpiryStr)
	if fooAuthToken != "SharedAccessSignature sig=8Ew%2B0SNKAp0jAMHLQnYYRlbQBOvwNMu5nP6E3IUySqo%3D&se=300&skn=fooSasUsername&sr=foo%253a%252f%252fbar%253abaz%252furi" {
		t.Error(fmt.Printf("The generated token for the 'foo' service '%s' is not the expected one!", fooAuthToken))
	}

	// the Event Hub AMQP 1.0 CBS (claims-based authorization) scenario
	eventHubURI := "amqp://<NAMESPACE>.servicebus.windows.net/<NAME>"
	eventHubAmqpAuthToken := sasSigner.Sign(eventHubURI, encoded1970ExpiryStr)
	if eventHubAmqpAuthToken != "SharedAccessSignature sig=YG4QyqZJTZg4mfgKeWSk8w52nEIksrsjIl8%2BIy2kxrg%3D&se=300&skn=fooSasUsername&sr=amqp%3a%2f%2f%3cnamespace%3e.servicebus.windows.net%2f%3cname%3e" {
		t.Error(fmt.Printf("The generated token for the 'Event Hub' service '%s' is not the expected one!", eventHubAmqpAuthToken))
	}

	// the Service Bus HTTPS CBS (claims-based authorization) scenario
	serviceBusURI := "https://<NAMESPACE>.servicebus.windows.net:443/<NAME>/head?timeout=60"
	serviceBusAuthToken := sasSigner.Sign(serviceBusURI, encoded1970ExpiryStr)
	if serviceBusAuthToken != "SharedAccessSignature sig=aBmX7BF9OinPcPOOfs9uqqPddqib76Slag27V8rxooo%3D&se=300&skn=fooSasUsername&sr=https%3a%2f%2f%3cnamespace%3e.servicebus.windows.net%3a443%2f%3cname%3e%2fhead%3ftimeout%3d60" {
		t.Error(fmt.Printf("The generated token for the 'Service Bus' '%s' is not the expected one!", serviceBusAuthToken))
	}
}
