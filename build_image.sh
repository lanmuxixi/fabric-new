# 链码ID
zzm="af494eb16992e0041dfe136a4b27503bb14e78406cfce70a75c9a9c31767c78f1b929e805377f70bac0fecaa0a19f62ff7f4a60472abe8f3a4c3679b03146cb5"

find ./txscripts/ -type f -name "*.sh" | xargs chmod +x 
find ./txscripts/ -type f -name "start*" | xargs chmod +x 

cd /home/qichang/pack_image/peer-image

cp -r /home/qichang/go/src/github.com/hyperledger/fabric/consensus .
cp -r /home/qichang/go/src/github.com/hyperledger/fabric/core .
cp -r /home/qichang/go/src/github.com/hyperledger/fabric/peer .
cp -r /home/qichang/go/src/github.com/hyperledger/fabric/protos .
rm -rf txscripts
cp -r /home/qichang/go/src/github.com/hyperledger/fabric/txscripts .
cp -r /home/qichang/go/src/github.com/hyperledger/fabric/examples/chaincode/go/chaincode_domain .
rm -rf chaincode_dns_reslover
cp -r /home/qichang/go/src/github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover .
sed -i '$d' .bashrc
echo "export zzm=\"$zzm\"" >> .bashrc
docker build -t peer-image .