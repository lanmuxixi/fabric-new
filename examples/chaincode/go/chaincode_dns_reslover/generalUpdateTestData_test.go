package main

import (
	"fmt"
	"github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover/myutils"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	TestFileLocation = "./test_data_tld_update.sh"
	TestDataNumber   = 10000
	//TestDataBatchSize 1000
	IsDiffUser          = true
	HaveNonce           = false
	ShouldSleepInterval = false
)

const (
	TestDefaultNonce          = "336710"
	TestDefaultTarget         = "0000ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	TestDataDefaultRecordType = "A"
	TestDataDefaultRecordTTL  = "86400"
)

const (
	TEST_INIT_ADMIN_NAME       = "ADMIN"
	TEST_INIT_ADMIN_CERTFICATE = "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"
	TEST_INIT_ADMIN_PRIVATEKEY = "-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDA6p2gap9n0U96P2rp+\nzudeJkw8VHGmMoakz6Um4pVi5auu7UyOYlqLu8XfoRQylm+gBwYFK4EEACKhZANi\nAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77oTl/\nLg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzEMSin\nf4U=\n-----END PRIVATE KEY-----"
	TEST_INIT_JIM_NAME         = "JIM"
	TEST_INIT_JIM_CERTFICATE   = "-----BEGIN CERTIFICATE-----\nMIICCzCCAZGgAwIBAgIQAOpb0QCV/y0qdDtDHZEE7zAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDczODU2WhcNMjUwNTA4MDcz\nODU2WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5k\nJNUDBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL\n10LhxzijgaEwgZ4wHQYDVR0OBBYEFGEEKfoi8WRktgpNQ+5ZW1yWej0SMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBRhBCn6IvFkZLYKTUPuWVtclno9EjAKBggqhkjOPQQDAgNoADBlAjAcdM3n\nsALhS5ksNd9h/XVXNFrNcrR22OKq81YLh3OU2GdWzAzqt8XU6UJM/UpudWECMQDt\nU/WJhQvaVAMr8XUrxjKdUoNThMh3J/zEAp3CZyS2vFfJa8cJDzV8j3s8a//8eVk=\n-----END CERTIFICATE-----"
	TEST_INIT_JIM_PRIVATEKEY   = "-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDDEzpnX/6bJHiAyX3YM\nsnjHAgflkru6J629fEXvXp9R3gvRoUyTVya275zul+u7irOgBwYFK4EEACKhZANi\nAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5kJNUD\nBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL10Lh\nxzg=\n-----END PRIVATE KEY-----"

	TEST_INIT_TOM_NAME       = "TOM"
	TEST_INIT_TOM_CERTFICATE = "-----BEGIN CERTIFICATE-----\nMIICCTCCAY+gAwIBAgIQAIG3dZMJb3sfGhNITXNvizAKBggqhkjOPQQDAjAWMRQw\nEgYDVQQDDAt0YW55b25nZmVuZzAeFw0yNDA1MDkwNTM0NTFaFw0zNDA1MDcwNTM0\nNTFaMBYxFDASBgNVBAMMC3RhbnlvbmdmZW5nMHYwEAYHKoZIzj0CAQYFK4EEACID\nYgAEWL4JsgdAv1SwcOzKif4mF5nL3Cecb5cPpg3j/kXI4K4hOQoLboIyTLdBeniM\ntrz90+Qxq7YEZ5D1io/SXQQ7EzWrHwN0vFSXXEO+yDW+xklWP96ijkpsZ15iYVP8\noogNo4GhMIGeMB0GA1UdDgQWBBS+Gtrt2Dda2tOZ7vkIF54PQ123nDAOBgNVHQ8B\nAf8EBAMCAYYwDwYDVR0TAQH/BAUwAwEB/zA7BgNVHSUENDAyBggrBgEFBQcDAgYI\nKwYBBQUHAwEGCCsGAQUFBwMDBggrBgEFBQcDBAYIKwYBBQUHAwgwHwYDVR0jBBgw\nFoAUvhra7dg3WtrTme75CBeeD0Ndt5wwCgYIKoZIzj0EAwIDaAAwZQIxANTPaFIT\n4mbG/k/fkKfVB3RZl7LGWBzv3sUbO+T8pXHAAvMcWGOXVql3+hn995f6swIwOfI9\nYhMyELb1Bu91N0emOC8kxhL74c1bh1/TFF2RDQ0xr9MPkmmsVFLt9jhGpWiH\n-----END CERTIFICATE-----"
	TEST_INIT_TOM_PRIVATEKEY = "-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDCcMe2re7ngLccZ4p0T\n4dEdsN4NhEqJ7EQ4g2D7Ir2yjxtgBqmzxY5Ijjy6zsC8Q3KgBwYFK4EEACKhZANi\nAARYvgmyB0C/VLBw7MqJ/iYXmcvcJ5xvlw+mDeP+RcjgriE5CgtugjJMt0F6eIy2\nvP3T5DGrtgRnkPWKj9JdBDsTNasfA3S8VJdcQ77INb7GSVY/3qKOSmxnXmJhU/yi\niA0=\n-----END PRIVATE KEY-----\n"

	TEST_INIT_JACK_NAME       = "JACK"
	TEST_INIT_JACK_CERTFICATE = "-----BEGIN CERTIFICATE-----\nMIIB/zCCAYWgAwIBAgIQAMdvxWY7W524PwanlREnFTAKBggqhkjOPQQDAjARMQ8w\nDQYDVQQDDAZmYWJyaWMwHhcNMjQwNTEwMDkyNzI4WhcNMjUwNTEwMDkyNzI4WjAR\nMQ8wDQYDVQQDDAZmYWJyaWMwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARxsQARd5ZO\n+pvT/hPDxMEI3ZS6VykjfkZ8eD3RhPbaRIriSZht8c3OIKh1q8XjUQk7WGHMutZL\nYY4TAhLlI/Fp1T5MfG81HB5WFyrxMsnVY4rSjp9bb8HNhEtx1fIfzx6jgaEwgZ4w\nHQYDVR0OBBYEFArvHXWmAURP24q9cxd8dB+oCuBTMA4GA1UdDwEB/wQEAwIBhjAP\nBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMCBggrBgEFBQcDAQYI\nKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSMEGDAWgBQK7x11pgFE\nT9uKvXMXfHQfqArgUzAKBggqhkjOPQQDAgNoADBlAjEAmKJnlJau5tf/UWsd7bay\nfly6K7KtTfHjcoBjfrExkE6ID4gLm3UeGOi0fTbNPZJGAjAHVcdqH3kKwXLEDN3a\n98hLkgMMJLX7y6laqob8T0ymgdufumJ6VotZVwtXqaM5szI=\n-----END CERTIFICATE-----\n"
	TEST_INIT_JACK_PRIVATEKEY = "-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDA2sc9pIdHri7uuEa0y\nKQ+nP1LpE1j8Vuq6yWbCtJay2xbqc1VDwtTgcQEdgbiDNg+gBwYFK4EEACKhZANi\nAARxsQARd5ZO+pvT/hPDxMEI3ZS6VykjfkZ8eD3RhPbaRIriSZht8c3OIKh1q8Xj\nUQk7WGHMutZLYY4TAhLlI/Fp1T5MfG81HB5WFyrxMsnVY4rSjp9bb8HNhEtx1fIf\nzx4=\n-----END PRIVATE KEY-----\n"

	TEST_INIT_TAN_NAME       = "TAN"
	TEST_INIT_TAN_CERTIFCATE = "-----BEGIN CERTIFICATE-----\nMIICADCCAYWgAwIBAgIQAK1AFOQ5usSqQwkFpnW1hTAKBggqhkjOPQQDAjARMQ8w\nDQYDVQQDDAZmYWJyaWMwHhcNMjQwNTEwMDkyODU0WhcNMjUwNTEwMDkyODU0WjAR\nMQ8wDQYDVQQDDAZmYWJyaWMwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAS/ceW8YLVz\n6qE7BTCdjkgBMr165CJmAJ7AncHkCx/vLq+AXDAro5Z6j1TOwxQPnRQjuqdzqs0o\nxuaMmcg7sccYU9YQiZIcCUWX2QmrQTvki2aW31Fb2O95UMrJIrv9FzijgaEwgZ4w\nHQYDVR0OBBYEFAqi3LStgQAE1bZcVFNRI1eeBx7xMA4GA1UdDwEB/wQEAwIBhjAP\nBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMCBggrBgEFBQcDAQYI\nKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSMEGDAWgBQKoty0rYEA\nBNW2XFRTUSNXngce8TAKBggqhkjOPQQDAgNpADBmAjEAxVR3sFC3LZlhU6KXvvuH\n7F1gyNoXe7kE9FVmgY1i8rjLhOsxpR9BsYuhNlMPbhr1AjEA/Cx9JLXC0LVe9BM/\n3famyizvkhIw2NvRN55dcUeIPbrhet1SEVGKGeG5QRhn0QT0\n-----END CERTIFICATE-----\n"
	TEST_INIT_TAN_PRIVATEKEY = "-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDACarIbKsEwcycvvUmt\n7+OaYOduzOLP0Ibsx3d6MIYkw4jlYAcan4l11vg9hgUbgaegBwYFK4EEACKhZANi\nAAS/ceW8YLVz6qE7BTCdjkgBMr165CJmAJ7AncHkCx/vLq+AXDAro5Z6j1TOwxQP\nnRQjuqdzqs0oxuaMmcg7sccYU9YQiZIcCUWX2QmrQTvki2aW31Fb2O95UMrJIrv9\nFzg=\n-----END PRIVATE KEY-----\n"
)

