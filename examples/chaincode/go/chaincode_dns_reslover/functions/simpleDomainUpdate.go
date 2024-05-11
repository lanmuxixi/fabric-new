package functions

import (
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"
	"strings"
)

// SimpleDomainUpdate is a function to update the domain record from bind dns server
func SimpleDomainUpdate(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 2 {
		return nil, errors.New("incorrect number of arguments, Expecting 2 args")
	}
	result := true
	domain := args[0]
	server := args[1]
	isValidDomain := myutils.CheckValidDomain(domain)
	if !isValidDomain {
		return nil, errors.New("input is invalid domain")
	}

	// Get the resolver ip from blockchain
	topLevelDomain, err := myutils.GetTopLevelDomain(domain)
	if err != nil || len(topLevelDomain) == 0 {
		return nil, errors.New("failed to get top level domain")
	}

	authorityServer, err := stub.GetState(topLevelDomain)
	splits := strings.Split(string(authorityServer), ":")
	if len(splits) != 2 {
		return nil, fmt.Errorf("the state from chainblock of %s is error", topLevelDomain)
	}
	resolverIp := splits[0]
	resolverPort := splits[1]

	// Quest the dns record
	result, err = myutils.SendUpdate(topLevelDomain, domain, server, resolverIp, resolverPort)
	if err != nil {
		return nil, err
	}
	return myutils.BuildResponse(result, "", nil), nil
}
