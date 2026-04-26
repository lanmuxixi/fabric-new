/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pbft

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/consensus"
	"github.com/hyperledger/fabric/consensus/util/events"
	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

type obcBatch struct {
	obcGeneric
	externalEventReceiver
	pbft        *pbftCore
	broadcaster *broadcaster

	batchSize        int
	batchStore       []*Request
	batchTimer       events.Timer
	batchTimerActive bool
	batchTimeout     time.Duration

	manager events.Manager // TODO, remove eventually, the event manager

	incomingChan chan *batchMessage // Queues messages for processing by main thread
	idleChan     chan struct{}      // Idle channel, to be removed

	reqStore *requestStore // Holds the outstanding and pending requests

	deduplicator *deduplicator

	bzDomains      map[string]struct{}
	bzHeldRequests map[string]*heldByzantineRequest

	persistForward
}

type heldByzantineRequest struct {
	original *Request
	cancel   chan struct{}
}

type batchMessage struct {
	msg    *pb.Message
	sender *pb.PeerID
}

// Event types

// batchMessageEvent is sent when a consensus message is received that is then to be sent to pbft
type batchMessageEvent batchMessage

// batchTimerEvent is sent when the batch timer expires
type batchTimerEvent struct{}

type byzantinePowReadyEvent struct {
	domainKey string
	request   *Request
}

type byzantinePowFailedEvent struct {
	domainKey string
}

const (
	byzantineTopLevelFunction   = "TopLevelUpdate"
	byzantineTopLevelDelete     = "TopLevelDelete"
	defaultByzantineAuthority   = "10.92.2.141:53"
)

