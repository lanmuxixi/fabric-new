package pbft

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
)

func newTopLevelInvokeRequest(t *testing.T, function string, args ...string) *Request {
	t.Helper()

	spec := &pb.ChaincodeSpec{
		Type: pb.ChaincodeSpec_GOLANG,
		ChaincodeID: &pb.ChaincodeID{
			Name: "dns-test",
		},
		CtorMsg: &pb.ChaincodeInput{
			Args: util.ToChaincodeArgs(append([]string{function}, args...)...),
		},
	}
	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}
	tx, err := pb.NewChaincodeExecute(invocation, util.GenerateUUID(), pb.Transaction_CHAINCODE_INVOKE)
	if err != nil {
		t.Fatalf("failed to build transaction: %v", err)
	}
	payload, err := proto.Marshal(tx)
	if err != nil {
		t.Fatalf("failed to marshal transaction: %v", err)
	}
	return &Request{Payload: payload, ReplicaId: 0}
}

func readInvokeArgs(t *testing.T, req *Request) (string, []string) {
	t.Helper()

	tx := &pb.Transaction{}
	if err := proto.Unmarshal(req.Payload, tx); err != nil {
		t.Fatalf("failed to unmarshal transaction: %v", err)
	}
	invocation := &pb.ChaincodeInvocationSpec{}
	if err := proto.Unmarshal(tx.Payload, invocation); err != nil {
		t.Fatalf("failed to unmarshal invocation spec: %v", err)
	}
	return getFunctionAndParams(getStringArgs(invocation.GetChaincodeSpec().GetCtorMsg().Args))
}

func TestMaybeHijackTopLevelUpdateRewritesAuthority(t *testing.T) {
	op := &obcBatch{
		pbft:      &pbftCore{id: 1},
		bzDomains: make(map[string]struct{}),
	}

	req := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", "10.92.2.140:53")
	byzantineReq, domainKey, hijacked, err := op.maybeHijackTopLevelUpdate(req)
	if err != nil {
		t.Fatalf("maybeHijackTopLevelUpdate failed: %v", err)
	}
	if !hijacked {
		t.Fatal("expected request to be hijacked")
	}
	if domainKey != "org" {
		t.Fatalf("unexpected domain key: %s", domainKey)
	}

	function, args := readInvokeArgs(t, byzantineReq)
	if function != byzantineTopLevelFunction {
		t.Fatalf("unexpected function: %s", function)
	}
	if len(args) != 2 {
		t.Fatalf("unexpected args: %#v", args)
	}
	if args[0] != "example.org" {
		t.Fatalf("unexpected domain: %s", args[0])
	}
	if args[1] != defaultByzantineAuthority {
		t.Fatalf("unexpected byzantine authority: %s", args[1])
	}
}

func TestMaybeHijackTopLevelUpdateOnlyOncePerDomain(t *testing.T) {
	op := &obcBatch{
		pbft:      &pbftCore{id: 1},
		bzDomains: make(map[string]struct{}),
	}

	first := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", "10.92.2.140:53")
	if _, _, hijacked, err := op.maybeHijackTopLevelUpdate(first); err != nil {
		t.Fatalf("first hijack failed: %v", err)
	} else if !hijacked {
		t.Fatal("expected first request to be hijacked")
	}

	second := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "another.org", "10.92.2.150:53")
	if _, _, hijacked, err := op.maybeHijackTopLevelUpdate(second); err != nil {
		t.Fatalf("second hijack failed: %v", err)
	} else if hijacked {
		t.Fatal("expected second request for the same top-level domain to be ignored")
	}
}

func TestMaybeHijackTopLevelDeleteClearsHijackMarker(t *testing.T) {
	op := &obcBatch{
		pbft:      &pbftCore{id: 1},
		bzDomains: map[string]struct{}{"org": {}},
	}

	deleteReq := newTopLevelInvokeRequest(t, byzantineTopLevelDelete, "example.org")
	if _, _, hijacked, err := op.maybeHijackTopLevelUpdate(deleteReq); err != nil {
		t.Fatalf("delete processing failed: %v", err)
	} else if hijacked {
		t.Fatal("delete should not itself be hijacked")
	}

	updateReq := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "another.org", "10.92.2.140:53")
	if _, domainKey, hijacked, err := op.maybeHijackTopLevelUpdate(updateReq); err != nil {
		t.Fatalf("update after delete failed: %v", err)
	} else if !hijacked {
		t.Fatal("expected hijack marker to be cleared by delete")
	} else if domainKey != "org" {
		t.Fatalf("unexpected domain key: %s", domainKey)
	}
}
