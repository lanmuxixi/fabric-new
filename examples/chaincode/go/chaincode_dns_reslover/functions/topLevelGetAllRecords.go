package functions

import (
	"errors"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/common"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"
)

// TopLevelDomainResolve resolves the top level domain
func TopLevelGetAllRecords(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	records, err := common.GetAllRecords(stub, common.DNS_RECORD_TABLE_NAME)
	if err != nil {
		return nil, errors.New("cannot get all records")
	}
	return myutils.BuildResponse(true, "", map[string]interface{}{"records": records}), nil
}
