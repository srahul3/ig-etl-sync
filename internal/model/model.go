package model

import (
	"fmt"
	"time"
)

type WriteRequest struct {
	Function *Function
	ToCreate *[]map[string]interface{}
	ToDelete *[]map[string]interface{}

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

func (f *Function) GetKey() string {
	return f.Type + ":" + f.Name
}

func (i *IntegrationItem) GetKey() (string, error) {
	switch i.Type {
	case "http":
		return i.Type + ":" + i.Url, nil
	default:
		return "", fmt.Errorf("invalid type: %s", i.Type)
	}
}
