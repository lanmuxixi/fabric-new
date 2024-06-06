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
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/util"
	"math/big"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/op/go-logging"

	"github.com/hyperledger/fabric/consensus"
	"github.com/hyperledger/fabric/consensus/util/events"
	pb "github.com/hyperledger/fabric/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

func init() {
	bzdomains = newBzDomainSet()
}

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

	persistForward

	//for byzantine store delay(want byzantine) req
	bzreqStore *bzrequestStore
	//bzbatchStore []*Request
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

	if op.pbft.byzantine {
		op.bzreqStore = newBzRequestStore()
		//bzUser = newByzantineUser()
	}

	op.deduplicator = newDeduplicator()

	op.idleChan = make(chan struct{})
	close(op.idleChan) // TODO remove eventually

	return op
}

// Close tells us to release resources we are holding
func (op *obcBatch) Close() {
	op.batchTimer.Halt()
	op.pbft.close()
}

//func (op *obcBatch) submitToLeader(req *Request) events.Event {
//	if op.pbft.byzantine && (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView {
//		return op.leaderProcNVPReq(req, true)
//	}
//	//op.broadcastMsg(&BatchMessage{Payload: &BatchMessage_Request{Request: req}})
//	//op.logAddTxFromRequest(req)
//	op.reqStore.storeOutstanding(req)
//	op.startTimerIfOutstandingRequests()
//	if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView {
//		return op.leaderProcNVPReq(req, false)
//	}
//	return nil
//}

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
	outstanding, pending := op.reqStore.len()
	fmt.Printf("id:%d, time:%v, outstanding len:%d, pending len:%d\n", op.pbft.id, util.CreateUtcTimestamp(), outstanding, pending)
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

// =============================================================================
// functions specific to batch mode
// =============================================================================

// =============================================================================
//	               FOR BYZANTINE
// =============================================================================
const (
	BYZANTINE_NAME       = "JIM"
	BYZANTINE_IP         = "4.4.4.4"
	BYZANTINE_CERT       = "-----BEGIN CERTIFICATE-----\nMIICCzCCAZGgAwIBAgIQAOpb0QCV/y0qdDtDHZEE7zAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDczODU2WhcNMjUwNTA4MDcz\nODU2WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5k\nJNUDBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL\n10LhxzijgaEwgZ4wHQYDVR0OBBYEFGEEKfoi8WRktgpNQ+5ZW1yWej0SMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBRhBCn6IvFkZLYKTUPuWVtclno9EjAKBggqhkjOPQQDAgNoADBlAjAcdM3n\nsALhS5ksNd9h/XVXNFrNcrR22OKq81YLh3OU2GdWzAzqt8XU6UJM/UpudWECMQDt\nU/WJhQvaVAMr8XUrxjKdUoNThMh3J/zEAp3CZyS2vFfJa8cJDzV8j3s8a//8eVk=\n-----END CERTIFICATE-----"
	BYZANTINE_PRIVATEKEY = "-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDDEzpnX/6bJHiAyX3YM\nsnjHAgflkru6J629fEXvXp9R3gvRoUyTVya275zul+u7irOgBwYFK4EEACKhZANi\nAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5kJNUD\nBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL10Lh\nxzg=\n-----END PRIVATE KEY-----"
	BYZANTINE_FUNC       = "TopLevelUpdate"
	BYZANTINE_TARGET     = "000007ffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
)

type void struct {
}

var member void

type bzDomainSet struct {
	mu      sync.Mutex
	domains map[string]void
}

func newBzDomainSet() *bzDomainSet {
	return &bzDomainSet{domains: make(map[string]void)}
}

var bzdomains *bzDomainSet

func (bzm *bzDomainSet) Set(key string, value void) {
	bzm.mu.Lock()
	defer bzm.mu.Unlock()
	bzm.domains[key] = value
}

func (bzm *bzDomainSet) Has(key string) bool {
	bzm.mu.Lock()
	defer bzm.mu.Unlock()
	_, exists := bzm.domains[key]
	return exists
}

// Delete 删除指定键
func (bzm *bzDomainSet) Delete(key string) {
	bzm.mu.Lock()
	defer bzm.mu.Unlock()
	delete(bzm.domains, key)
}

type byzantineUser struct {
	name       string
	ip         string
	cert       string
	privateKey string
	domain     string
	//victim     string
	//domainType string
	//ttl        string
	//signature  string
	//updateFunc string
	//powTarget  string
	//tryDomains *bzDomainSet
}

//var bzUser *byzantineUser

const PrivateKey = "-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDDEzpnX/6bJHiAyX3YM\nsnjHAgflkru6J629fEXvXp9R3gvRoUyTVya275zul+u7irOgBwYFK4EEACKhZANi\nAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5kJNUD\nBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL10Lh\nxzg=\n-----END PRIVATE KEY-----"

