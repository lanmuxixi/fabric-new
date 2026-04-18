package main

import (
	"encoding/json"
	"errors"
	"sort"
	"testing"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/functions"
)

type responseEnvelope struct {
	Code int                    `json:"Code"`
	Msg  string                 `json:"Msg"`
	Data map[string]interface{} `json:"Data"`
}

type rangeQueryStub struct {
	*shim.MockStub
}

type stateIterator struct {
	stub *shim.MockStub
	keys []string
	idx  int
}

func (s *rangeQueryStub) RangeQueryState(startKey, endKey string) (shim.StateRangeQueryIteratorInterface, error) {
	keys := make([]string, 0, len(s.State))
	for key := range s.State {
		if startKey != "" && key < startKey {
			continue
		}
		if endKey != "" && key > endKey {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return &stateIterator{stub: s.MockStub, keys: keys}, nil
}

func (it *stateIterator) HasNext() bool {
	return it.idx < len(it.keys)
}

func (it *stateIterator) Next() (string, []byte, error) {
	if !it.HasNext() {
		return "", nil, errors.New("no more values")
	}
	key := it.keys[it.idx]
	it.idx++
	value, err := it.stub.GetState(key)
	return key, value, err
}

func (it *stateIterator) Close() error {
	return nil
}

func mustInit(t *testing.T, stub *shim.MockStub, args ...string) {
	t.Helper()
	if _, err := stub.MockInit("1", "init", args); err != nil {
		t.Fatalf("init failed: %v", err)
	}
}

func mustInvoke(t *testing.T, stub *shim.MockStub, function string, args ...string) responseEnvelope {
	t.Helper()
	bytes, err := stub.MockInvoke("1", function, args)
	if err != nil {
		t.Fatalf("invoke %s failed: %v", function, err)
	}
	return decodeResponse(t, bytes)
}

func mustQuery(t *testing.T, stub *shim.MockStub, function string, args ...string) responseEnvelope {
	t.Helper()
	bytes, err := stub.MockQuery(function, args)
	if err != nil {
		t.Fatalf("query %s failed: %v", function, err)
	}
	return decodeResponse(t, bytes)
}

func decodeResponse(t *testing.T, payload []byte) responseEnvelope {
	t.Helper()
	var response responseEnvelope
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("failed to decode response %s: %v", string(payload), err)
	}
	return response
}

func TestInitStoresAuthorityServers(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("dns", scc)

	mustInit(t, stub, "com:1.1.1.1", "cn:1.1.1.2:5353")

	if got := string(stub.State["com"]); got != "1.1.1.1:53" {
		t.Fatalf("unexpected com server: %s", got)
	}
	if got := string(stub.State["cn"]); got != "1.1.1.2:5353" {
		t.Fatalf("unexpected cn server: %s", got)
	}
}

func TestTopLevelQuestReturnsAuthorityServer(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("dns", scc)

	mustInit(t, stub, "com:1.1.1.1", "cn:1.1.1.2")

	response := mustQuery(t, stub, TopLevelQuest, "google.com")
	if response.Code != 0 {
		t.Fatalf("unexpected code: %+v", response)
	}
	if got := response.Data["authorityServer"]; got != "1.1.1.1:53" {
		t.Fatalf("unexpected authority server: %#v", got)
	}
}

func TestTopLevelUpdateAndDelete(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("dns", scc)

	mustInit(t, stub, "com:1.1.1.1")
	updateResp := mustInvoke(t, stub, TopLevelUpdate, "example.org", "2.2.2.2:53")
	if updateResp.Code != 0 {
		t.Fatalf("unexpected update response: %+v", updateResp)
	}
	if got := string(stub.State["org"]); got != "2.2.2.2:53" {
		t.Fatalf("unexpected stored org server: %s", got)
	}

	deleteResp := mustInvoke(t, stub, TopLevelDelete, "example.org")
	if deleteResp.Code != 0 {
		t.Fatalf("unexpected delete response: %+v", deleteResp)
	}
	if _, ok := stub.State["org"]; ok {
		t.Fatalf("org should have been deleted")
	}
}

func TestTopLevelGetAllReturnsMappings(t *testing.T) {
	scc := new(SimpleChaincode)
	baseStub := shim.NewMockStub("dns", scc)
	mustInit(t, baseStub, "com:1.1.1.1", "cn:1.1.1.2", "org:1.1.1.3")

	responseBytes, err := functions.TopLevelDomainGetAll(&rangeQueryStub{MockStub: baseStub}, nil)
	if err != nil {
		t.Fatalf("TopLevelDomainGetAll failed: %v", err)
	}
	response := decodeResponse(t, responseBytes)
	if response.Code != 0 {
		t.Fatalf("unexpected response: %+v", response)
	}

	mappings, ok := response.Data["mappings"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected mappings payload: %#v", response.Data["mappings"])
	}
	if len(mappings) != 3 {
		t.Fatalf("unexpected mapping size: %#v", mappings)
	}
	if mappings["cn"] != "1.1.1.2:53" || mappings["com"] != "1.1.1.1:53" || mappings["org"] != "1.1.1.3:53" {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
}

func TestUnknownFunctionReturnsErrorResponse(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("dns", scc)

	mustInit(t, stub, "com:1.1.1.1")
	response := mustQuery(t, stub, "unknown")
	if response.Code != 1 {
		t.Fatalf("unexpected response for unknown function: %+v", response)
	}
	if response.Msg != "unknown functions name" {
		t.Fatalf("unexpected error message: %s", response.Msg)
	}
}
