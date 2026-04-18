package main

import (
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/functions"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"
	"strings"
)

type SimpleChaincode struct{}

var strategies map[string]func(stub shim.ChaincodeStubInterface, args []string) ([]byte, error)

const SimpleDomainQuest = "resolve"
const SimpleDomainUpdate = "update"
const SimpleDomainDelete = "delete"

const TopLevelUpdate = "TopLevelUpdate"
const TopLevelQuest = "TopLevelQuest"
const TopLevelDelete = "TopLevelDelete"
const TopLevelGetAll = "TopLevelGetAll"

func init() {
	strategies = make(map[string]func(stub shim.ChaincodeStubInterface, args []string) ([]byte, error))

	strategies[SimpleDomainQuest] = functions.SimpleDomainResolve
	strategies[SimpleDomainDelete] = functions.SimpleDomainDelete
	strategies[SimpleDomainUpdate] = functions.SimpleDomainUpdate

	strategies[TopLevelQuest] = functions.TopLevelDomainResolve
	strategies[TopLevelUpdate] = functions.TopLevelDomainUpdate
	strategies[TopLevelDelete] = functions.TopLevelDomainDelete
	strategies[TopLevelGetAll] = functions.TopLevelDomainGetAll
}

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	var topLevelDomain, serverIp string
	for _, arg := range args {
		pairs := strings.Split(arg, ":")
		if len(pairs) != 3 && len(pairs) != 2 {
			return nil, errors.New("incorrect number of arguments. Expecting 2")
		}
		topLevelDomain = pairs[0]
		serverIp = pairs[1]
		if len(pairs) == 2 {
			serverIp = pairs[1] + ":53"
		} else {
			serverIp = pairs[1] + ":" + pairs[2]
		}
		if topLevelDomain == "" || !myutils.CheckValidIp(serverIp) {
			return nil, errors.New("input is invalid domain or ip")
		}
		if err := stub.PutState(topLevelDomain, []byte(serverIp)); err != nil {
			return nil, errors.New("failed to update the domain-owner relation")
		}
	}
	return myutils.BuildResponse(true, "", nil), nil
}

func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, functionName string, args []string) ([]byte, error) {
	if len(functionName) != 0 && strategies[functionName] != nil {
		return strategies[functionName](stub, args)
	}
	return myutils.BuildWrongResponse("unknown functions name"), nil
}

func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if len(function) != 0 && strategies[function] != nil {
		return strategies[function](stub, args)
	}
	return myutils.BuildWrongResponse("unknown functions name"), nil
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}
