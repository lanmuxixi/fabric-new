cd ~/pack_image/peer-image
rm -rf ./peer
cp -r ~/go/src/github.com/hyperledger/fabric/peer .
docker build -t peer-image .