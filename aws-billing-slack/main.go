package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/aws/aws-sdk-go/service/costexplorer/costexploreriface"
)

// https://api.slack.com/reference/messaging/payload
type payload struct {
	Blocks []sectionBlock `json:"blocks,omitempty"`
}

// https://api.slack.com/reference/block-kit/blocks#section
type sectionBlock struct {
	Type   string       `json:"type"`
	Text   *textObject  `json:"text,omitempty"`
	Fields []textObject `json:"fields,omitempty"`
}

// https://api.slack.com/reference/block-kit/composition-objects#text
type textObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// getServiceCost : Get monthly charges for each service
func getServiceCost(svc costexploreriface.CostExplorerAPI) ([]*costexplorer.Group, error) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	now := time.Now().UTC().In(jst)

	currentLocation := now.Location()
	currentYear, currentMonth, _ := now.Date()
	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	startTime, endTime := firstOfMonth, lastOfMonth

	metric := "UnblendedCost"
	input := costexplorer.GetCostAndUsageInput{
		Granularity: aws.String("MONTHLY"),
		Metrics:     []*string{&metric},
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(startTime.Format("2006-01-02")),
			End:   aws.String(endTime.Format("2006-01-02")),
		},
		GroupBy: []*costexplorer.GroupDefinition{
			{
				Key:  aws.String("SERVICE"),
				Type: aws.String("DIMENSION"),
			},
		},
	}

	result, err := svc.GetCostAndUsage(&input)
	if err != nil {
		return nil, fmt.Errorf("Failed to get AWS usage charges for each service: %w", err)
	}

	return result.ResultsByTime[0].Groups, nil
}

// calcTotalCost : Get the total monthly charges for all services
func calcTotalCost(serviceBillingList []*costexplorer.Group) (float64, error) {
	var totalCost float64
	for _, serviceBilling := range serviceBillingList {
		billingStr := serviceBilling.Metrics["UnblendedCost"].Amount
		billing, _ := strconv.ParseFloat(*billingStr, 64)

		totalCost += billing
	}

	return totalCost, nil
}

func makeMessagePayload(totalCost float64, serviceBillingList []*costexplorer.Group) payload {
	var serviceTextObjectList []textObject
	for _, serviceBilling := range serviceBillingList {
		serviceName := *serviceBilling.Keys[0]
		billingStr := serviceBilling.Metrics["UnblendedCost"].Amount
		billing, _ := strconv.ParseFloat(*billingStr, 64)

		serviceTextObjectList = append(serviceTextObjectList, textObject{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*%s*\n%.2f USD", serviceName, billing),
		})
	}

	var sectionBlockList []sectionBlock
	for i := range serviceTextObjectList {
		if i%2 != 0 {
			continue
		}

		if i+1 < len(serviceTextObjectList) {
			sectionBlockList = append(sectionBlockList, sectionBlock{
				Type: "section",
				Fields: []textObject{
					serviceTextObjectList[i],
					serviceTextObjectList[i+1],
				},
			})
		} else {
			sectionBlockList = append(sectionBlockList, sectionBlock{
				Type: "section",
				Fields: []textObject{
					serviceTextObjectList[i],
				},
			})
		}
	}

	return payload{
		Blocks: append([]sectionBlock{
			{
				Type: "section",
				Text: &textObject{
					Type: "mrkdwn",
					Text: fmt.Sprintf("<https://console.aws.amazon.com/billing/home|AWS Billing Management Console>\n*Total Cost(Monthly)* : %.2f USD", totalCost),
				},
			},
		}, sectionBlockList...),
	}
}

func postMessage(p payload) error {
	params, err := json.Marshal(&p)
	if err != nil {
		return fmt.Errorf("json.Marshal(): %w", err)
	}

	resp, err := http.PostForm(os.Getenv("SLACK_WEBHOOK_URL"), url.Values{"payload": {string(params)}})
	if err != nil {
		return fmt.Errorf("http.PostForm(): %w", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ioutil.ReadAll(): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Error sending msg. (%s) Status: %v", string(body), resp.Status)
	}

	return nil
}

func handler() error {
	ce := costexplorer.New(session.Must(session.NewSession()))

	serviceCost, err := getServiceCost(ce)
	if err != nil {
		return err
	}

	totalCost, err := calcTotalCost(serviceCost)
	if err != nil {
		return err
	}

	payload := makeMessagePayload(totalCost, serviceCost)
	err = postMessage(payload)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
