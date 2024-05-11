package main

//WARNING - this chaincode's ID is hard-coded in chaincode_example04 to illustrate one way of
//calling chaincode from a chaincode. If this example is modified, chaincode_example04.go has
//to be modified as well with the new ID of chaincode_example02.
//chaincode_example05 show's how chaincode ID can be passed in as a parameter instead of
//hard-coding.

import (
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/common"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/functions"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"
	"strings"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct{}
type void struct{}
type ResponseCode int

// strategies 用于存储查询策略
var strategies map[string]func(stub shim.ChaincodeStubInterface, args []string) ([]byte, error)

const (
	SimpleDomainQuest  = "resolve"
	SimpleDomainUpdate = "update"
	SimpleDomainDelete = "delete"
)

const (
	TopLevelUpdate = "TopLevelUpdate"
	TopLevelQuest  = "TopLevelQuest"
	TopLevelDelete = "TopLevelDelete"
	TopLevelGetAll = "TopLevelGetAll"
)

const (
	UserRegister = "UserRegister"
)

func init() {
	strategies = make(map[string]func(stub shim.ChaincodeStubInterface, args []string) ([]byte, error))

	// 非顶级域名操作
	strategies[SimpleDomainQuest] = functions.SimpleDomainResolve
	strategies[SimpleDomainDelete] = functions.SimpleDomainDelete
	strategies[SimpleDomainUpdate] = functions.SimpleDomainUpdate

	// 顶级域名操作
	strategies[TopLevelQuest] = functions.TopLevelDomainResolve
	strategies[TopLevelUpdate] = functions.TopLevelDomainUpdate
	strategies[TopLevelDelete] = functions.TopLevelDomainDelete
	strategies[TopLevelGetAll] = functions.TopLevelGetAllRecords

	// 用户操作
	strategies[UserRegister] = functions.UserRegister
}

// Init init the domain-ip relation
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	table := &shim.Table{
		Name: common.DNS_RECORD_TABLE_NAME,
		ColumnDefinitions: []*shim.ColumnDefinition{
			{Name: "name", Type: shim.ColumnDefinition_STRING, Key: true},
			{Name: "value", Type: shim.ColumnDefinition_STRING, Key: false},
			{Name: "type", Type: shim.ColumnDefinition_STRING, Key: false},
			{Name: "owner", Type: shim.ColumnDefinition_STRING, Key: false},
			{Name: "ttl", Type: shim.ColumnDefinition_INT32, Key: false},
		},
	}
	if err := stub.CreateTable(table.Name, table.ColumnDefinitions); err != nil {
		return nil, shim.ErrTableNotFound
	}

	var topLevelDomain, serverAddress string
	var err error
	for _, arg := range args {
		pairs := strings.Split(arg, ":")
		if len(pairs) != 3 && len(pairs) != 2 {
			return nil, errors.New("incorrect number of arguments. Expecting 2")
		}
		topLevelDomain = pairs[0]
		serverAddress = pairs[1]
		if len(pairs) == 2 {
			serverAddress = pairs[1] + ":53"
		} else {
			serverAddress = pairs[1] + ":" + pairs[2]
		}
		if topLevelDomain == "" || !myutils.CheckValidIp(serverAddress) {
			return nil, errors.New("input is invalid domain or ip")
		}
		// Write the state to the ledger
		record := common.TableRecord{
			RecordName:  topLevelDomain,
			RecordValue: serverAddress,
			RecordType:  common.DEAULT_TYPE,
			RecordOwner: common.DEAULT_OWNER,
			RecordTTL:   common.DEAFULT_TTL,
		}
		row := common.BuildRowFromRecord(record)
		if _, err = stub.InsertRow(common.DNS_RECORD_TABLE_NAME, row); err != nil {
			return nil, errors.New("failed to update the domain-owner relation")
		}
	}

	// 注册初始用户
	err = stub.PutState(common.TableUserPrefix+common.USER_ADMIN_NAME, []byte(common.USER_ADMIN_CERTFICATE))
	err = stub.PutState(common.TableUserPrefix+common.USER_JIM_NAME, []byte(common.USER_JIM_CERTFICATE))
	if err != nil {
		return nil, fmt.Errorf("init user failed %+v", err)
	}
	return myutils.BuildResponse(true, "", nil), nil
}

// Invoke update the domain-ip relation
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, functionName string, args []string) ([]byte, error) {
	if len(functionName) != 0 && strategies[functionName] != nil {
		return strategies[functionName](stub, args)
	}
	return myutils.BuildWrongResponse("unknown functions name"), nil
}

// Query query the domain-ip relation
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
