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
	"fmt"
	"math/big"
	"os/exec"
	"sync"
	"time"

	"github.com/hyperledger/fabric/consensus"
	"github.com/hyperledger/fabric/consensus/util/events"
	pb "github.com/hyperledger/fabric/protos"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

type void struct {
}

var member void

//var bzDomainMap map[string]void
var domainMap *bzDomainMap

type bzDomainMap struct {
	mu     sync.Mutex
	domain map[string]void
}

func (m *bzDomainMap) Set(key string, value void) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.domain[key] = value
}

func (m *bzDomainMap) Get(key string) (void, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.domain[key]
	return value, ok
}
func NewBzDomainMap() *bzDomainMap {
	return &bzDomainMap{
		domain: make(map[string]void),
	}
}
func init() {
	domainMap = NewBzDomainMap()
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

	bzreqStore *bzrequestStore
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
	op.manager.Start()
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

	op.bzreqStore = newBzRequestStore()

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

func (op *obcBatch) submitToLeader(req *Request, domain string, username string) events.Event {
	// Broadcast the request to the network, in case we're in the wrong view
	//
	op.broadcastMsg(&BatchMessage{Payload: &BatchMessage_Request{Request: req}})
	//op.logAddTxFromRequest(req)
	op.reqStore.storeOutstanding(req)
	op.startTimerIfOutstandingRequests()
	if op.pbft.byzantine && username == TEST_INIT_JIM_NAME && op.bzreqStore.outstandingRequests.has(domain) {
		//存在bzreqStore没有对应的req的情况：收到同伙作恶req，自己还没作恶
		//_req, err := op.bzreqStore.outstandingRequests.get(domain)
		e, err := op.bzreqStore.outstandingRequests.get_e(domain)
		if err != nil {
			logger.Errorf("failed get req by domain in bzreqstore")
			return nil
		}
		val, ok := e.Value.(bzrequestContainer)
		if ok {
			_req := val.req
			if val.flag {
				//time.Sleep(5 * time.Millisecond)
				op.broadcastMsg(&BatchMessage{Payload: &BatchMessage_Request{Request: _req}})
			}
			//删了
			op.bzreqStore.remove(_req, domain)
			//op.logAddTxFromRequest(_req)
			op.reqStore.storeOutstanding(_req)
			op.startTimerIfOutstandingRequests()
		}
	}
	if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView {
		return op.leaderProcReq(req)
	}
	return nil
}

func (op *obcBatch) broadcastMsg(msg *BatchMessage) {
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

// =============================================================================
// functions specific to batch mode
// =============================================================================

func (op *obcBatch) leaderProcReq(req *Request) events.Event {
	// XXX check req sig
	digest := hash(req)
	logger.Debugf("Batch primary %d queueing new request %s", op.pbft.id, digest)
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

const (
	TEST_INIT_JIM_NAME       = "JIM"
	TEST_INIT_JIM_CERTFICATE = "-----BEGIN CERTIFICATE-----\nMIICCzCCAZGgAwIBAgIQAOpb0QCV/y0qdDtDHZEE7zAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDczODU2WhcNMjUwNTA4MDcz\nODU2WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5k\nJNUDBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL\n10LhxzijgaEwgZ4wHQYDVR0OBBYEFGEEKfoi8WRktgpNQ+5ZW1yWej0SMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBRhBCn6IvFkZLYKTUPuWVtclno9EjAKBggqhkjOPQQDAgNoADBlAjAcdM3n\nsALhS5ksNd9h/XVXNFrNcrR22OKq81YLh3OU2GdWzAzqt8XU6UJM/UpudWECMQDt\nU/WJhQvaVAMr8XUrxjKdUoNThMh3J/zEAp3CZyS2vFfJa8cJDzV8j3s8a//8eVk=\n-----END CERTIFICATE-----"
	TEST_INIT_JIM_PRIVATEKEY = "-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDDEzpnX/6bJHiAyX3YM\nsnjHAgflkru6J629fEXvXp9R3gvRoUyTVya275zul+u7irOgBwYFK4EEACKhZANi\nAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5kJNUD\nBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL10Lh\nxzg=\n-----END PRIVATE KEY-----"
	TEST_INIT_JIM_IP         = "4.4.4.4"
	//NONCE                    = "336710"
	TARGET = "000fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
)

type ECDSASignature struct {
	R, S *big.Int
}

type byzantineUser struct {
	name       string
	ip         string
	cert       string
	privateKey string
	domain     string
	domaintype string
	ttl        string
	signature  string
}

func newByzantineUser(_domain string, _domaintype string, _ttl string) *byzantineUser {
	u := &byzantineUser{
		name:       TEST_INIT_JIM_NAME,
		ip:         TEST_INIT_JIM_IP,
		cert:       TEST_INIT_JIM_CERTFICATE,
		privateKey: TEST_INIT_JIM_PRIVATEKEY,
		domain:     _domain,
		domaintype: _domaintype,
		ttl:        _ttl,
		signature:  "",
	}
	return u
}

// Sign 签名
func sign(certKey []byte, text string) string {

	block, _ := pem.Decode(certKey)
	if block == nil {
		//fmt.Printf("ERROR: block of decoded private key is nil\n")
		return ""
	}

	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		//fmt.Printf("ERROR: failed get ECDSA private key, %v\n", err)
		return ""
	}
	ecPrivKey := privKey.(*ecdsa.PrivateKey)

	hash := sha256.Sum256([]byte(text))
	r, s, err := ecdsa.Sign(rand.Reader, ecPrivKey, hash[:])
	if err != nil {
		//fmt.Printf("ERROR: failed to get signature, %v\n", err)
		return ""
	}

	// asn1 output DER format
	signature, err := asn1.Marshal(ECDSASignature{
		R: r,
		S: s,
	})
	if err != nil {
		//fmt.Printf("ERROR: asn1.Marshal ECDSA signature: %v\n", err)
		return ""
	}
	//fmt.Printf("%s\n", base64.StdEncoding.EncodeToString(signature))
	return base64.StdEncoding.EncodeToString(signature)

}

//func recordDomain(domain string) bool {
//	file, err := os.OpenFile("./domain.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
//	if err != nil {
//		return false
//	}
//	defer file.Close()
//
//	if _, err := file.WriteString(domain + "\n"); err != nil {
//		return false
//	}
//	return true
//}
//
//func isRecord(targetDomain string) (bool, error) {
//	// 打开文件
//	file, err := os.Open("./domain.txt")
//	if err != nil {
//		fmt.Println("Error:", err)
//		return false, err
//	}
//	defer file.Close()
//
//	// 创建一个 Scanner 来逐行读取文件
//	scanner := bufio.NewScanner(file)
//
//	// 逐行检查域名是否存在
//	for scanner.Scan() {
//		// 获取当前行的域名
//		domain := scanner.Text()
//
//		// 判断目标域名是否存在于当前行
//		if domain == targetDomain {
//			return true, nil
//		}
//	}
//	return false, nil
//
//}

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

func (op *obcBatch) dobyzantine(params []string) bool {
	//判断是否要抢占这个domain
	//TODO:根据某规则
	//只有不是JIM且没抢占过才true
	if params[4] != TEST_INIT_JIM_NAME {
		//没抢占过
		//ok, _ := isRecord(params[0])
		//if ok {
		//	//已经尝试抢占过
		//	return false
		//}
		_, ok := domainMap.Get(params[0])
		return !ok
	}
	//没抢占过，是jim
	return false
}
func getHash(data []byte) *big.Int {
	hash := sha256.Sum256(data)
	hash256 := new(big.Int)
	hash256.SetBytes(hash[:])

	//hash256str := fmt.Sprintf("%064x", hash256)
	//fmt.Printf("0x" + hash256str + "\n")
	return hash256
}
func getNonce(s string, c chan uint32) {
	target := new(big.Int)
	target.SetString(TARGET, 16)
	//fmt.Printf("target = 0x" + fmt.Sprintf("%064x", target) + "\n")
	var nonce uint32
	nonce = 0
	compact := fmt.Sprintf("%d%s", nonce, s)
	for getHash([]byte(compact)).Cmp(target) > 0 {
		nonce++
		compact = fmt.Sprintf("%d%s", nonce, s)
	}
	c <- nonce

}
func makeTXbyPow(bzuser *byzantineUser, c chan uint32) {
	//Args = ["domain", "ip", "type", "ttl", "username", "signature", "nonce", "target"]
	nonce := <-c
	cmd := "peer"
	zzm := viper.GetString("dns.chaincodeid")
	function := "TopLevelUpdate"

	args := []string{
		"chaincode",
		"invoke",
		"-n",
		zzm,
		"-c",
		fmt.Sprintf("{\"Function\": \"%s\", \"Args\": [\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%d\"]}", function, bzuser.domain, bzuser.ip, bzuser.domaintype, bzuser.ttl, bzuser.name, bzuser.signature, TARGET, nonce),
	}
	fmt.Println("=====================================================================")
	fmt.Println("抢注命令：", cmd, args)
	fmt.Println("=====================================================================")

	command := exec.Command(cmd, args...)

	output, err := command.CombinedOutput()
	if err != nil {
		fmt.Println("=====================================================================")
		fmt.Println("抢注命令执行失败:", err)
		fmt.Println("=====================================================================")

		return
	}
	fmt.Println("=====================================================================")
	fmt.Println("抢注命令执行成功:", string(output))

	if _, ok := domainMap.Get(bzuser.domain); !ok {
		domainMap.Set(bzuser.domain, member)
	}
	fmt.Println("=====================================================================")
}

func makeTXnoPow(bzuser *byzantineUser) {
	cmd := "peer"
	zzm := viper.GetString("dns.chaincodeid")
	function := "TopLevelUpdate"

	args := []string{
		"chaincode",
		"invoke",
		"-n",
		zzm,
		"-c",
		fmt.Sprintf("{\"Function\": \"%s\", \"Args\": [\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]}", function, bzuser.domain, bzuser.ip, bzuser.domaintype, bzuser.ttl, bzuser.name, bzuser.signature),
	}
	//fmt.Println("=====================================================================")
	//fmt.Println("抢注命令：", cmd, args)
	//fmt.Println("=====================================================================")
	exec.Command(cmd, args...)
	command := exec.Command(cmd, args...)

	output, err := command.CombinedOutput()
	if err != nil {
		fmt.Println("=====================================================================")
		fmt.Println("抢注命令执行失败:", err)
		fmt.Println("=====================================================================")

		return
	}
	fmt.Println("=====================================================================")
	fmt.Println("抢注命令执行成功:", string(output))
	if _, ok := domainMap.Get(bzuser.domain); !ok {
		domainMap.Set(bzuser.domain, member)
	}
	fmt.Println("=====================================================================")
}

//func (op *obcBatch) normalProcReq(ocMsg *pb.Message) events.Event {
//	req := op.txToReq(ocMsg.Payload)
//	return op.submitToLeader(req, "", "")
//}

func (op *obcBatch) tryByzantineForCTX(req *Request) events.Event {
	//只可能是从nvp或本地来的
	var tx pb.Transaction
	if err := proto.Unmarshal(req.Payload, &tx); err != nil {
		return nil
	}
	if tx.Type == pb.Transaction_CHAINCODE_INVOKE {
		ccis := &pb.ChaincodeInvocationSpec{}
		err := proto.Unmarshal(tx.Payload, ccis)
		if err != nil {
			return nil
		}
		ctormsg := ccis.GetChaincodeSpec().GetCtorMsg()
		stringargs := getStringArgs(ctormsg.Args)
		function, params := getFuncAndParams(stringargs)
		if function == "TopLevelUpdate" {
			//Args = ["domain", "ip", "type", "ttl", "username", "signature", "nonce", "target"]
			//domain := params[0]
			//ip := params[1]
			//recordtype := params[2]
			//ttl := params[3]
			//username := params[4]
			//signature := params[5]
			//nonce := params[6]
			//target := params[7]
			if op.dobyzantine(params) {
				//不是JIM且没抢占过 tip:只能是nvp发来的
				bzuser := newByzantineUser(params[0], params[2], params[3])
				bzuser.signature = sign([]byte(bzuser.privateKey), bzuser.domain)
				//op.logAddTxFromRequest(req)
				op.bzreqStore.storeOutstanding(req, bzuser.domain, true) //先缓存
				//byzantine for has pow
				//var c chan uint32
				c := make(chan uint32)
				go getNonce(function, c)
				go makeTXbyPow(bzuser, c)
				//go makeTXnoPow(bzuser)
				return nil
			} else {
				//不作恶
				//只会是抢占过了或者JIM
				if params[4] == TEST_INIT_JIM_NAME {
					return op.submitToLeader(req, params[0], params[4])
				}
			}
			//else domain不想要或已经抢占过了 冒充正常节点op.submitToLeader(req, "", "")
		}
	}
	//冒充正常节点
	return op.submitToLeader(req, "", "")
}

func (op *obcBatch) tryByzantineForCNS(req *Request) events.Event {
	//FOR Message_CONSENSUS
	var tx pb.Transaction
	if err := proto.Unmarshal(req.Payload, &tx); err != nil {
		return nil
	}

	if tx.Type == pb.Transaction_CHAINCODE_INVOKE {
		ccis := &pb.ChaincodeInvocationSpec{}
		err := proto.Unmarshal(tx.Payload, ccis)
		if err != nil {
			return nil
		}
		ctormsg := ccis.GetChaincodeSpec().GetCtorMsg()
		stringargs := getStringArgs(ctormsg.Args)
		function, params := getFuncAndParams(stringargs)
		if function == "TopLevelUpdate" {
			//Args = ["domain", "ip", "type", "ttl", "username", "signature", "nonce", "target"]
			//domain := params[0]
			//ip := params[1]
			//recordtype := params[2]
			//ttl := params[3]
			//username := params[4]
			//signature := params[5]
			//nonce := params[6]
			//target := params[7]
			if op.dobyzantine(params) {
				//不是jim且没抢占过
				bzuser := newByzantineUser(params[0], params[2], params[3])
				bzuser.signature = sign([]byte(bzuser.privateKey), bzuser.domain)
				//op.logAddTxFromRequest(req)
				op.bzreqStore.storeOutstanding(req, bzuser.domain, false) //先缓存
				//byzantine for has pow
				//var c chan uint32
				c := make(chan uint32)
				go getNonce(function, c)
				go makeTXbyPow(bzuser, c)
				//go makeTXnoPow(bzuser)
				//op.reqStore.storeOutstanding(req)
				return nil
			}
			//else 是jim或抢占过了， 冒充正常节点
		}
	}

	op.reqStore.storeOutstanding(req)
	op.startTimerIfOutstandingRequests()
	if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView {
		return op.leaderProcReq(req)
	}
	return nil
}

func (op *obcBatch) processMessage(ocMsg *pb.Message, senderHandle *pb.PeerID) events.Event {

	if ocMsg.Type == pb.Message_CHAIN_TRANSACTION {
		//从nvp或本地收到, 非主节点作恶
		//req种类 1. 没抢占过的 2. 抢占过了
		//1. 生成作恶req 2. 广播作恶req 3. 延缓广播/随机播/不播被作恶req
		req := op.txToReq(ocMsg.Payload)
		if op.pbft.byzantine && op.pbft.primary(op.pbft.view) != op.pbft.id {
			//是恶意但不是主节点
			return op.tryByzantineForCTX(req)
		}
		return op.submitToLeader(req, "", "")
	}

	if ocMsg.Type != pb.Message_CONSENSUS {
		//从vp广播收到
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

		if op.pbft.byzantine && op.pbft.primary(op.pbft.view) != op.pbft.id {
			//此处的req只可能是从外部来的
			//1. 同伙的
			//2. 正常节点的
			return op.tryByzantineForCNS(req)
		}
		//op.logAddTxFromRequest(req)
		op.reqStore.storeOutstanding(req)

		if (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView {
			return op.leaderProcReq(req)
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
				if msg := op.leaderProcReq(nreq); msg != nil {
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