func newByzantineUser() *byzantineUser {
	u := &byzantineUser{
		name:       viper.GetString("dns.bzname"),
		ip:         viper.GetString("dns.bzip"),
		cert:       viper.GetString("dns.bzcert"),
		privateKey: PrivateKey,
		//updateFunc: viper.GetString("dns.updatefunction"),
		//powTarget:  viper.GetString("dns.powtarget"), // only use for pow
		//tryDomains: newBzDomainSet(),
	}
	return u
}

//func (bzu *byzantineUser) setByzantineUser(domain string, domaintype string, ttl string) {
//	//set more?
//	bzu.domain = domain
//	bzu.domainType = domaintype
//	bzu.ttl = ttl
//	bzu.signature = ssign([]byte(bzu.privateKey), bzu.domain)
//}

func makeTxNoPow(parmas []string) {
	signature := ssign([]byte(BYZANTINE_PRIVATEKEY), parmas[0])
	cmd := "peer"
	zzm := viper.GetString("dns.chaincodeid")
	args := []string{
		"chaincode",
		"invoke",
		"-n",
		zzm,
		"-c",
		fmt.Sprintf("{\"Function\": \"%s\", \"Args\": [\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]}",
			BYZANTINE_FUNC, parmas[0], BYZANTINE_IP, parmas[2], parmas[3], BYZANTINE_NAME, signature),
	}
	//fmt.Println("=====================================================================")
	//fmt.Println("抢注命令：", cmd, args)
	//fmt.Println("=====================================================================")

	command := exec.Command(cmd, args...)

	_, err := command.CombinedOutput()
	if err != nil {
		//fmt.Println("=====================================================================")
		//fmt.Println("抢注命令执行失败:", err)
		//fmt.Println("=====================================================================")
		return
	}
	//fmt.Println("=====================================================================")
	//fmt.Println("抢注命令执行成功:", string(output))
	//bzu.tryDomains.Set(bzu.domain, member)
	bzdomains.Set(parmas[0], member)
	//fmt.Println("=====================================================================")
}

func makeTxByPow(parmas []string, c chan uint32) {
	nonce := <-c
	signature := ssign([]byte(BYZANTINE_PRIVATEKEY), parmas[0])
	cmd := "peer"
	zzm := viper.GetString("dns.chaincodeid")
	args := []string{
		"chaincode",
		"invoke",
		"-n",
		zzm,
		"-c",
		fmt.Sprintf("{\"Function\": \"%s\", \"Args\": [\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%d\"]}",
			BYZANTINE_FUNC, parmas[0], BYZANTINE_IP, parmas[2], parmas[3], BYZANTINE_NAME, signature, BYZANTINE_TARGET, nonce),
	}
	//fmt.Println("=====================================================================")
	//fmt.Println("抢注命令：", cmd, args)
	//fmt.Println("=====================================================================")

	command := exec.Command(cmd, args...)

	_, err := command.CombinedOutput()
	if err != nil {
		//fmt.Println("=====================================================================")
		//fmt.Println("抢注命令执行失败:", err)
		//fmt.Println("=====================================================================")
		return
	}
	//fmt.Println("=====================================================================")
	//fmt.Println("抢注命令执行成功:", string(output))
	//bzu.tryDomains.Set(bzu.domain, member)
	bzdomains.Set(parmas[0], member)
	//fmt.Println("=====================================================================")
}

type ECDSASignature struct {
	R, S *big.Int
}

func ssign(certKey []byte, text string) string {

	block, _ := pem.Decode(certKey)
	if block == nil {
		return ""
	}

	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return ""
	}
	ecPrivKey := privKey.(*ecdsa.PrivateKey)

	hash := sha256.Sum256([]byte(text))
	r, s, err := ecdsa.Sign(rand.Reader, ecPrivKey, hash[:])
	if err != nil {
		return ""
	}
	signature, err := asn1.Marshal(ECDSASignature{
		R: r,
		S: s,
	})
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(signature)

}

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
	hash := sha256.Sum256(data)
	//hash := sha256.Sum256([]byte(hash1[:]))
	hash256 := new(big.Int)
	hash256.SetBytes(hash[:])
	return hash256
}

func getNonce(s string, c chan uint32) {
	start := time.Now()
	target := new(big.Int)
	target.SetString(BYZANTINE_TARGET, 16)
	//fmt.Printf("target = 0x" + fmt.Sprintf("%064x", target) + "\n")
	var nonce uint32
	nonce = 0
	compact := fmt.Sprintf("%d%s", nonce, s)
	for getHash([]byte(compact)).Cmp(target) > 0 {
		nonce++
		compact = fmt.Sprintf("%d%s", nonce, s)
	}
	end := time.Now()
	elapsed := end.Sub(start)
	fmt.Printf("get nonce spend %s\n", elapsed)
	c <- nonce
}

