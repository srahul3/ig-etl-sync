package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/srahul3/cypher-transform/internal/model"
	"github.com/srahul3/cypher-transform/internal/recon"
	"github.com/srahul3/cypher-transform/internal/store"
)

var integration = []*model.IntegrationItem{
	{
		Type: "http",
		Url:  "https://api.cloud.hashicorp.com/packer/2022-12-02/organizations/404cabe0-7f2a-456a-ac0b-be021e926ae0/projects/c0e149a8-85db-41a6-a79c-fbcc5669f63e/buckets",
		Functions: []model.Function{
			{
				Name:          "buckets",
				Type:          store.CREATE_NODE,
				Params:        []string{"bucket"},
				TransformPath: "data/transform/transform_bucket.json.tmpl",
			},
			{
				Name:          "orgs",
				Type:          store.CREATE_NODE,
				Params:        []string{"org"},
				TransformPath: "data/transform/transform_organization.json.tmpl",
			},
			{
				Name:          "projects",
				Type:          store.CREATE_NODE,
				Params:        []string{"project"},
				TransformPath: "data/transform/transform_project.json.tmpl",
			},
			{
				Name:          "(org)-[has]->(project)",
				Type:          store.CREATE_RELATION,
				Params:        []string{"org", "has", "project"},
				TransformPath: "data/transform/transform_org_project_R.json.tmpl",
			},
			{
				Name:          "(project)-[has]->(bucket)",
				Type:          store.CREATE_RELATION,
				Params:        []string{"project", "has", "bucket"},
				TransformPath: "data/transform/transform_project_bucket_R.json.tmpl",
			},
			{
				Name:          "packer_build",
				Type:          store.CREATE_NODE,
				Params:        []string{"packer_build"},
				TransformPath: "data/transform/transform_build.json.tmpl",
			},
			{
				Name:          "version",
				Type:          store.CREATE_NODE,
				Params:        []string{"version"},
				TransformPath: "data/transform/transform_version.json.tmpl",
			},
			{
				Name:          "(bucket)-[creates]->(version)",
				Type:          store.CREATE_RELATION,
				Params:        []string{"bucket", "creates", "version"},
				TransformPath: "data/transform/transform_bucket_version_R.json.tmpl",
			},
			{
				Name:          "(version)-[creates]->(packer_build)",
				Type:          store.CREATE_RELATION,
				Params:        []string{"version", "creates", "packer_build"},
				TransformPath: "data/transform/transform_version_build_R.json.tmpl",
			},
		},
	},
}

func generateHCPToken() (string, error) {
	// keys of service principal with access to HCP
	client_id := os.Getenv("HCP_CLIENT_ID")
	client_secret := os.Getenv("HCP_CLIENT_SECRET")

	// http request to get token
	// url := "https://iam.cloud.ibm.com/identity/token"
	auth_url := "https://auth.idp.hashicorp.com/oauth2/token"

	// url encoded form data
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", client_id)
	data.Set("client_secret", client_secret)
	data.Set("audience", "https://api.hashicorp.cloud")

	client := &http.Client{}
	r, _ := http.NewRequest("POST", auth_url, strings.NewReader(data.Encode()))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Accept", "application/json")

	resp, err := client.Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(body), &result)

	return result["access_token"].(string), nil
}

func execute(igSynchronizer *IGSynchronizer, integrationItem *model.IntegrationItem, updateData func(data *(map[string]interface{}))) error {
	data := map[string]interface{}{}

	// get the data from integrationItem.url
	if integrationItem.Type == "http" {
		req, err := http.NewRequest("GET", integrationItem.Url, nil)
		if err != nil {
			return err
		}

		req.Header.Add("Authorization", "Bearer "+integrationItem.Token)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// read the response body into data
		body, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(body, &data)
		if err != nil {
			return err
		}
	}

	updateData(&data)

	funcMap := template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
		"add": func(a, b int) int {
			return a + b
		},
	}

	for _, function := range integrationItem.Functions {
		name := filepath.Base(function.TransformPath)
		t, err := template.New(name).Funcs(funcMap).ParseFiles(function.TransformPath)
		if err != nil {
			return err
		}

		// Create a bytes.Buffer to capture the output
		var buf bytes.Buffer

		// Execute the template and write output to the buffer
		if err := t.Execute(&buf, data); err != nil {
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

		toDelete, toCreate, err := igSynchronizer.Reconciler.Reconcile(
			integrationItem,
			function,
			unmarshaledOutput,
		)

		if err != nil {
			return err
		}

		fmt.Println("Delete:" + fmt.Sprint(toDelete))
		fmt.Println("Create:" + fmt.Sprint(toCreate))

		writeRequest := &model.WriteRequest{
			Function: &function,
			ToCreate: &toCreate,
			ToDelete: &toDelete,
		}

		err = igSynchronizer.Store.Write(writeRequest)
		if err != nil {
			return err
		}

		err = igSynchronizer.Reconciler.Commit(
			integrationItem,
			function,
			toDelete,
			toCreate)

		if err != nil {
			return err
		}

		// test the commit
		toDelete, toCreate, err = igSynchronizer.Reconciler.Reconcile(
			integrationItem,
			function,
			unmarshaledOutput,
		)
		if err != nil {
			return err
		}

		// expecting empty toDelete and toCreate
		if function.Type == store.CREATE_NODE && (len(toDelete) != 0 || len(toCreate) != 0) {
			return fmt.Errorf("commit failed")
		}

	}
	return nil
}

type IGSynchronizer struct {
	Store      store.Store
	Reconciler *recon.Reconciler

	HCPToken string
}

func main() {
	// load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	store := store.NewNeo4jStore()
	defer store.Close()
	err = store.Connect()
	if err != nil {
		panic(err)
	}

	err = store.Setup()
	if err != nil {
		panic(err)
	}

	// common token for all HCP integrations
	token, err := generateHCPToken()
	if err != nil {
		panic(err)
	}

	igSynchronizer := &IGSynchronizer{
		Store:      store,
		Reconciler: recon.NewReconciler(),
		HCPToken:   token,
	}

	for _, item := range integration {
		item.Token = token
		err := execute(igSynchronizer, item, func(data *(map[string]interface{})) {})
		if err != nil {
			panic(err)
		}

		// test by removing data
		err = execute(igSynchronizer, item, func(data *(map[string]interface{})) {
			(*data)["buckets"] = []string{}
			fmt.Println("Removing data")
		})
		if err != nil {
			panic(err)
		}
	}
}