type TestData struct {
	domain     string
	ip         string
	recordType string
	ttl        string
	owner      string
	signature  string
}

type User struct {
	name        string
	certificate []byte
	privateKey  []byte
}

var users []User

func init() {
	//jim is hacker, don't use it
	jim := User{
		name:        TEST_INIT_JIM_NAME,
		certificate: []byte(TEST_INIT_JIM_CERTFICATE),
		privateKey:  []byte(TEST_INIT_JIM_PRIVATEKEY),
	}
	admin := User{
		name:        TEST_INIT_ADMIN_NAME,
		certificate: []byte(TEST_INIT_ADMIN_CERTFICATE),
		privateKey:  []byte(TEST_INIT_ADMIN_PRIVATEKEY),
	}
	tom := User{
		name:        TEST_INIT_TOM_NAME,
		certificate: []byte(TEST_INIT_TOM_CERTFICATE),
		privateKey:  []byte(TEST_INIT_TOM_PRIVATEKEY),
	}
	jack := User{
		name:        TEST_INIT_JACK_NAME,
		certificate: []byte(TEST_INIT_JACK_CERTFICATE),
		privateKey:  []byte(TEST_INIT_JACK_PRIVATEKEY),
	}
	tan := User{
		name:        TEST_INIT_TAN_NAME,
		certificate: []byte(TEST_INIT_TAN_CERTIFCATE),
		privateKey:  []byte(TEST_INIT_TAN_PRIVATEKEY),
	}

	users = append(users, jim, admin, tom, jack, tan)
}

