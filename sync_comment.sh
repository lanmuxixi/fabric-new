# 将 peer 的注释和 vp0 同步
cid=`docker ps | grep debug-vp0 | awk '{print $1}'`
dst="/go/src/github.com/hyperledger/fabric/peer"
docker cp peer $cid:$dst
dst="/go/src/github.com/hyperledger/fabric/consensus/pbft"
docker cp consensus/pbft $cid:$dst

zzm=`peer chaincode deploy -p github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02 -c '{"Function":"init", "Args": ["a","100", "b", "200"]}'` | awk '{print $3}'