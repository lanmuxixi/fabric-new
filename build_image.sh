# 链码ID
zzm="ee5b24a1f17c356dd5f6e37307922e39ddba12e5d2e203ed93401d7d05eb0dd194fb9070549c5dc31eb63f4e654dbd5a1d86cbb30c48e3ab1812590cd0f78539"

cd ~/pack_image/peer-image
cp -r ~/go/src/github.com/hyperledger/fabric/consensus .
cp -r ~/go/src/github.com/hyperledger/fabric/core .
cp -r ~/go/src/github.com/hyperledger/fabric/peer .
sed -i '$d' .bashrc
echo "export zzm=\"$zzm\"" >> .bashrc
docker build -t peer-image .