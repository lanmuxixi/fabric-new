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

func mustTopLevelNonce(t *testing.T, domain string, authority string) string {
	t.Helper()
	nonce, err := util.FindTopLevelUpdateNonce(domain, authority, util.DefaultTopLevelPowTarget)
	if err != nil {
		t.Fatalf("failed to compute nonce: %v", err)
	}
	return nonce
}

func TestMaybeHijackTopLevelUpdateHoldsOriginalRequest(t *testing.T) {
	op := &obcBatch{
		pbft:           &pbftCore{id: 1},
		reqStore:       newRequestStore(),
		bzDomains:      make(map[string]struct{}),
		bzHeldRequests: make(map[string]*heldByzantineRequest),
	}

	req := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", "10.92.2.140:53", util.DefaultTopLevelPowTarget, mustTopLevelNonce(t, "example.org", "10.92.2.140:53"))
	hijacked, domainKey, err := op.maybeHijackTopLevelUpdate(req)
	if err != nil {
		t.Fatalf("maybeHijackTopLevelUpdate failed: %v", err)
	}
	if !hijacked {
		t.Fatal("expected request to be hijacked")
	}
	if domainKey != "org" {
		t.Fatalf("unexpected domain key: %s", domainKey)
	}
	if _, exists := op.bzHeldRequests["org"]; !exists {
		t.Fatal("expected original request to be held while pow is recomputed")
	}
	if !op.reqStore.pendingRequests.has(hash(req)) {
		t.Fatal("expected held request to be marked pending")
	}
}

func TestMaybeHijackTopLevelUpdateOnlyOncePerDomain(t *testing.T) {
	op := &obcBatch{
		pbft:           &pbftCore{id: 1},
		reqStore:       newRequestStore(),
		bzDomains:      make(map[string]struct{}),
		bzHeldRequests: make(map[string]*heldByzantineRequest),
	}

	first := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", "10.92.2.140:53", util.DefaultTopLevelPowTarget, mustTopLevelNonce(t, "example.org", "10.92.2.140:53"))
	if hijacked, _, err := op.maybeHijackTopLevelUpdate(first); err != nil {
		t.Fatalf("first hijack failed: %v", err)
	} else if !hijacked {
		t.Fatal("expected first request to be hijacked")
	}

	second := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "another.org", "10.92.2.150:53", util.DefaultTopLevelPowTarget, mustTopLevelNonce(t, "another.org", "10.92.2.150:53"))
	if hijacked, _, err := op.maybeHijackTopLevelUpdate(second); err != nil {
		t.Fatalf("second hijack failed: %v", err)
	} else if hijacked {
		t.Fatal("expected second request for the same top-level domain to be ignored")
	}
}

func TestMaybeHijackTopLevelDeleteClearsHijackMarker(t *testing.T) {
	op := &obcBatch{
		pbft:           &pbftCore{id: 1},
		reqStore:       newRequestStore(),
		bzDomains:      map[string]struct{}{"org": {}},
		bzHeldRequests: make(map[string]*heldByzantineRequest),
	}

	heldReq := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", "10.92.2.140:53", util.DefaultTopLevelPowTarget, mustTopLevelNonce(t, "example.org", "10.92.2.140:53"))
	op.bzHeldRequests["org"] = &heldByzantineRequest{original: heldReq, cancel: make(chan struct{})}
	op.reqStore.storePending(heldReq)

	deleteReq := newTopLevelInvokeRequest(t, byzantineTopLevelDelete, "example.org")
	if hijacked, _, err := op.maybeHijackTopLevelUpdate(deleteReq); err != nil {
		t.Fatalf("delete processing failed: %v", err)
	} else if hijacked {
		t.Fatal("delete should not itself be hijacked")
	}
	if len(op.bzHeldRequests) != 0 {
		t.Fatal("expected held request to be cleared by delete")
	}

	updateReq := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "another.org", "10.92.2.140:53", util.DefaultTopLevelPowTarget, mustTopLevelNonce(t, "another.org", "10.92.2.140:53"))
	if hijacked, domainKey, err := op.maybeHijackTopLevelUpdate(updateReq); err != nil {
		t.Fatalf("update after delete failed: %v", err)
	} else if !hijacked {
		t.Fatal("expected hijack marker to be cleared by delete")
	} else if domainKey != "org" {
		t.Fatalf("unexpected domain key: %s", domainKey)
	}
}

