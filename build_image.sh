# 链码ID
zzm="7ddc511a621936a639a4d8dfdbc2dcd07d0420f9e8dcd59c957ddecd11097265b7db9391191c03cad9b75867acc8112406ee9a3654e84400033f6cd8e435f22f"

cd ~/pack_image/peer-image
cp -r ~/go/src/github.com/hyperledger/fabric/consensus .
cp -r ~/go/src/github.com/hyperledger/fabric/core .
cp -r ~/go/src/github.com/hyperledger/fabric/peer .
cp -r ~/go/src/github.com/hyperledger/fabric/protos .
cp -r ~/go/src/github.com/hyperledger/fabric/txscripts .
cp -r ~/go/src/github.com/hyperledger/fabric/examples/chaincode/go/chaincode_domain .
rm -rf chaincode_dns_reslover
cp -r ~/go/src/github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover .
sed -i '$d' .bashrc
echo "export zzm=\"$zzm\"" >> .bashrc
docker build -t peer-image .