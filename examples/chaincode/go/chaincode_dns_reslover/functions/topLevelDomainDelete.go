package functions

import (
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/common"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// TopLevelDomainDelete deletes the top level domain from the state in ledger
func TopLevelDomainDelete(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 3 {
		return nil, errors.New("incorrect number of arguments. Expecting 3")
	}

	requestDomain := args[0]
	owner := args[1]
	signature := args[2]
	topLevelDomain, err := myutils.GetTopLevelDomain(requestDomain)
	record := common.TableRecord{RecordName: topLevelDomain}
	if err != nil {
		return nil, errors.New("failed to get top level domain")
	}

	// check the content
	isValidContent, err := verifyContent(stub, owner, requestDomain, signature)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}
	if !isValidContent {
		return nil, fmt.Errorf("the content is invalid, please check it")
	}

	isSameUser, err := verifySameUser(stub, owner, topLevelDomain)
	if err != nil {
		return nil, fmt.Errorf("error occur when get the history record %+v", err)
	}
	if !isSameUser {
		return nil, fmt.Errorf("error occur when check the history record %s", owner)
	}

	// Delete the key from the state in ledger
	if err := common.DeleteRecord(stub, record); err != nil {
		return nil, fmt.Errorf("failed to delete state + %v", err)
	}

	return myutils.BuildResponse(true, "", nil), nil
}