func TestGenerateData(t *testing.T) {
	// 检查文件是否存在
	var file *os.File
	var err error
	_, err = os.Stat(TestFileLocation)
	dir, _ := os.Getwd()
	fmt.Println(dir)
	if os.IsNotExist(err) {
		_, err := os.Create(TestFileLocation)
		if err != nil {
			fmt.Printf("error occur, when %s file create", TestFileLocation)
		} else {
			fmt.Printf("%s test file not existed, but have created....\n", TestFileLocation)
		}
	}

	file, err = os.OpenFile(TestFileLocation, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Printf("start creating....\n")

	testData := generateData()
	WriteInsertUser2File(file, users)
	WriteUpdateData2File(file, testData)

	absolutePath, _ := filepath.Abs(TestFileLocation)
	fmt.Printf("Create success!, the test date is located in %s\n", absolutePath)
}

func WriteInsertUser2File(file *os.File, userss []User) {
	for _, user := range userss {
		data := fmt.Sprintf("peer chaincode invoke -n ${zzm} -c '{\"Function\": \"UserRegister\", \"Args\":[\"%s\", \"%s\"]}'", user.name, user.certificate)
		data = strings.Replace(data, "\n", "\\n", -1)
		if _, err := file.WriteString(data + "\n"); err != nil {
			fmt.Printf("register user error\n")
		}
	}
}

func WriteUpdateData2File(file *os.File, datas []*TestData) {
	for _, data := range datas {
		if HaveNonce {
			if ShouldSleepInterval {
				if _, err := fmt.Fprintf(file, "peer chaincode invoke -n ${zzm} -c '{\"Function\": \"TopLevelUpdate\", \"Args\":[\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]}'; sleep %s\n", data.domain, data.ip, data.recordType, data.ttl, data.owner, data.signature, TestDefaultTarget, TestDefaultNonce, getRandomMs()); err != nil {
					fmt.Printf("write error")
				}
			} else {
				if _, err := fmt.Fprintf(file, "peer chaincode invoke -n ${zzm} -c '{\"Function\": \"TopLevelUpdate\", \"Args\":[\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]}'\n", data.domain, data.ip, data.recordType, data.ttl, data.owner, data.signature, TestDefaultTarget, TestDefaultNonce); err != nil {
					fmt.Printf("write error")
				}
			}
		} else {
			if ShouldSleepInterval {
				if _, err := fmt.Fprintf(file, "peer chaincode invoke -n ${zzm} -c '{\"Function\": \"TopLevelUpdate\", \"Args\":[\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]}'; sleep %s\n", data.domain, data.ip, data.recordType, data.ttl, data.owner, data.signature, getRandomMs()); err != nil {
					fmt.Printf("write error")
				}
			} else {
				if _, err := fmt.Fprintf(file, "peer chaincode invoke -n ${zzm} -c '{\"Function\": \"TopLevelUpdate\", \"Args\":[\"%s\", \"%s\", \"%s\", \"%s\", \"%s\", \"%s\"]}'\n", data.domain, data.ip, data.recordType, data.ttl, data.owner, data.signature); err != nil {
					fmt.Printf("write error")
				}
			}
		}

	}
}

// generateDomain generate the init domain and ips
func generateData() []*TestData {
	var domains []string
	var ips []string
	var datas []*TestData
	for i := 'a'; i <= 'z'; i++ {
		for j := 'a'; j <= 'z'; j++ {
			for k := 'a'; k <= 'z'; k++ {
				for m := 'a'; m <= 'z'; m++ {
					if len(domains) > TestDataNumber {
						break
					}
					domains = append(domains, string(i)+string(j)+string(k)+string(m))
				}

			}
		}
	}
	for i := 1; i <= 255; i++ {
		for j := 1; j <= 255; j++ {
			for k := 1; k <= 255; k++ {
				for m := 1; m <= 255; m++ {
					if len(ips) > TestDataNumber {
						break
					}
					ips = append(ips, fmt.Sprintf("%d.%d.%d.%d", i, j, k, m))
				}

			}
		}
	}
	for i := 0; i < TestDataNumber; i++ {
		user := users[1]
		n := len(users) - 1
		if IsDiffUser {
			randIndex := rand.Intn(n)
			user = users[randIndex+1]
		}
		data := TestData{
			domain:     domains[i],
			ip:         ips[i],
			recordType: TestDataDefaultRecordType,
			ttl:        TestDataDefaultRecordTTL,
			owner:      user.name,
			signature:  myutils.Sign(user.privateKey, domains[i]),
		}
		datas = append(datas, &data)
	}
	return datas
}

func generateRandomFloat(min, max float64) float64 {
	rand.Seed(time.Now().UnixNano())
	return min + rand.Float64()*(max-min)
}
func getRandomMs() string {
	randomFloat := generateRandomFloat(0.01, 0.05)
	// 将随机数四舍五入到最近的0.001
	randomFloat = float64(int(randomFloat*1000+0.5)) / 1000
	return fmt.Sprintf("%.3f", randomFloat)

}