func TestByzantinePowReadyEventQueuesByzantineRequestFirst(t *testing.T) {
	op := &obcBatch{
		pbft:             &pbftCore{id: 1, byzantine: true},
		batchSize:        3,
		batchTimerActive: true,
		reqStore:         newRequestStore(),
		bzDomains:        make(map[string]struct{}),
		bzHeldRequests:   make(map[string]*heldByzantineRequest),
	}

	originalReq := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", "10.92.2.140:53", util.DefaultTopLevelPowTarget, mustTopLevelNonce(t, "example.org", "10.92.2.140:53"))
	op.bzDomains["org"] = struct{}{}
	op.bzHeldRequests["org"] = &heldByzantineRequest{original: originalReq, cancel: make(chan struct{})}

	byzantineNonce := mustTopLevelNonce(t, "example.org", defaultByzantineAuthority)
	byzantineReq := newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", defaultByzantineAuthority, util.DefaultTopLevelPowTarget, byzantineNonce)
	if event := op.ProcessEvent(byzantinePowReadyEvent{domainKey: "org", request: byzantineReq}); event != nil {
		t.Fatalf("expected batch to remain open, got %#v", event)
	}

	if len(op.batchStore) != 2 {
		t.Fatalf("expected 2 queued requests, got %d", len(op.batchStore))
	}

	firstFunction, firstArgs := readInvokeArgs(t, op.batchStore[0])
	if firstFunction != byzantineTopLevelFunction {
		t.Fatalf("unexpected first function: %s", firstFunction)
	}
	if len(firstArgs) != 4 || firstArgs[0] != "example.org" || firstArgs[1] != defaultByzantineAuthority {
		t.Fatalf("unexpected first request args: %#v", firstArgs)
	}

	secondFunction, secondArgs := readInvokeArgs(t, op.batchStore[1])
	if secondFunction != byzantineTopLevelFunction {
		t.Fatalf("unexpected second function: %s", secondFunction)
	}
	if len(secondArgs) != 4 || secondArgs[0] != "example.org" || secondArgs[1] != "10.92.2.140:53" {
		t.Fatalf("unexpected second request args: %#v", secondArgs)
	}
}

func TestViewChangedClearsByzantineMarkers(t *testing.T) {
	omni := *inertState
	omni.UnicastImpl = func(msg *pb.Message, receiverHandle *pb.PeerID) error { return nil }

	b := newObcBatch(0, loadConfig(), &omni)
	defer b.Close()
	b.StateUpdated(&checkpointMessage{seqNo: 0, id: inertState.GetBlockchainInfoBlobImpl()}, inertState.GetBlockchainInfoImpl())

	b.bzDomains["org"] = struct{}{}
	b.bzHeldRequests["org"] = &heldByzantineRequest{original: newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", "10.92.2.140:53", util.DefaultTopLevelPowTarget, mustTopLevelNonce(t, "example.org", "10.92.2.140:53")), cancel: make(chan struct{})}
	b.ProcessEvent(viewChangedEvent{})

	if len(b.bzDomains) != 0 {
		t.Fatalf("expected byzantine domain markers to be cleared on view change, got %#v", b.bzDomains)
	}
	if len(b.bzHeldRequests) != 0 {
		t.Fatalf("expected held requests to be cleared on view change, got %#v", b.bzHeldRequests)
	}
}

func TestStateUpdatedClearsByzantineMarkers(t *testing.T) {
	omni := *inertState
	omni.UnicastImpl = func(msg *pb.Message, receiverHandle *pb.PeerID) error { return nil }

	b := newObcBatch(0, loadConfig(), &omni)
	defer b.Close()
	b.StateUpdated(&checkpointMessage{seqNo: 0, id: inertState.GetBlockchainInfoBlobImpl()}, inertState.GetBlockchainInfoImpl())

	b.bzDomains["org"] = struct{}{}
	b.bzHeldRequests["org"] = &heldByzantineRequest{original: newTopLevelInvokeRequest(t, byzantineTopLevelFunction, "example.org", "10.92.2.140:53", util.DefaultTopLevelPowTarget, mustTopLevelNonce(t, "example.org", "10.92.2.140:53")), cancel: make(chan struct{})}
	b.ProcessEvent(stateUpdatedEvent{
		chkpt:  &checkpointMessage{seqNo: 0, id: inertState.GetBlockchainInfoBlobImpl()},
		target: inertState.GetBlockchainInfoImpl(),
	})

	if len(b.bzDomains) != 0 {
		t.Fatalf("expected byzantine domain markers to be cleared on state update, got %#v", b.bzDomains)
	}
	if len(b.bzHeldRequests) != 0 {
		t.Fatalf("expected held requests to be cleared on state update, got %#v", b.bzHeldRequests)
	}
}
