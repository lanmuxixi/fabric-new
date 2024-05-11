package main

//WARNING - this chaincode's ID is hard-coded in chaincode_example04 to illustrate one way of
//calling chaincode from a chaincode. If this example is modified, chaincode_example04.go has
//to be modified as well with the new ID of chaincode_example02.
//chaincode_example05 show's how chaincode ID can be passed in as a parameter instead of
//hard-coding.

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/util"
	"math/big"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}
type void struct{}
type set map[string]void

// strategies 用于存储查询策略
var strategies map[string]func(stub shim.ChaincodeStubInterface, args []string) ([]byte, error)

const INVOKEDOMAIN = "TopLevelUpdate"
const DELETEDOMAIN = "TopLevelDelete"
const DNS_RESLOVER_CHAINCODE = "e4014dae936dec87a63374e4d14d4a8c0b1c108b259579241774ba5ff781b2712be7e02497ff111b8b193bfb71db1d18e918a5db0cfb27e1dbb6f20a56c0467e"
const TARGET = "0000ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" //for TopLevelDelete nonce = 336710

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	return nil, nil
}

// Transaction makes payment of X units from A to B
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	//
	if function == DELETEDOMAIN {
		if len(args) != 1 {
			return nil, errors.New("incorrect number of arguments. Expecting 1")
		}
		//for delete
		// Args = ["domain"]
		chaincodeToCall := GetChaincodeToCall()
		f := DELETEDOMAIN
		invokeArgs := util.ToChaincodeArgs(f, args[0])
		response, err := stub.InvokeChaincode(chaincodeToCall, invokeArgs)
		if err != nil {
			errStr := fmt.Sprintf("Failed to invoke chaincode. Got error: %s", err.Error())
			fmt.Printf(errStr)
			return nil, errors.New(errStr)
		}
		fmt.Printf("Invoke chaincode successful. Got response %s", string(response))
		return nil, nil
	}

	if function == INVOKEDOMAIN {
		var nonce string // nonce args[2]

		if len(args) != 8 {
			return nil, errors.New("incorrect number of arguments. Expecting 6")
		}
		//Args = ["domain", "ip", "type", "ttl", "username", "signature", "nonce", "target"]

		nonce = args[6]

		if args[7] != TARGET {
			return nil, errors.New("target inconsistent with configuration!")
		}

		target := new(big.Int)
		target.SetString(args[7], 16)
		fmt.Printf("target = 0x" + fmt.Sprintf("%064x", target) + "\n")

		// check pow
		//compact := requestIP + requestDomain + nonce
		compact := nonce + function
		fmt.Printf("compact is :" + compact + "\n")
		hash := getHash([]byte(compact))
		if CheckProofOfWork(hash, target) {
			chaincodeToCall := GetChaincodeToCall()
			f := INVOKEDOMAIN
			invokeArgs := util.ToChaincodeArgs(f, args[0], args[1], args[2], args[3], args[4], args[5])
			response, err := stub.InvokeChaincode(chaincodeToCall, invokeArgs)
			if err != nil {
				errStr := fmt.Sprintf("Failed to invoke dns chaincode. Got error: %s", err.Error())
				fmt.Printf(errStr)
				return nil, errors.New(errStr)
			}
			fmt.Printf("Invoke chaincode successful. Got response %s", string(response))
		} else {
			return nil, errors.New("check proof of work not pass")
		}
		return nil, nil
	} else {
		return nil, errors.New("unknown function name")
	}

}

// Query callback representing the query of a chaincode
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	//Args = ["domain"]
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments. Expecting 1")
	}
	chaincodeToCall := GetChaincodeToCall()
	invokeArgs := util.ToChaincodeArgs(function, args[0])
	response, err := stub.QueryChaincode(chaincodeToCall, invokeArgs)
	if err != nil {
		errStr := fmt.Sprintf("Failed to query dns chaincode. Got error: %s", err.Error())
		fmt.Printf(errStr)
		return nil, errors.New(errStr)
	}
	return response, nil
}

//
func GetChaincodeToCall() string {
	//is dns_reslover chaincode id
	chainCodeToCall := DNS_RESLOVER_CHAINCODE
	return chainCodeToCall
}

//SHA256(SHA256(CtorMsg + nonce)) < TARGET
func CheckProofOfWork(hash *big.Int, target *big.Int) bool {
	result := hash.Cmp(target)
	if result < 1 {
		return true
	}
	return false
}

func getHash(data []byte) *big.Int {
	hash := sha256.Sum256(data)
	hash256 := new(big.Int)
	hash256.SetBytes(hash[:])

	hash256str := fmt.Sprintf("%064x", hash256)
	fmt.Printf("0x" + hash256str + "\n")
	return hash256
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}
