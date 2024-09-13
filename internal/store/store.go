package store

import "github.com/srahul3/cypher-transform/internal/model"

const (
	// CREATE_NODE is a function to create a node in the graph database
	CREATE_NODE = "CREATE_NODE"
	// CREATE_RELATION is a function to create a relation between two nodes in the graph database
	CREATE_RELATION = "CREATE_RELATION"
)

type Store interface {
	Connect() error
	Setup() error
	Write(*model.WriteRequest) error
	Close() error
}
