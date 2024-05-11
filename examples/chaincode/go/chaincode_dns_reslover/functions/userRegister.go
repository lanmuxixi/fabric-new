package functions

import (
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/common"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"
)

// UserRegister function
func UserRegister(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 2 {
		return nil, errors.New("incorrect UserRegister number of arguments, Expecting 2 args")
	}
	// 1. exact args
	user := args[0]
	cert := args[1]

	// 2. get user from ledger
	u, err := stub.GetState(common.TableUserPrefix + user)
	if err != nil {
		return nil, fmt.Errorf("error occur when get user info")
	}
	if u != nil {
		return nil, fmt.Errorf("error occur because user have existed")
	}

	if !myutils.VerifyCertificate([]byte(cert)) {
		return nil, fmt.Errorf("error occur because certificate is not valid")
	}

	// 3. put user into ledger
	err = stub.PutState(common.TableUserPrefix+user, []byte(cert))
	if err != nil {
		return nil, fmt.Errorf("error occur when put user info")
	}
	return myutils.BuildResponse(true, "", nil), nil
}
