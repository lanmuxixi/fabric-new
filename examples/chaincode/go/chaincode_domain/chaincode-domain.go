package main

//WARNING - this chaincode's ID is hard-coded in chaincode_example04 to illustrate one way of
//calling chaincode from a chaincode. If this example is modified, chaincode_example04.go has
//to be modified as well with the new ID of chaincode_example02.
//chaincode_example05 show's how chaincode ID can be passed in as a parameter instead of
//hard-coding.

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"math/big"
	"strconv"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}
type void struct{}
type set map[string]void

// strategies 用于存储查询策略
var strategies map[string]func(stub shim.ChaincodeStubInterface, args []string) ([]byte, error)

const QUERY_OWNER_BY_DOMAIN = "getOwnerByDomain"
const QUERY_DOMAINS_BY_OWNER = "getDomainsByOwner"

func init() {
	strategies = make(map[string]func(stub shim.ChaincodeStubInterface, args []string) ([]byte, error))
	strategies[QUERY_DOMAINS_BY_OWNER] = GetOwnerByDomain
	strategies[QUERY_OWNER_BY_DOMAIN] = GetDomainsByOwner
}

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	var A, B string    // Entities
	var Aval, Bval set // Asset holdings
	var err error

	if len(args) != 4 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2")
	}

	// Initialize the chaincode
	A = args[0]
	Aval = make(set)
	B = args[2]
	Bval = make(set)

	// map2json
	AvalStr, _ := json.Marshal(Aval)
	BvalStr, _ := json.Marshal(Bval)

	// Write the state to the ledger
	err = stub.PutState(A, AvalStr)
	//err = stub.PutState()
	if err != nil {
		fmt.Printf("put state error")
		return nil, err
	}

	err = stub.PutState(B, BvalStr)
	if err != nil {
		fmt.Printf("put state error")
		return nil, err
	}

	return nil, nil
}

// Transaction makes payment of X units from A to B
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if function == "delete" {
		// Deletes an entity from its state
		return t.delete(stub, args)
	}

	var requester string     // owner, args[0]
	var requestDomain string // domain, args[1]
	var err error

	if len(args) != 4 {
		return nil, errors.New("incorrect number of arguments. Expecting 2")
	}

	requester = args[0]
	requestDomain = args[1]
	nonce := args[2]
	nbits, _ := strconv.ParseUint(args[3], 16, 32)
	// 1. check pow
	compact := requester + requestDomain + nonce
	fmt.Printf("compact is :" + compact + "\n")
	hash := GetHash([]byte(compact))
	//fmt.Printf("hash is : %d\n", hash)
	//targetstr := nbits2targetStr(uint32(nbits))
	//fmt.Println("target is :" + targetstr)
	if CheckProofOfWork(hash, uint32(nbits)) {
		fmt.Println("check proof of work pass")

		// owner, err := stub.GetState(requestDomain)
		// if err != nil {
		// 	return nil, errors.New("get state failed")
		// }
		// if len(owner) != 0 {
		// 	return nil, errors.New("domain already occupy")
		// 	//return nil, nil
		// }
		err = stub.PutState(requestDomain, []byte(requester))
		if err != nil {
			return nil, errors.New("failed to update the domain-owner relation")
		}

		// 3. 更新拥有者-域名集合的关系
		domainList, err := stub.GetState(requester)
		if err != nil {
			return nil, errors.New("failed to get domain list")
		}
		var domains set
		err = json.Unmarshal(domainList, &domains)
		if err != nil {
			return nil, errors.New("failed to unmarshal the domain list")
		}
		var member void
		domains[requestDomain] = member
		domainsJson, err := json.Marshal(domains)
		if err != nil {
			return nil, errors.New("failed to marshal the domains")
		}

		err = stub.PutState(requester, domainsJson)
		if err != nil {
			return nil, errors.New("failed to put state")
		}

		return nil, nil
	}
	err = errors.New("check proof of work failed")
	return nil, err
}

// Deletes an entity from state
func (t *SimpleChaincode) delete(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments. Expecting 1")
	}

	A := args[0]

	// Delete the key from the state in ledger
	err := stub.DelState(A)
	if err != nil {
		return nil, errors.New("Failed to delete state")
	}

	return nil, nil
}

// Query callback representing the query of a chaincode
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	if len(function) == 0 || strategies[function] == nil {
		return nil, errors.New("invalid query function name. Expecting \"query\"")
	}
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments, Expecting 1 args")
	}

	if response, err := strategies[function](stub, args); err != nil {
		return nil, err
	} else {
		fmt.Printf("Query Response:%s\n", response)
		return response, nil
	}

}

// GetOwnerByDomain 通过域名获取用户名
func GetOwnerByDomain(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	domain := args[0]
	owner, err := stub.GetState(domain)
	if err != nil {
		jsonResp := "{\"Error\":\"failed to get owner by domain-" + domain + "\"}"
		return nil, errors.New(jsonResp)
	}
	return owner, nil
}

// GetDomainsByOwner 通过用户名获取域名集合
func GetDomainsByOwner(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	owner := args[0]
	domainList, err := stub.GetState(owner)
	if err != nil {
		jsonResp := "{\"Error\":\"failed to get domain list by owner" + owner + "\"}"
		return nil, errors.New(jsonResp)
	}
	return domainList, nil
}

//SHA256(SHA256(CtorMsg + nonce)) < TARGET
func CheckProofOfWork(hash *big.Int, nbits uint32) bool {
	var pfNegative *bool
	var pfOverflow *bool
	target := nbits2target(nbits, pfNegative, pfOverflow)
	//if(pfNegative || target == 0 || pfOverflow || target > ?)
	fmt.Printf("target is 0x%064x\n", target)
	fmt.Printf("hash is 0x%064x\n", hash)
	result := hash.Cmp(target)
	if result < 1 {
		return true
	}
	//return false
	return true
}

func GetHash(data []byte) *big.Int {
	hash1 := sha256.Sum256(data)
	hash := sha256.Sum256([]byte(hash1[:]))
	hash256 := new(big.Int)
	hash256.SetBytes(hash[:])
	return hash256
}

func nbits2target(nBits uint32, pfNegative *bool, pfOverflow *bool) *big.Int {
	exponent := nBits >> 24
	mantissa := nBits & 0x007fffff

	var rtn *big.Int

	if exponent <= 3 {
		mantissa >>= uint(8 * (3 - exponent))
		rtn = new(big.Int).SetUint64(uint64(mantissa))
	} else {
		rtn = new(big.Int).SetUint64(uint64(mantissa))
		rtn.Lsh(rtn, uint(8*(exponent-3)))
	}

	//*pfNegative = mantissa != 0 && (nBits&0x00800000) != 0
	//
	//*pfOverflow = mantissa != 0 && ((exponent > 34) ||
	//	(mantissa > 0xff && exponent > 33) ||
	//	(mantissa > 0xffff && exponent > 32))

	return rtn
}

func nbits2targetStr(nBits uint32) string {
	var pfNegative *bool
	var pfOverflow *bool
	target := nbits2target(nBits, pfNegative, pfOverflow)
	targetStr := fmt.Sprintf("%064x", target)
	return "0x" + targetStr
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}
