package myutils

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/url"
	"path"
	"regexp"
	"strings"
)

type ResponseCode int

const (
	Success ResponseCode = iota
	Failed
)

type RecordType string

const (
	A     RecordType = "A"
	MX    RecordType = "MX"
	CNAME RecordType = "CNAME"
	TXT   RecordType = "TXT"
)

type Record struct {
	Type      RecordType
	Value     string
	TTL       int64
	CreatedAt int64
	UpdateAt  int64
}
type Response struct {
	Code ResponseCode
	Msg  string
	Data map[string]interface{}
}

// GetTopLevelDomain returns the top level domain of a given domain
func GetTopLevelDomain(domain string) (string, error) {
	if !strings.Contains(domain, "://") {
		domain = "http://" + domain
	}
	u, err := url.Parse(domain)
	if err != nil {
		return "", err
	}
	hostParts := strings.Split(u.Host, ":")
	tld := path.Ext(hostParts[0])
	if tld == "" {
		tld = hostParts[0]
	}
	return strings.TrimPrefix(tld, "."), nil
}

// CheckValidDomain checks if a given domain is valid
func CheckValidDomain(domain string) bool {
	if domain == "" {
		return false
	}
	urlRegex := `^((http|https):\/\/)?[^\s/$.?#].[^\s]*$`
	match, _ := regexp.MatchString(urlRegex, domain)
	return match
}

// CheckValidIp checks if a given ip is valid
func CheckValidIp(ip string) bool {
	if ip == "" {
		return false
	}
	ipRegex := `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(:\d{1,5})?$`
	match, _ := regexp.MatchString(ipRegex, ip)
	return match
}

func BuildResponse(status bool, message string, data map[string]interface{}) []byte {
	var response Response
	var responseJson []byte
	var statusCode ResponseCode
	if status {
		statusCode = Success
	} else {
		statusCode = Failed
	}
	response = Response{
		Code: statusCode,
		Msg:  message,
		Data: data,
	}
	responseJson, _ = json.Marshal(response)
	return responseJson
}
func BuildWrongResponse(message string) []byte {
	return BuildResponse(false, message, nil)
}

type ECDSASignature struct {
	R, S *big.Int
}

// Sign 签名
func Sign(certKey []byte, text string) string {

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

// Verify VerifySignature 验证签名
func Verify(certStr []byte, text string, sign string) bool {
	block, _ := pem.Decode(certStr)
	if block == nil {
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

	arr := []byte(text)
	h := sha256.New()
	h.Write(arr)
	hashed := h.Sum(nil)

	signatureDec, _ := base64.StdEncoding.DecodeString(sign)
	sig := new(ECDSASignature)

	_, err = asn1.Unmarshal(signatureDec, sig)
	if err != nil {
		//fmt.Printf("ERROR: failed unmashalling signature, error: %v", err)
		return false
	}

	pub, _ := cert.PublicKey.(*ecdsa.PublicKey)
	if !ecdsa.Verify(pub, hashed[:], sig.R, sig.S) {
		//fmt.Printf("ERROR: Failed to verify Signature: %v\n", err)
		return false
	}
	//fmt.Printf("Successed to verify Signature and nonce\n")
	return true
}

// VerifyCertificate 验证证书
func VerifyCertificate(certStr []byte) bool {
	block, _ := pem.Decode(certStr)
	if block == nil {
		//fmt.Printf("ERROR: block of decoded private key is nil\n")
		return false
	}

	_, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		//fmt.Printf("ERROR: failed get ECDSA private key, error: %v\n", err)
		return false
	}
	return true
}
