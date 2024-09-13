package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/srahul3/cypher-transform/internal/model"
	"github.com/srahul3/cypher-transform/internal/store"
)

var integration = []*model.IntegrationItem{
	{
		InputJsonPath: "data/input/organizations.json",
		TransformPath: "data/transform/transform_org.json.tmpl",
		Function:      store.CREATE_NODE,
		Labels:        []string{"OrganizationTF"},
		// Query:         "UNWIND $list AS item MERGE (a:OrganizationTF {external_id: item.external_id}) SET a = item",
	},
	{
		InputJsonPath: "data/input/workspaces.json",
		TransformPath: "data/transform/transform_ws.json.tmpl",
		Function:      store.CREATE_NODE,
		Labels:        []string{"WorkspaceX"},
		// Query:         "UNWIND $list AS item MERGE (a:WorkspaceX {external_id: item.external_id}) SET a = item",
	},
	{
		InputJsonPath: "data/input/workspaces.json",
		TransformPath: "data/transform/transform_vcs.json.tmpl",
		Function:      store.CREATE_NODE,
		Labels:        []string{"VCS"},
		// Query:         "UNWIND $list AS item MERGE (a:VCS {external_id: item.external_id}) SET a = item",
	},
	{
		InputJsonPath: "data/input/workspaces.json",
		TransformPath: "data/transform/transform_ws_R.json.tmpl",
		Function:      store.CREATE_RELATION,
		Labels:        []string{"WorkspaceX", "OrganizationTF", "has"},
		// Query:         "UNWIND $list AS item MATCH (a:WorkspaceX {external_id: item.a_id}) MATCH (b:OrganizationTF {external_id: item.b_id}) MERGE (b)-[:has]->(a)",
	},
	{
		InputJsonPath: "data/input/workspaces.json",
		TransformPath: "data/transform/transform_vcs_R.json.tmpl",
		Function:      store.CREATE_RELATION,
		Labels:        []string{"VCS", "WorkspaceX", "listens"},
		// Query:         "UNWIND $list AS item MATCH (a:VCS {external_id: item.a_id}) MATCH (b:WorkspaceX {external_id: item.b_id}) MERGE (b)-[:listens]->(a)",
	},
}

func execute(store store.Store, integrationItem *model.IntegrationItem) error {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
		"add": func(a, b int) int {
			return a + b
		},
	}
	name := filepath.Base(integrationItem.TransformPath)
	t, err := template.New(name).Funcs(funcMap).ParseFiles(integrationItem.TransformPath)
	if err != nil {
		return err
	}

	m := map[string]interface{}{}
	file, err := os.Open(integrationItem.InputJsonPath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&m)
	if err != nil {
		return err
	}

	// Create a bytes.Buffer to capture the output
	var buf bytes.Buffer

	// Execute the template and write output to the buffer
	if err := t.Execute(&buf, m); err != nil {
		return err
	}

	// Convert buffer to string
	outputString := buf.String()
	// Print the string output
	fmt.Println("Template Output as String:")
	fmt.Println(outputString)

	// Unmarshal the string back into a generic interface
	var unmarshaledOutput []map[string]interface{}
	err = json.Unmarshal([]byte(outputString), &unmarshaledOutput)
	if err != nil {
		return err
	}

	// Print the unmarshaled output
	fmt.Println("Unmarshaled Output:")
	fmt.Printf("%+v\n", unmarshaledOutput)

	writeRequest := &model.WriteRequest{
		IntegrationItem: integrationItem,
		Data:            &unmarshaledOutput,
	}

	store.Write(writeRequest)

	if err != nil {
		return err
	}

	return nil
}

func main() {
	store := store.NewNeo4jStore()
	defer store.Close()
	err := store.Connect()
	if err != nil {
		panic(err)
	}

	err = store.Setup()
	if err != nil {
		panic(err)
	}

	for _, item := range integration {
		err := execute(store, item)
		if err != nil {
			panic(err)
		}
	}
}
