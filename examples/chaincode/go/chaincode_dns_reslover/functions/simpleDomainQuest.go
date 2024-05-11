package functions

import (
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"
	"strings"
)

// SimpleDomainResolve is a simple function to resolve the domain from bind dns server
func SimpleDomainResolve(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments, Expecting 2 args")
	}
	domain := args[0]
	isValidDomain := myutils.CheckValidDomain(domain)
	if !isValidDomain {
		return nil, errors.New("input is invalid domain")
	}

	// Get the resolver ip from blockchain
	topLevelDomain, err := myutils.GetTopLevelDomain(domain)
	if err != nil || len(topLevelDomain) == 0 {
		return nil, errors.New("failed to get top level domain")
	}
	authorityAddress, err := stub.GetState(topLevelDomain)
	splits := strings.Split(string(authorityAddress), ":")
	if len(splits) != 2 {
		return nil, fmt.Errorf("the state from chainblock of %s is error", topLevelDomain)
	}
	resolverIp := splits[0]
	resolverPort := splits[1]

	// Update the dns record
	result, err := myutils.SendQuest(domain, resolverIp, resolverPort)
	if err != nil {
		return nil, err
	}

	return myutils.BuildResponse(true, "", map[string]interface{}{
		"records": result,
	}), nil
}
