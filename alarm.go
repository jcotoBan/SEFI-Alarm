package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	conf            = loadConfig()
	bearerToken     = conf["bearerToken"].(string)
	integrationID   = fmt.Sprintf("%d", conf["integrationId"].(int))
	tenantID        = fmt.Sprintf("%d", conf["tenantId"].(int))
	endpointURL     = setRegionUrl(conf["region"].(string)) + integrationID + "/" + tenantID
	checkInterval   = time.Duration((conf["pollIntervalSecs"].(int))) * time.Second
	slackWebhookURL = conf["slackWebhookUrl"].(string)
	integrationURL  = setIntegrationUrl(conf["region"].(string))
)

type Payload struct {
	CustomerID    int        `json:"customerId"`
	IntegrationID int        `json:"integrationId"`
	Count         int        `json:"count"`
	Errors        []ErrorLog `json:"errors"`
}

type ErrorLog struct {
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
}

type SlackMessage struct {
	Text string `json:"text"`
}

func setRegionUrl(region string) string {

	baseurlus1 := "https://secure.sysdig.com/api/v1/eventsForwarding/errors/"
	baseurlus2 := "https://us2.app.sysdig.com/api/v1/eventsForwarding/errors/"
	baseurlus4 := "https://app.us4.sysdig.com/api/v1/eventsForwarding/errors/"
	baseurleu1 := "https://eu1.app.sysdig.com/api/v1/eventsForwarding/errors/"
	baseurlau1 := "https://app.au1.sysdig.com/api/v1/eventsForwarding/errors/"
	baseurlme2 := "https://app.me2.sysdig.com/api/v1/eventsForwarding/errors/"
	baseurlin1 := "https://app.in1.sysdig.com/api/v1/eventsForwarding/errors/"

	switch region {

	case "us1":
		return baseurlus1
	case "us2":
		return baseurlus2
	case "us4":
		return baseurlus4
	case "eu1":
		return baseurleu1
	case "au1":
		return baseurlau1
	case "me2":
		return baseurlme2
	case "in1":
		return baseurlin1
	default:
		return baseurlus1

	}
}

func setIntegrationUrl(region string) string {

	baseurlus1 := "https://secure.sysdig.com/secure/#/settings/events-forwarding/"
	baseurlus2 := "https://us2.app.sysdig.com/secure/#/settings/events-forwarding/"
	baseurlus4 := "https://app.us4.sysdig.com/secure/#/settings/events-forwarding/"
	baseurleu1 := "https://eu1.app.sysdig.com/secure/#/settings/events-forwarding/"
	baseurlau1 := "https://app.au1.sysdig.com/secure/#/settings/events-forwarding/"
	baseurlme2 := "https://app.me2.sysdig.com/secure/#/settings/events-forwarding/"
	baseurlin1 := "https://app.in1.sysdig.com/secure/#/settings/events-forwarding/"

	switch region {

	case "us1":
		return baseurlus1
	case "us2":
		return baseurlus2
	case "us4":
		return baseurlus4
	case "eu1":
		return baseurleu1
	case "au1":
		return baseurlau1
	case "me2":
		return baseurlme2
	case "in1":
		return baseurlin1
	default:
		return baseurlus1

	}
}

func loadConfig() map[string]interface{} {

	obj := make(map[string]interface{})

	data, err := os.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(data, &obj)
	if err != nil {
		panic(err)
	}

	configMap := obj["config"].(map[string]interface{})

	return (configMap)
}

func pollEndpoint() (*Payload, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", endpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return &payload, nil
}

func sendSlackNotification(message string) error {
	slackPayload := SlackMessage{
		Text: message,
	}

	payloadBytes, err := json.Marshal(slackPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %v", err)
	}

	resp, err := http.Post(slackWebhookURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack notification failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func createSlackMessage(errors []ErrorLog, payload *Payload, integrationUrl string) string {
	message := "Recent Errors found on integration: " + fmt.Sprintf("%d", payload.IntegrationID) + "\n"
	for _, err := range errors {
		message += err.Error + "\n"
	}
	message += "\n" + "You can check the integration in the following link: " + integrationUrl + fmt.Sprintf("%d", payload.IntegrationID)
	return message
}

func main() {
	for {
		payload, err := pollEndpoint()
		if err != nil {
			log.Printf("Error fetching data: %v\n", err)
			continue
		}

		now := time.Now().UTC()
		oneMinuteAgo := now.Add(-1 * time.Minute)
		var recentErrors []ErrorLog

		for _, err := range payload.Errors {
			timestamp, parseErr := time.Parse(time.RFC3339Nano, err.Timestamp)
			if parseErr != nil {
				log.Printf("Error parsing timestamp: %v\n", parseErr)
				continue
			}

			if timestamp.After(oneMinuteAgo) && timestamp.Before(now) {
				recentErrors = append(recentErrors, err)
			}
		}

		if len(recentErrors) > 0 {

			fmt.Println(payload.IntegrationID)

			slackMessage := createSlackMessage(recentErrors, payload, integrationURL)

			err := sendSlackNotification(slackMessage)
			if err != nil {
				log.Printf("Error sending Slack notification: %v\n", err)
			} else {
				log.Println("Slack notification sent successfully.")
			}
		} else {
			log.Println("No new errors found.")
		}

		time.Sleep(checkInterval)
	}
}