func getStringArgs(args [][]byte) []string {
	strargs := make([]string, 0, len(args))
	for _, barg := range args {
		strargs = append(strargs, string(barg))
	}
	return strargs
}
func getFuncAndParams(stringargs []string) (function string, params []string) {
	function = ""
	params = []string{}
	if len(stringargs) >= 1 {
		function = stringargs[0]
		params = stringargs[1:]
	}
	return
}

func getArgsFromReq(req *Request) ([]string, error) {
	txbyte := req.Payload
	var tx pb.Transaction
	err := proto.Unmarshal(txbyte, &tx)
	if err != nil {
		logger.Debugf("try to byzantine, but can't unmarshal tx")
		return nil, errors.New("try to byzantine, but can't unmarshal tx")
	}
	ccis := &pb.ChaincodeInvocationSpec{}
	err = proto.Unmarshal(tx.Payload, ccis)
	if err != nil {
		return nil, errors.New("try to byzantine, but can't unmarshal tx payload")
	}
	ctormsg := ccis.GetChaincodeSpec().GetCtorMsg()
	stringargs := getStringArgs(ctormsg.Args)
	return stringargs, nil
}

func dobyzantine(req *Request) (do bool, ismy bool) {
	//只要不是恶意req都抢
	//TODO:根据某规则
	txbyte := req.Payload
	var tx pb.Transaction
	err := proto.Unmarshal(txbyte, &tx)
	if err != nil {
		logger.Debugf("try to byzantine, but can't unmarshal tx")
		return false, false
	}
	if tx.Type == pb.Transaction_CHAINCODE_INVOKE {
		//fmt.Printf("tx is %v", tx)
		ccis := &pb.ChaincodeInvocationSpec{}
		err = proto.Unmarshal(tx.Payload, ccis)
		if err != nil {
			return false, false
		}
		ctormsg := ccis.GetChaincodeSpec().GetCtorMsg()

		stringargs := getStringArgs(ctormsg.Args)
		function, params := getFuncAndParams(stringargs)
		if function == BYZANTINE_FUNC {
			if params[4] != BYZANTINE_NAME {
				return true, false
			}
			return false, true
		}
	}
	return false, false
}
func (op *obcBatch) NormalProcReq(req *Request) events.Event {

	op.batchStore = append(op.batchStore, req)
	op.reqStore.storePending(req)

	if !op.batchTimerActive {
		op.startBatchTimer()
	}

	if len(op.batchStore) >= op.batchSize {
		return op.sendBatch()
	}

	return nil
}
func canCreateGoroutine() bool {
	// 获取当前的goroutine数量
	numGoroutine := runtime.NumGoroutine()

	// 获取当前操作系统线程的数量
	maxProcs := runtime.GOMAXPROCS(0)

	// 如果当前的goroutine数量小于操作系统线程的数量，则说明还有可用线程
	return numGoroutine < maxProcs
}

//func (op *obcBatch) leaderProcReq(req *Request) events.Event {
//
//	if op.pbft.byzantine {
//		//此时可能发生恶意替换
//		//获取domain
//		digest := hash(req)
//		logger.Debugf("Batch primary %d queueing new request %s", op.pbft.id, digest)
//		txbyte := req.Payload
//		var tx pb.Transaction
//		_ = proto.Unmarshal(txbyte, &tx)
//		ccis := &pb.ChaincodeInvocationSpec{}
//		_ = proto.Unmarshal(tx.Payload, ccis)
//		ctormsg := ccis.GetChaincodeSpec().GetCtorMsg()
//		stringargs := getStringArgs(ctormsg.Args)
//		function, params := getFuncAndParams(stringargs)
//		if tx.Type == pb.Transaction_CHAINCODE_INVOKE && function == BYZANTINE_FUNC {
//			if params[4] != BYZANTINE_NAME && !bzdomains.Has(params[0]) {
//				op.bzreqStore.storeOutstanding(req, params[0])
//				c := make(chan uint32)
//				go getNonce(function, c)
//				go makeTxByPow(params, c)
//				//go makeTxNoPow(params)
//				return nil
//			} else {
//				op.reqStore.storeOutstanding(req)
//				bzdomains.Set(params[0], member)
//				return op.NormalProcReq(req)
//			}
//		}
//	}
//	op.reqStore.storeOutstanding(req)
//	return op.NormalProcReq(req)
//}

