package model

import "time"

type WriteRequest struct {
	IntegrationItem *IntegrationItem
	Data            *[]map[string]interface{}

	Duration *time.Duration
}

type IntegrationItem struct {
	InputJsonPath string
	TransformPath string
	Function      string
	Labels        []string
}
