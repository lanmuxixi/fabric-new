package functions

import (
	"errors"
	fabricutil "github.com/hyperledger/fabric/core/util"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// TopLevelDomainUpdate 添加权威域名映射
func TopLevelDomainUpdate(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 4 {
		return nil, errors.New("incorrect number of arguments, Expecting 4 args")
	}
	result := true
	domain := args[0]
	authorityServer := args[1]
	target := args[2]
	nonce := args[3]
	isValidDomain := myutils.CheckValidDomain(domain)
	if !isValidDomain {
		return nil, errors.New("input is invalid domain")
	}
	if err := fabricutil.ValidateTopLevelUpdatePow(domain, authorityServer, target, nonce); err != nil {
		return nil, err
	}
	topLevelDomain, err := myutils.GetTopLevelDomain(domain)
	if err != nil || len(topLevelDomain) == 0 {
		return nil, errors.New("failed to get top level domain")
	}
	currentAuthorityServer, err := stub.GetState(topLevelDomain)
	if err != nil {
		return nil, errors.New("failed to query the domain-owner relation")
	}
	if len(currentAuthorityServer) != 0 {
		return nil, errors.New("top level domain already registered")
	}
	if err := stub.PutState(topLevelDomain, []byte(authorityServer)); err != nil {
		result = false
		return nil, errors.New("failed to update the domain-owner relation")
	}
	return myutils.BuildResponse(result, "", nil), nil
}