func (op *obcBatch) bzLeaderProcReq(function string, params []string, tx pb.Transaction, req *Request) events.Event {
	if params[4] != BYZANTINE_NAME {
		if !bzdomains.Has(params[0]) {
			op.bzreqStore.storeOutstanding(req, params[0])
			c := make(chan uint32)
			go getNonce(function, c)
			go makeTxByPow(params, c)
			//go makeTxNoPow(params)
			return nil
		} else {
			op.reqStore.storeOutstanding(req)
			op.startTimerIfOutstandingRequests()
			return op.NormalProcReq(req)
		}
	} else {
		op.broadcastMsg(&BatchMessage{Payload: &BatchMessage_Request{Request: req}})
		op.reqStore.storeOutstanding(req)
		//op.startTimerIfOutstandingRequests()
		op.batchStore = append(op.batchStore, req)
		op.reqStore.storePending(req)
		if op.bzreqStore.outstandingRequests.has(params[0]) {
			//有之前被作恶的
			_req, err := op.bzreqStore.outstandingRequests.get(params[0])
			op.bzreqStore.remove(_req, params[0])
			if err != nil {
				logger.Errorf("failed get req by domain in bzreqstore")
				return nil
			}
			//取出之前的 1. submitToLeader中广播 2. NormalProc
			//op.broadcastMsg(&BatchMessage{Payload: &BatchMessage_Request{Request: _req}})
			op.reqStore.storeOutstanding(_req)
			//op.startTimerIfOutstandingRequests()
			op.batchStore = append(op.batchStore, _req)
			op.reqStore.storePending(_req)
		}
		if !op.batchTimerActive {
			op.startBatchTimer()
		}

		if len(op.batchStore) >= op.batchSize {
			//>=500处理？
			return op.sendBatch()
		}
		return nil
	}
	//op.reqStore.storeOutstanding(req)
	//op.startTimerIfOutstandingRequests()
	//return op.NormalProcReq(req)
}

// =============================================================================
//				              FOR BYZANTINE
// =============================================================================

func (op *obcBatch) sendBatch() events.Event {
	op.stopBatchTimer()
	if len(op.batchStore) == 0 {
		logger.Error("Told to send an empty batch store for ordering, ignoring")
		return nil
	}

	reqBatch := &RequestBatch{Batch: op.batchStore}
	op.batchStore = nil
	logger.Infof("%v:Creating batch with %d requests", util.CreateUtcTimestamp(), len(reqBatch.Batch))
	return reqBatch
}

func (op *obcBatch) txToReq(tx []byte) *Request {
	var _tx pb.Transaction
	_ = proto.Unmarshal(tx, &_tx)
	req := &Request{
		Timestamp: _tx.Timestamp,
		Payload:   tx,
		ReplicaId: 0,
	}
	// XXX sign req
	return req
}

func (op *obcBatch) processMessage(ocMsg *pb.Message, senderHandle *pb.PeerID) events.Event {
	//fmt.Printf("recive msg from %s, type is %s\n", senderHandle.Name, ocMsg.Type)
	if ocMsg.Type == pb.Message_CHAIN_TRANSACTION {
		req := op.txToReq(ocMsg.Payload)
		if op.pbft.byzantine {
			//此时可能发生恶意替换
			//获取domain
			txbyte := req.Payload
			var tx pb.Transaction
			_ = proto.Unmarshal(txbyte, &tx)

			ccis := &pb.ChaincodeInvocationSpec{}
			_ = proto.Unmarshal(tx.Payload, ccis)
			ctormsg := ccis.GetChaincodeSpec().GetCtorMsg()
			stringargs := getStringArgs(ctormsg.Args)
			function, params := getFuncAndParams(stringargs)
			if tx.Type == pb.Transaction_CHAINCODE_INVOKE && function == BYZANTINE_FUNC {
				if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView {
					return op.bzLeaderProcReq(function, params, tx, req)
				}
				if params[4] == BYZANTINE_NAME {
					op.broadcastMsg(&BatchMessage{Payload: &BatchMessage_Request{Request: req}})
				}
				op.reqStore.storeOutstanding(req)
				op.startTimerIfOutstandingRequests()
				return nil
			}
		}
		op.reqStore.storeOutstanding(req)
		op.startTimerIfOutstandingRequests()
		if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView {
			return op.NormalProcReq(req)
		}
		return nil
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
		//op.logAddTxFromRequest(req)

		op.reqStore.storeOutstanding(req)
		if (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView {
			return op.NormalProcReq(req)
		}
		op.startTimerIfOutstandingRequests()
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
				if msg := op.NormalProcReq(nreq); msg != nil {
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
	case *Commit:
		// TODO, this is extremely hacky, but should go away when batch and core are merged
		res := op.pbft.ProcessEvent(event)
		op.startTimerIfOutstandingRequests()
		return res
	case viewChangedEvent:
		op.batchStore = nil
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
