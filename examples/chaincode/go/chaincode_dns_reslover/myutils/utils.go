package myutils

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
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

// BuildNewRecord 新建记录
func BuildNewRecord(recordType RecordType, value string, ttl int64, createAt int64) Record {
	if createAt == 0 {
		createAt = time.Now().Unix()
	}
	return Record{
		Type:      recordType,
		Value:     value,
		TTL:       ttl,
		CreatedAt: createAt,
		UpdateAt:  time.Now().Unix(),
	}
}
