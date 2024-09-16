package model

import "time"

type WriteRequest struct {
	Function *Function
	Data     *[]map[string]interface{}

	Duration *time.Duration
}

type IntegrationItem struct {
	Type          string
	Url           string
	InputJsonPath string
	Functions     []Function

	// token to access the API
	Token string
}

type Function struct {
	Name          string
	Type          string
	Params        []string
	TransformPath string
}
