package store

import (
	"fmt"
	"os"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/srahul3/cypher-transform/internal/model"
)

func NewNeo4jStore() Store {
	return &neo4jStore{}
}

type neo4jStore struct {
	driver  neo4j.Driver
	session neo4j.Session
}

func (s *neo4jStore) Connect() error {
	// c.ui
	// Initialize the Neo4j driver
	var err error

	// read NEO4J_URI from environment variables
	uri := os.Getenv("NEO4J_URI")
	username := os.Getenv("NEO4J_DB_USERNAME")
	password := os.Getenv("NEO4J_DB_PASSWORD")

	fmt.Println("uri:", uri)
	fmt.Println("username:", username)
	fmt.Println("password:", password)

	s.driver, err = neo4j.NewDriver(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return fmt.Errorf("failed to create driver: %v", err)
	}

	// Open a new session
	s.session = s.driver.NewSession(neo4j.SessionConfig{})
	if err != nil {
		return fmt.Errorf("failed to open session: %v", err)
	}

	return nil
}

func (s *neo4jStore) Setup() error {
	// terraform plan and apply

	return nil
}

func (s *neo4jStore) createIndex(req *model.WriteRequest) error {
	if req.Function.Type == CREATE_RELATION {
		tx, err := s.session.BeginTransaction()
		if err != nil {
			return err
		}
		defer tx.Close()

		query := "CREATE INDEX IF NOT EXISTS FOR (n:%s) ON (n.external_id)"
		_, err = tx.Run(fmt.Sprintf(query, req.Function.Params[0]), nil)
		if err != nil {
			return err
		}

		return tx.Commit()
	}
	return fmt.Errorf("invalid function: ", req.Function.Type)
}

func (s *neo4jStore) Write(req *model.WriteRequest) error {
	fmt.Println("creating index")
	s.createIndex(req)

	tx, err := s.session.BeginTransaction()
	if err != nil {
		return err
	}
	defer tx.Close()

	fmt.Println("creating label")
	var query string

	if req.Function.Type == CREATE_NODE {
		query = "UNWIND $list AS item MERGE (x:%s {external_id: item.external_id}) SET x = item"
		query = fmt.Sprintf(query, req.Function.Params[0])
	} else if req.Function.Type == CREATE_RELATION {
		query = "UNWIND $list AS item MATCH (a:%s {external_id: item.a_id}) MATCH (b:%s {external_id: item.b_id}) MERGE (a)-[:%s]->(b)"
		query = fmt.Sprintf(query, req.Function.Params[0], req.Function.Params[2], req.Function.Params[1])
	} else {
		return fmt.Errorf("invalid function: ", req.Function.Type)
	}
	result, err := tx.Run(query, map[string]interface{}{"list": req.ToCreate})
	if err != nil {
		return err
	}

	// iterate over the result
	for result.Next() {
		record := result.Record()
		fmt.Printf("Created Item with ID: %d\n", record.GetByIndex(0))
	}

	summary, e := result.Consume()
	if e != nil {
		return e
	}

	d := summary.ResultAvailableAfter()
	// set the duration
	req.Duration = &d

	// delete the items
	if req.Function.Type == CREATE_NODE {
		query = "UNWIND $list AS item MATCH (x:%s {external_id: item.external_id}) DETACH DELETE x"
		query = fmt.Sprintf(query, req.Function.Params[0])
	} else if req.Function.Type == CREATE_RELATION {
		query = ""
	} else {
		return fmt.Errorf("invalid function: ", req.Function.Type)
	}

	if query != "" {
		result, err = tx.Run(query, map[string]interface{}{"list": req.ToDelete})
		if err != nil {
			return err
		}

		// iterate over the result
		for result.Next() {
			record := result.Record()
			fmt.Printf("Deleted Item with ID: %d\n", record.GetByIndex(0))
		}

		summary, e = result.Consume()
		if e != nil {
			return e
		}

		d += summary.ResultAvailableAfter()
		// set the duration
		req.Duration = &d
	}

	// commit the transaction
	return tx.Commit()
}

func (s *neo4jStore) Close() error {
	if s == nil {
		return nil
	}

	s.session.Close()
	return s.driver.Close()
}
