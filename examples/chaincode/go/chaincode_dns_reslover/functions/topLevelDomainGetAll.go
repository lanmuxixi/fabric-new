package functions

import (
	"errors"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"
)

// TopLevelDomainGetAll lists all top-level-domain authority mappings stored in the ledger.
func TopLevelDomainGetAll(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 0 {
		return nil, errors.New("incorrect number of arguments, Expecting 0 args")
	}

	iter, err := stub.RangeQueryState("", "")
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	mappings := make(map[string]string)
	for iter.HasNext() {
		key, value, err := iter.Next()
		if err != nil {
			return nil, err
		}
		mappings[key] = string(value)
	}

	return myutils.BuildResponse(true, "", map[string]interface{}{
		"mappings": mappings,
	}), nil
}
