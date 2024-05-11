package main

import (
	"encoding/hex"
	"fmt"
	"github.com/golang/protobuf/proto"
	pb "github.com/hyperledger/fabric/protos"
	"testing"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

func checkInit(t *testing.T, stub *shim.MockStub, args []string) {
	_, err := stub.MockInit("1", "init", args)
	if err != nil {
		fmt.Println("Init failed", err)
		t.FailNow()
	}
}

func checkState(t *testing.T, stub *shim.MockStub, name string, value string) {
	bytes := stub.State[name]
	if bytes == nil {
		fmt.Println("State", name, "failed to get value")
		t.FailNow()
	}
	if string(bytes) != value {
		fmt.Println("State value", name, "was not", value, "as expected")
		t.FailNow()
	}
}

func checkQuery(t *testing.T, stub *shim.MockStub, function string, name string, value string) {
	bytes, err := stub.MockQuery(function, []string{name})
	if err != nil {
		fmt.Println("Query", name, "failed", err)
		t.FailNow()
	}
	if bytes == nil {
		fmt.Println("Query", name, "failed to get value")
		if value != "" {
			t.FailNow()
		}
	}
	if string(bytes) != value {
		fmt.Println("Query value", name, "was not", value, "as expected")
		t.FailNow()
	}
}

func checkInvoke(t *testing.T, stub *shim.MockStub, args []string) {
	_, err := stub.MockInvoke("1", "invoke", args)
	if err != nil {
		fmt.Println("Invoke", args, "failed", err)
		t.FailNow()
	}
}

func TestExample02_Init(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)

	// Init A="" B=""
	checkInit(t, stub, []string{"A", "", "B", ""})

	checkState(t, stub, "A", "{}")
	checkState(t, stub, "B", "{}")
}

func TestExample02_Query(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)

	// Init A have no domain, B have no domain
	checkInit(t, stub, []string{"A", "", "B", ""})
	// Query A
	checkQuery(t, stub, "getDomainsByOwner", "A", "{}")
	// Query B
	checkQuery(t, stub, "getOwnerByDomain", "google.com", "")
}

func TestExample02_Invoke(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)

	// A and B have no domain
	checkInit(t, stub, []string{"1.1.1.1", "", "7.7.7.7", ""})

	// A get domain google.com
	checkInvoke(t, stub, []string{"1.1.1.1", "google.com", "8", "1d00ffff"})
	checkQuery(t, stub, "getOwnerByDomain", "google.com", "1.1.1.1")
	checkInvoke(t, stub, []string{"7.7.7.7", "google.com", "8", "1d00ffff"})
	checkQuery(t, stub, "getOwnerByDomain", "google.com", "1.1.1.1")
	checkInvoke(t, stub, []string{"1.1.1.1", "hello.com", "7", "1d00ffff"})
	//checkInvoke(t, stub, []string{"A", "google.com"})
	//checkInvoke(t, stub, []string{"A", "hello.com"})
	//checkInvoke(t, stub, []string{"B", "baidu.com"})
	checkQuery(t, stub, "getDomainsByOwner", "1.1.1.1", "{\"google.com\":{},\"hello.com\":{}}")
	checkQuery(t, stub, "getOwnerByDomain", "google.com", "1.1.1.1")
	checkQuery(t, stub, "getDomainsByOwner", "7.7.7.7", "{}")

}

func TestProto(t *testing.T) {
	hexString := "080112830112800133323161353538323033373661383635663062363639656631376539343932646538333965666463353466313831626465303863326532666334636163643032643033313231626335333564656234366363646365366536373838353637613539643864653866353265643035383839636239303566666533623765326435381a320a0e546f704c6576656c5570646174650a0a676f6f676c652e6f72670a0a312e312e312e333a35330a01410a053836343030"
	payload, err := hex.DecodeString(hexString)
	// process the metadata
	ccsp := pb.ChaincodeSpec{}
	err = proto.Unmarshal(payload, &ccsp)
	if err != nil {
		fmt.Printf("failed to unmarshal payload, %v", err)
	}

	fmt.Println("==========================================")
	fmt.Printf("ccsp: %v\n", ccsp)
	fmt.Printf("ccsp secure context: %v\n", ccsp.SecureContext)
	fmt.Println("==========================================")
}