func newObcBatch(id uint64, config *viper.Viper, stack consensus.Stack) *obcBatch {
	var err error

	op := &obcBatch{
		obcGeneric: obcGeneric{stack: stack},
	}

	op.persistForward.persistor = stack

	logger.Debugf("Replica %d obtaining startup information", id)

	op.manager = events.NewManagerImpl() // TODO, this is hacky, eventually rip it out
	op.manager.SetReceiver(op)
	etf := events.NewTimerFactoryImpl(op.manager)
	op.pbft = newPbftCore(id, config, op, etf)
	op.manager.Start() //go eventLoop, 开始事件处理
	blockchainInfoBlob := stack.GetBlockchainInfoBlob()
	op.externalEventReceiver.manager = op.manager
	op.broadcaster = newBroadcaster(id, op.pbft.N, op.pbft.f, op.pbft.broadcastTimeout, stack)
	op.manager.Queue() <- workEvent(func() {
		op.pbft.stateTransfer(&stateUpdateTarget{
			checkpointMessage: checkpointMessage{
				seqNo: op.pbft.lastExec,
				id:    blockchainInfoBlob,
			},
		})
	})

	op.batchSize = config.GetInt("general.batchsize")
	op.batchStore = nil
	op.batchTimeout, err = time.ParseDuration(config.GetString("general.timeout.batch"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse batch timeout: %s", err))
	}
	logger.Infof("PBFT Batch size = %d", op.batchSize)
	logger.Infof("PBFT Batch timeout = %v", op.batchTimeout)

	if op.batchTimeout >= op.pbft.requestTimeout {
		op.pbft.requestTimeout = 3 * op.batchTimeout / 2
		logger.Warningf("Configured request timeout must be greater than batch timeout, setting to %v", op.pbft.requestTimeout)
	}

	if op.pbft.requestTimeout >= op.pbft.nullRequestTimeout && op.pbft.nullRequestTimeout != 0 {
		op.pbft.nullRequestTimeout = 3 * op.pbft.requestTimeout / 2
		logger.Warningf("Configured null request timeout must be greater than request timeout, setting to %v", op.pbft.nullRequestTimeout)
	}

	op.incomingChan = make(chan *batchMessage)

	op.batchTimer = etf.CreateTimer()

	op.reqStore = newRequestStore()

	op.deduplicator = newDeduplicator()
	op.bzDomains = make(map[string]struct{})
	op.bzHeldRequests = make(map[string]*heldByzantineRequest)

	op.idleChan = make(chan struct{})
	close(op.idleChan) // TODO remove eventually

	return op
}

// Close tells us to release resources we are holding
func (op *obcBatch) Close() {
	op.batchTimer.Halt()
	op.pbft.close()
}

func (op *obcBatch) submitToLeader(req *Request) events.Event {
	// Broadcast the request to the network, in case we're in the wrong view
	//从客户端或者nvp收到的请求
	//if !op.pbft.byzantine {
	//	op.broadcastMsg(&BatchMessage{Payload: &BatchMessage_Request{Request: req}})
	//}
	op.broadcastMsg(&BatchMessage{Payload: &BatchMessage_Request{Request: req}})
	op.logAddTxFromRequest(req)
	op.reqStore.storeOutstanding(req)
	op.startTimerIfOutstandingRequests()
	if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView {
		return op.handleLeaderRequest(req)
	}
	return nil
}

func (op *obcBatch) broadcastMsg(msg *BatchMessage) { //msg.payload.request == req, req.payload == proto.marshal(tx)
	msgPayload, _ := proto.Marshal(msg)
	ocMsg := &pb.Message{
		Type:    pb.Message_CONSENSUS,
		Payload: msgPayload,
	}
	op.broadcaster.Broadcast(ocMsg)
}

// send a message to a specific replica
func (op *obcBatch) unicastMsg(msg *BatchMessage, receiverID uint64) {
	msgPayload, _ := proto.Marshal(msg)
	ocMsg := &pb.Message{
		Type:    pb.Message_CONSENSUS,
		Payload: msgPayload,
	}
	op.broadcaster.Unicast(ocMsg, receiverID)
}

// =============================================================================
// innerStack interface (functions called by pbft-core)
// =============================================================================

// multicast a message to all replicas
func (op *obcBatch) broadcast(msgPayload []byte) {
	op.broadcaster.Broadcast(op.wrapMessage(msgPayload))
}

// send a message to a specific replica
func (op *obcBatch) unicast(msgPayload []byte, receiverID uint64) (err error) {
	return op.broadcaster.Unicast(op.wrapMessage(msgPayload), receiverID)
}

func (op *obcBatch) sign(msg []byte) ([]byte, error) {
	return op.stack.Sign(msg)
}

// verify message signature
func (op *obcBatch) verify(senderID uint64, signature []byte, message []byte) error {
	senderHandle, err := getValidatorHandle(senderID)
	if err != nil {
		return err
	}
	return op.stack.Verify(senderHandle, signature, message)
}

// execute an opaque request which corresponds to an OBC Transaction
func (op *obcBatch) execute(seqNo uint64, reqBatch *RequestBatch) {
	var txs []*pb.Transaction
	for _, req := range reqBatch.GetBatch() {
		tx := &pb.Transaction{}
		if err := proto.Unmarshal(req.Payload, tx); err != nil {
			logger.Warningf("Batch replica %d could not unmarshal transaction %s", op.pbft.id, err)
			continue
		}
		logger.Debugf("Batch replica %d executing request with transaction %s from outstandingReqs, seqNo=%d", op.pbft.id, tx.Txid, seqNo)
		if outstanding, pending := op.reqStore.remove(req); !outstanding || !pending {
			logger.Debugf("Batch replica %d missing transaction %s outstanding=%v, pending=%v", op.pbft.id, tx.Txid, outstanding, pending)
		}
		txs = append(txs, tx)
		op.deduplicator.Execute(req)
	}
	meta, _ := proto.Marshal(&Metadata{seqNo})
	logger.Debugf("Batch replica %d received exec for seqNo %d containing %d transactions", op.pbft.id, seqNo, len(txs))
	op.stack.Execute(meta, txs) // This executes in the background, we will receive an executedEvent once it completes
}

//====================================================================================
//                            FOR  BYZANTINE
//====================================================================================
func nbits2target(nBits uint32) *big.Int {
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

	//判断负数和溢出
	//pfNegative := mantissa != 0 && (nBits&0x00800000) != 0
	//
	//pfOverflow := mantissa != 0 && ((exponent > 34) ||
	//	(mantissa > 0xff && exponent > 33) ||
	//	(mantissa > 0xffff && exponent > 32))

	return rtn
}
func getHash(data []byte) *big.Int {
	hash1 := sha256.Sum256(data)
	hash := sha256.Sum256([]byte(hash1[:]))
	hash256 := new(big.Int)
	hash256.SetBytes(hash[:])

	hash256str := fmt.Sprintf("%064x", hash256)
	fmt.Printf("0x" + hash256str + "\n")
	return hash256
}
func getNonce(nbits uint32, domain string, byzantineIP string) uint32 {
	target := nbits2target(nbits)
	fmt.Printf("target = 0x" + fmt.Sprintf("%064x", target) + "\n")
	var nonce uint32
	data := []byte(byzantineIP + domain)
	nonce = 0
	compact := fmt.Sprintf("%d%s", nonce, data)
	for getHash([]byte(compact)).Cmp(target) > 0 {
		fmt.Println(compact)
		nonce++
		compact = fmt.Sprintf("%d%s", nonce, data)
		if nonce > 200 { //真要挖矿？
			break
		}
	}
	return nonce
}
func makeTx(nonce uint32, domain string, ip string) (pb.Transaction, error) {
	tx := pb.Transaction{}
	return tx, nil
}

func getStringArgs(args [][]byte) []string {
	stringArgs := make([]string, 0, len(args))
	for _, arg := range args {
		stringArgs = append(stringArgs, string(arg))
	}
	return stringArgs
}

func getFunctionAndParams(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
}

func topLevelKey(domain string) string {
	lastDot := strings.LastIndex(domain, ".")
	if lastDot == -1 || lastDot == len(domain)-1 {
		return domain
	}
	return domain[lastDot+1:]
}

func (op *obcBatch) byzantineAuthority() string {
	if authority := viper.GetString("dns.bzauthority"); authority != "" {
		return authority
	}
	return defaultByzantineAuthority
}

func (op *obcBatch) releaseByzantineDomain(domainKey string) *Request {
	held, ok := op.bzHeldRequests[domainKey]
	if ok {
		delete(op.bzHeldRequests, domainKey)
		close(held.cancel)
		op.reqStore.pendingRequests.remove(held.original)
	}
	delete(op.bzDomains, domainKey)
	if held == nil {
		return nil
	}
	return held.original
}

func (op *obcBatch) takeHeldByzantineRequest(domainKey string) *Request {
	held, ok := op.bzHeldRequests[domainKey]
	if !ok {
		return nil
	}
	delete(op.bzHeldRequests, domainKey)
	return held.original
}

func (op *obcBatch) resetByzantineTracking() {
	for domainKey, held := range op.bzHeldRequests {
		delete(op.bzHeldRequests, domainKey)
		close(held.cancel)
	}
	op.bzDomains = make(map[string]struct{})
}

func (op *obcBatch) buildByzantineTopLevelRequest(txType pb.Transaction_Type, invocation *pb.ChaincodeInvocationSpec, domain string, target string, nonce string) (*Request, error) {
	rewritten := proto.Clone(invocation).(*pb.ChaincodeInvocationSpec)
	rewritten.ChaincodeSpec.CtorMsg.Args = util.ToChaincodeArgs(byzantineTopLevelFunction, domain, op.byzantineAuthority(), target, nonce)

	byzantineTx, err := pb.NewChaincodeExecute(rewritten, util.GenerateUUID(), txType)
	if err != nil {
		return nil, err
	}
	payload, err := proto.Marshal(byzantineTx)
	if err != nil {
		return nil, err
	}
	return op.txToReq(payload), nil
}

func (op *obcBatch) startByzantinePowWorker(domainKey string, txType pb.Transaction_Type, invocation *pb.ChaincodeInvocationSpec, domain string, target string, cancel <-chan struct{}) {
	authority := op.byzantineAuthority()
	go func() {
		targetValue, err := util.ParseTopLevelPowTarget(target)
		if err != nil {
			logger.Warningf("PBFT byzantine primary %d received invalid pow target for %s: %s", op.pbft.id, domainKey, err)
			op.manager.Queue() <- byzantinePowFailedEvent{domainKey: domainKey}
			return
		}

		for nonce := uint64(0); ; nonce++ {
			select {
			case <-cancel:
				return
			default:
			}

			nonceStr := fmt.Sprintf("%d", nonce)
			if !util.TopLevelUpdatePowMeetsTarget(domain, authority, nonceStr, targetValue) {
				continue
			}

			byzantineReq, err := op.buildByzantineTopLevelRequest(txType, invocation, domain, target, nonceStr)
			if err != nil {
				logger.Warningf("PBFT byzantine primary %d failed to build rewritten request for %s: %s", op.pbft.id, domainKey, err)
				op.manager.Queue() <- byzantinePowFailedEvent{domainKey: domainKey}
				return
			}

			op.manager.Queue() <- byzantinePowReadyEvent{domainKey: domainKey, request: byzantineReq}
			return
		}
	}()
}

func (op *obcBatch) maybeHijackTopLevelUpdate(req *Request) (bool, string, error) {
	tx := &pb.Transaction{}
	if err := proto.Unmarshal(req.Payload, tx); err != nil {
		return false, "", err
	}
	if tx.Type != pb.Transaction_CHAINCODE_INVOKE {
		return false, "", nil
	}

	invocation := &pb.ChaincodeInvocationSpec{}
	if err := proto.Unmarshal(tx.Payload, invocation); err != nil {
		return false, "", err
	}
	if invocation.GetChaincodeSpec() == nil || invocation.GetChaincodeSpec().GetCtorMsg() == nil {
		return false, "", nil
	}

	function, params := getFunctionAndParams(getStringArgs(invocation.GetChaincodeSpec().GetCtorMsg().Args))
	switch function {
	case byzantineTopLevelDelete:
		if len(params) == 1 {
			op.releaseByzantineDomain(topLevelKey(params[0]))
		}
		return false, "", nil
	case byzantineTopLevelFunction:
	default:
		return false, "", nil
	}

	if len(params) != 4 {
		return false, "", nil
	}

	domainKey := topLevelKey(params[0])
	if domainKey == "" {
		return false, "", nil
	}
	if params[1] == op.byzantineAuthority() {
		return false, "", nil
	}
	if _, exists := op.bzDomains[domainKey]; exists {
		return false, "", nil
	}

	if err := util.ValidateTopLevelUpdatePow(params[0], params[1], params[2], params[3]); err != nil {
		return false, "", nil
	}

	op.bzDomains[domainKey] = struct{}{}
	op.bzHeldRequests[domainKey] = &heldByzantineRequest{
		original: req,
		cancel:   make(chan struct{}),
	}
	op.reqStore.storePending(req)
	op.startByzantinePowWorker(domainKey, tx.Type, proto.Clone(invocation).(*pb.ChaincodeInvocationSpec), params[0], params[2], op.bzHeldRequests[domainKey].cancel)
	return true, domainKey, nil
}

func (op *obcBatch) queueBatchRequest(req *Request) {
	op.batchStore = append(op.batchStore, req)
	op.reqStore.storePending(req)
	if !op.batchTimerActive {
		op.startBatchTimer()
	}
}

func (op *obcBatch) maybeSendBatch() events.Event {
	if len(op.batchStore) >= op.batchSize {
		return op.sendBatch()
	}
	return nil
}

func (op *obcBatch) normalLeaderProcReq(req *Request) events.Event {
	digest := hash(req)
	logger.Debugf("Batch primary %d queueing new request %s", op.pbft.id, digest)
	op.queueBatchRequest(req)
	return op.maybeSendBatch()
}

func (op *obcBatch) handleLeaderRequest(req *Request) events.Event {
	if op.pbft.byzantine {
		hijacked, domainKey, err := op.maybeHijackTopLevelUpdate(req)
		if err != nil {
			logger.Warningf("PBFT byzantine primary %d failed to rewrite request: %s", op.pbft.id, err)
		} else if hijacked {
			logger.Warningf("PBFT byzantine primary %d delaying top level registration for %s while recomputing pow", op.pbft.id, domainKey)
			return nil
		}
	}

	return op.normalLeaderProcReq(req)
}

//====================================================================================
//                            FOR  BYZANTINE
//====================================================================================

// =============================================================================
// functions specific to batch mode
// =============================================================================
func (op *obcBatch) leaderProcReq(req *Request) events.Event {
	// XXX check req sig
	//主节点作恶处
	if op.pbft.byzantine {
		digest := hash(req)
		logger.Debugf("Batch primary %d queueing new request %s", op.pbft.id, digest)
		//此时可能发生恶意替换

		txbyte := req.Payload
		var tx pb.Transaction
		err := proto.Unmarshal(txbyte, &tx)
		if err != nil {
			logger.Debugf("try to byzantine, but can't unmarshal tx")
		}
		fmt.Println("tx is ", tx)
		//get domain and nbits
		nbits, err := strconv.ParseUint("1d00ffff", 16, 32)
		domain := string(tx.Payload)
		//get nonce
		byzantineIP := "7.7.7.7"
		nonce := getNonce(uint32(nbits), domain, byzantineIP)
		//make tx and broadcast?
		byzantineTx, err := makeTx(nonce, domain, byzantineIP)

		reqPayload, err := proto.Marshal(&byzantineTx)
		if err != nil {
			return nil
		}
		req = op.txToReq(reqPayload)
	}
	digest := hash(req)
	logger.Debugf("Batch primary %d queueing new request %s", op.pbft.id, digest)

	op.batchStore = append(op.batchStore, req)
	op.reqStore.storePending(req)

	if !op.batchTimerActive {
		op.startBatchTimer()
	}

	if len(op.batchStore) >= op.batchSize {
		//>=500处理？
		return op.sendBatch()
	}

	return nil
}

func (op *obcBatch) sendBatch() events.Event {
	op.stopBatchTimer()
	if len(op.batchStore) == 0 {
		logger.Error("Told to send an empty batch store for ordering, ignoring")
		return nil
	}

	reqBatch := &RequestBatch{Batch: op.batchStore}
	op.batchStore = nil
	logger.Infof("Creating batch with %d requests", len(reqBatch.Batch))
	return reqBatch
}

func (op *obcBatch) txToReq(tx []byte) *Request {
	now := time.Now()
	req := &Request{
		Timestamp: &timestamp.Timestamp{
			Seconds: now.Unix(),
			Nanos:   int32(now.UnixNano() % 1000000000),
		},
		Payload:   tx,
		ReplicaId: op.pbft.id,
	}
	// XXX sign req
	return req
}

func (op *obcBatch) processMessage(ocMsg *pb.Message, senderHandle *pb.PeerID) events.Event {
	if ocMsg.Type == pb.Message_CHAIN_TRANSACTION {
		req := op.txToReq(ocMsg.Payload) //封装request
		return op.submitToLeader(req)
	}

	if ocMsg.Type != pb.Message_CONSENSUS {
		logger.Errorf("Unexpected message type: %s", ocMsg.Type)
		return nil
	}

	batchMsg := &BatchMessage{}
	err := proto.Unmarshal(ocMsg.Payload, batchMsg)
	if err != nil {
		logger.Errorf("Error unmarshaling message: %s", err)
		return nil
	}

	if req := batchMsg.GetRequest(); req != nil {
		if !op.deduplicator.IsNew(req) {
			logger.Warningf("Replica %d ignoring request as it is too old", op.pbft.id)
			return nil
		}

		op.logAddTxFromRequest(req)
		op.reqStore.storeOutstanding(req) //存到队列
		//TODO：收到共识消息，非主节点也应该做点什么表示
		//一个用户想申请一个域名，应该计算nonce，通过用户名和域名
		//节点收到用户交易，应该对其进行保存？
		//为什么单播改成广播？因为单播主节点，主节点作恶会静默消息，发送自己的
		//如何避免？改成广播，每个节点都受到消息，之后呢？共识还是由主节点主持吗？
		//如果还是主节点主持，可以每个节点将这个消息保存起来，如果主节点作恶用自己的交易替代，可检查
		//如果不是主节点主持，直接自行开始共识？
		//主节点对request沉默会被换，主节点替换交易应被识别

		//作恶可能发生在这，某主节点收到共识消息
		if (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView {
			//leader收集固定数量交易打包成RequestBatch返回
			return op.handleLeaderRequest(req)

		}
		op.startTimerIfOutstandingRequests() //view change计时？主节点沉默
		return nil
	} else if pbftMsg := batchMsg.GetPbftMessage(); pbftMsg != nil {
		senderID, err := getValidatorID(senderHandle) // who sent this?
		if err != nil {
			panic("Cannot map sender's PeerID to a valid replica ID")
		}
		msg := &Message{}
		err = proto.Unmarshal(pbftMsg, msg)
		if err != nil {
			logger.Errorf("Error unpacking payload from message: %s", err)
			return nil
		}
		return pbftMessageEvent{
			msg:    msg,
			sender: senderID,
		}
	}

	logger.Errorf("Unknown request: %+v", batchMsg)

	return nil
}

func (op *obcBatch) logAddTxFromRequest(req *Request) {
	if logger.IsEnabledFor(logging.DEBUG) {
		// This is potentially a very large expensive debug statement, guard
		tx := &pb.Transaction{}
		err := proto.Unmarshal(req.Payload, tx)
		if err != nil {
			logger.Errorf("Replica %d was sent a transaction which did not unmarshal: %s", op.pbft.id, err)
		} else {
			logger.Debugf("Replica %d adding request from %d with transaction %s into outstandingReqs", op.pbft.id, req.ReplicaId, tx.Txid)
		}
	}
}

func (op *obcBatch) resubmitOutstandingReqs() events.Event {
	op.startTimerIfOutstandingRequests()

	// If we are the primary, and know of outstanding requests, submit them for inclusion in the next batch until
	// we run out of requests, or a new batch message is triggered (this path will re-enter after execution)
	// Do not enter while an execution is in progress to prevent duplicating a request
	if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView && op.pbft.currentExec == nil {
		needed := op.batchSize - len(op.batchStore)

		for op.reqStore.hasNonPending() {
			outstanding := op.reqStore.getNextNonPending(needed)

			// If we have enough outstanding requests, this will trigger a batch
			for _, nreq := range outstanding {
				if msg := op.handleLeaderRequest(nreq); msg != nil {
					op.manager.Inject(msg)
				}
			}
		}
	}
	return nil
}

// allow the primary to send a batch when the timer expires
func (op *obcBatch) ProcessEvent(event events.Event) events.Event {
	logger.Debugf("Replica %d batch main thread looping", op.pbft.id)
	switch et := event.(type) {
	case batchMessageEvent:
		ocMsg := et
		//msg交易，sender peerid
		return op.processMessage(ocMsg.msg, ocMsg.sender)
	case executedEvent:
		op.stack.Commit(nil, et.tag.([]byte))
	case committedEvent:
		logger.Debugf("Replica %d received committedEvent", op.pbft.id)
		return execDoneEvent{}
	case execDoneEvent:
		if res := op.pbft.ProcessEvent(event); res != nil {
			// This may trigger a view change, if so, process it, we will resubmit on new view
			return res
		}
		return op.resubmitOutstandingReqs()
	case batchTimerEvent:
		logger.Infof("Replica %d batch timer expired", op.pbft.id)
		if op.pbft.activeView && (len(op.batchStore) > 0) {
			return op.sendBatch()
		}
	case byzantinePowReadyEvent:
		originalReq := op.takeHeldByzantineRequest(et.domainKey)
		if originalReq == nil {
			return nil
		}
		if op.pbft.primary(op.pbft.view) != op.pbft.id || !op.pbft.activeView || !op.pbft.byzantine {
			return nil
		}

		logger.Warningf("PBFT byzantine primary %d forged pow for %s and is queueing the rewritten request first", op.pbft.id, et.domainKey)
		op.reqStore.storeOutstanding(et.request)
		op.queueBatchRequest(et.request)
		op.queueBatchRequest(originalReq)
		return op.maybeSendBatch()
	case byzantinePowFailedEvent:
		originalReq := op.releaseByzantineDomain(et.domainKey)
		if originalReq == nil {
			return nil
		}
		if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView {
			return op.normalLeaderProcReq(originalReq)
		}
		return nil
	case *Commit:
		// TODO, this is extremely hacky, but should go away when batch and core are merged
		res := op.pbft.ProcessEvent(event)
		op.startTimerIfOutstandingRequests()
		return res
	case viewChangedEvent:
		op.batchStore = nil
		op.resetByzantineTracking()
		// Outstanding reqs doesn't make sense for batch, as all the requests in a batch may be processed
		// in a different batch, but PBFT core can't see through the opaque structure to see this
		// so, on view change, clear it out
		op.pbft.outstandingReqBatches = make(map[string]*RequestBatch)

		logger.Debugf("Replica %d batch thread recognizing new view", op.pbft.id)
		if op.batchTimerActive {
			op.stopBatchTimer()
		}

		if op.pbft.skipInProgress {
			// If we're the new primary, but we're in state transfer, we can't trust ourself not to duplicate things
			op.reqStore.outstandingRequests.empty()
		}

		op.reqStore.pendingRequests.empty()
		for i := op.pbft.h + 1; i <= op.pbft.h+op.pbft.L; i++ {
			if i <= op.pbft.lastExec {
				continue
			}

			cert, ok := op.pbft.certStore[msgID{v: op.pbft.view, n: i}]
			if !ok || cert.prePrepare == nil {
				continue
			}

			if cert.prePrepare.BatchDigest == "" {
				// a null request
				continue
			}

			if cert.prePrepare.RequestBatch == nil {
				logger.Warningf("Replica %d found a non-null prePrepare with no request batch, ignoring")
				continue
			}

			op.reqStore.storePendings(cert.prePrepare.RequestBatch.GetBatch())
		}

		return op.resubmitOutstandingReqs()
	case stateUpdatedEvent:
		// When the state is updated, clear any outstanding requests, they may have been processed while we were gone
		op.reqStore = newRequestStore()
		op.resetByzantineTracking()
		return op.pbft.ProcessEvent(event)
	default:
		return op.pbft.ProcessEvent(event)
	}

	return nil
}

func (op *obcBatch) startBatchTimer() {
	op.batchTimer.Reset(op.batchTimeout, batchTimerEvent{})
	logger.Debugf("Replica %d started the batch timer", op.pbft.id)
	op.batchTimerActive = true
}

func (op *obcBatch) stopBatchTimer() {
	op.batchTimer.Stop()
	logger.Debugf("Replica %d stopped the batch timer", op.pbft.id)
	op.batchTimerActive = false
}

// Wraps a payload into a batch message, packs it and wraps it into
// a Fabric message. Called by broadcast before transmission.
func (op *obcBatch) wrapMessage(msgPayload []byte) *pb.Message {
	batchMsg := &BatchMessage{Payload: &BatchMessage_PbftMessage{PbftMessage: msgPayload}}
	packedBatchMsg, _ := proto.Marshal(batchMsg)
	ocMsg := &pb.Message{
		Type:    pb.Message_CONSENSUS,
		Payload: packedBatchMsg,
	}
	return ocMsg
}

// Retrieve the idle channel, only used for testing
func (op *obcBatch) idleChannel() <-chan struct{} {
	return op.idleChan
}

// TODO, temporary
func (op *obcBatch) getManager() events.Manager {
	return op.manager
}

func (op *obcBatch) startTimerIfOutstandingRequests() {
	if op.pbft.skipInProgress || op.pbft.currentExec != nil || !op.pbft.activeView {
		// Do not start view change timer if some background event is in progress
		logger.Debugf("Replica %d not starting timer because skip in progress or current exec or in view change", op.pbft.id)
		return
	}

	if !op.reqStore.hasNonPending() {
		// Only start a timer if we are aware of outstanding requests
		logger.Debugf("Replica %d not starting timer because all outstanding requests are pending", op.pbft.id)
		return
	}
	op.pbft.softStartTimer(op.pbft.requestTimeout, "Batch outstanding requests")
}
