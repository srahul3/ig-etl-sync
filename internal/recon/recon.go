package recon

import (
	"encoding/json"
	"hash/crc32"

	"github.com/srahul3/cypher-transform/internal/model"
	"github.com/srahul3/cypher-transform/internal/store"
)

type Reconciler struct {
	// map to store integration and its corresponding functions
	integrationMap map[string]map[string]map[string][]interface{}
	crc32q         *crc32.Table
}

// NewReconciler creates a new Reconciler
func NewReconciler() *Reconciler {
	return &Reconciler{
		integrationMap: make(map[string]map[string]map[string][]interface{}),
		// In this package, the CRC polynomial is represented in reversed notation,
		// or LSB-first representation.
		//
		// LSB-first representation is a hexadecimal number with n bits, in which the
		// most significant bit represents the coefficient of x⁰ and the least significant
		// bit represents the coefficient of xⁿ⁻¹ (the coefficient for xⁿ is implicit).
		//
		// For example, CRC32-Q, as defined by the following polynomial,
		//	x³²+ x³¹+ x²⁴+ x²²+ x¹⁶+ x¹⁴+ x⁸+ x⁷+ x⁵+ x³+ x¹+ x⁰
		// has the reversed notation 0b11010101100000101000001010000001, so the value
		// that should be passed to MakeTable is 0xD5828281.
		crc32q: crc32.MakeTable(0xD5828281),
	}
}

func (r *Reconciler) Reconcile(
	integrationItem *model.IntegrationItem,
	function model.Function,
	data []map[string]interface{},
) (toDelete []map[string]interface{}, toCreate []map[string]interface{}, err error) {

	if function.Type == store.CREATE_RELATION {
		return nil, data, nil
	}

	integrationItemKey, err := integrationItem.GetKey()
	if err != nil {
		return nil, nil, err
	}

	// Check if the integration item exists in the map else create a new entry
	if _, ok := r.integrationMap[integrationItemKey]; !ok {
		r.integrationMap[integrationItemKey] = make(map[string]map[string][]interface{})
	}

	functionKey := function.GetKey()
	// Check if the function exists in the map else create a new entry
	if _, ok := r.integrationMap[integrationItemKey][functionKey]; !ok {
		r.integrationMap[integrationItemKey][functionKey] = make(map[string][]interface{})
	}

	// map storing external id vs crc32 hash
	previousItemsOrig := r.integrationMap[integrationItemKey][functionKey]

	// create copy of this map
	previousItems := make(map[string][]interface{})
	for k, v := range previousItemsOrig {
		previousItems[k] = v
	}

	// iterate over the data and add it to the map
	for _, d := range data {
		externalID := d["external_id"].(string)

		crc32Hash := r.GetCRC32(d)

		// check if the crc32 hash exists in the map
		// remove d from the map `previousItems` if it exists
		if _, ok := previousItems[externalID]; ok {
			// check if the crc32 hash is different
			if previousItems[externalID][0] != crc32Hash {
				// add to create list
				toCreate = append(toCreate, d)
			}

			delete(previousItems, externalID)
		} else {
			// add to create list
			toCreate = append(toCreate, d)
		}
	}

	// add the remaining items in the map `previousItems` to the delete list
	for k, _ := range previousItems {
		toDelete = append(toDelete, map[string]interface{}{"external_id": k})
	}

	return toDelete, toCreate, nil

}

func (r *Reconciler) Commit(integrationItem *model.IntegrationItem, function model.Function, deleted []map[string]interface{}, created []map[string]interface{}) error {
	if function.Type == store.CREATE_RELATION {
		return nil
	}

	integrationItemKey, err := integrationItem.GetKey()
	if err != nil {
		return err
	}

	functionKey := function.GetKey()

	// map storing external id vs crc32 hash
	orig := r.integrationMap[integrationItemKey][functionKey]

	// update the map with the new data
	for _, d := range created {
		externalID := d["external_id"].(string)
		crc32Hash := r.GetCRC32(d)
		orig[externalID] = []interface{}{crc32Hash}
	}

	// remove the deleted items from the map
	for _, d := range deleted {
		externalID := d["external_id"].(string)
		delete(orig, externalID)
	}

	return nil
}

func (r *Reconciler) GetCRC32(d map[string]interface{}) uint32 {
	var data []byte = nil
	if d["updated_at"] != nil && d["updated_at"] != "" {
		// crc 32 hash of updated_at
		data = []byte(d["updated_at"].(string))
	} else if d["updated-at"] != nil && d["updated-at"] != "" {
		// crc 32 hash of updated_at
		data = []byte(d["updated-at"].(string))
	} else if d["index"] != nil && d["index"] != "" {
		// crc 32 hash of index
		data = []byte(d["index"].(string))
	}

	if data == nil {
		// crc 32 hash of the entire data
		b, err := json.Marshal(d)
		if err != nil {
			panic(err)
		}
		data = b
	}

	return crc32.Checksum(data, r.crc32q)
}
