package main

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

func checkInit(t *testing.T, stub *shim.MockStub, args []string) {
	_, err := stub.MockInit("1", "init", args)
	if err != nil {
		fmt.Println("Init failed", err)
		t.FailNow()
	}
}

func checkState(t *testing.T, stub *shim.MockStub, name string, value string) {
	bytes := stub.State[name]
	if bytes == nil {
		fmt.Println("State", name, "failed to get value")
		t.FailNow()
	}
	if string(bytes) != value {
		fmt.Println("State value", name, "was not", value, "as expected")
		t.FailNow()
	}
}

func checkQuery(t *testing.T, stub *shim.MockStub, function string, name string, value string) {
	bytes, err := stub.MockQuery(function, []string{name})
	if err != nil {
		fmt.Println("Query", name, "failed", err)
		t.FailNow()
	}
	if bytes == nil {
		fmt.Println("Query", name, "failed to get value")
		if value != "" {
			t.FailNow()
		}
	}
	if string(bytes) != value {
		fmt.Println("Query value", name, "was not", value, "as expected", "the actual value is", string(bytes))
		t.FailNow()
	}
}

func checkInvoke(t *testing.T, stub *shim.MockStub, function string, args []string) {
	_, err := stub.MockInvoke("1", function, args)
	if err != nil {
		fmt.Println("Invoke", args, "failed", err)
		t.FailNow()
	}
}

func TestExample02_Init(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)

	//checkInit(t, stub, []string{"A", "", "B", ""})
	checkInit(t, stub, []string{"com:1.1.1.1", "cn:1.1.1.2"})

	checkState(t, stub, "com", "1.1.1.1:53")
	checkState(t, stub, "cn", "1.1.1.2:53")
}

func TestExample02_Query(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)

	// Init A have no domain, B have no domain
	checkInit(t, stub, []string{"com:1.1.1.1:53", "cn:1.1.1.2:53"})
	// Query A
	checkQuery(t, stub, "TopLevelQuest", "google.com", "1.1.1.1:53")
	checkQuery(t, stub, "TopLevelQuest", "google.cn", "1.1.1.2:53")
}

// TestExample03_UpdateSimpleDomain 测试插入删除
func TestExample04_UpdateSimpleDomain(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("simpleDomainQuest Test", scc)
	// 增加用户
	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
	// Init A have no domain, B have no domain
	//checkInit(t, stub, []string{"com:127.0.0.1:30054", "cn:127.0.0.1:30054"})
	// Query A
	checkInvoke(t, stub, "TopLevelUpdate", []string{"xxx.google.com", "2.2.2.2", "A", "86400", "admin", "MGUCMGSC7XpQpEfC4kh0KS7h9kvpTb97kwV38NEChHZo5NviGtt2CK2nSR0+EqKEY82X6gIxAKsJEHh3DlvARyhhG+N6lKNErJ5Fv8a6y0HX2fRbOTARjJAnuZfS32pnuAqu9yvP8g=="})
	//checkQuery(t, stub, "resolve", "xxx.google.com", "2.2.2.2,2.2.2.3")
}

func TestExample05_DeleteSimpleDomain(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("simpleDomainDelete Test", scc)

	// Init A have no domain, B have no domain
	checkInit(t, stub, []string{"com:127.0.0.1:30054", "cn:127.0.0.1:30054"})
	// Query A
	checkInvoke(t, stub, "update", []string{"xxx.google.com", "2.2.2.2"})
	checkInvoke(t, stub, "update", []string{"xxx.google.com", "2.2.2.3"})
	checkInvoke(t, stub, "delete", []string{"xxx.google.com"})
	checkInvoke(t, stub, "update", []string{"xxx.google.com", "2.2.2.4"})
	checkQuery(t, stub, "resolve", "xxx.google.com", "2.2.2.4")
}

func TestExample02_Invoke(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)
	// A and B have no domain
	checkInit(t, stub, []string{"com:1.1.1.1", "cn:1.1.1.2"})

	// A get domain google.com
	checkInvoke(t, stub, "add", []string{"google.kr", "1.1.1.3"})
	checkInvoke(t, stub, "add", []string{"google.jp", "1.1.1.4"})
	checkQuery(t, stub, "resolveDomain", "google.kr", "1.1.1.3")
	checkQuery(t, stub, "resolveDomain", "google.jp", "1.1.1.4")
}

/**
*
*  顶级域名管理测试
*
 */
// TestExample03_UpdateSimpleDomain 测试正常
func TestExample05_UpdateTLDomain(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("simpleDomainQuest Test", scc)
	// 增加用户
	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
	// Query A
	checkInvoke(t, stub, "TopLevelUpdate", []string{"xxx.google.com", "2.2.2.2", "A", "86400", "admin", "MGUCMGSC7XpQpEfC4kh0KS7h9kvpTb97kwV38NEChHZo5NviGtt2CK2nSR0+EqKEY82X6gIxAKsJEHh3DlvARyhhG+N6lKNErJ5Fv8a6y0HX2fRbOTARjJAnuZfS32pnuAqu9yvP8g=="})
}

// TestExample04_DeleteSimpleDomain 测试用户证书不合法
func TestExample05_UpdateTLDomain_Invalid_Cert(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("simpleDomainQuest Test", scc)
	// Wrong Cert
	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
	checkInvoke(t, stub, "TopLevelUpdate", []string{"xxx.google.com", "2.2.2.2", "A", "86400", "admin", "MGUCMGSC7XpQpEfC4kh0KS7h9kvpTb97kwV38NEChHZo5NviGtt2CK2nSR0+EqKEY82X6gIxAKsJEHh3DlvARyhhG+N6lKNErJ5Fv8a6y0HX2fRbOTARjJAnuZfS32pnuAqu9yvP8g=="})
}

// TestExample04_DeleteSimpleDomain 测试签名不正确
func TestExample05_UpdateTLDomain_Invalid_Sign(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("simpleDomainQuest Test", scc)
	// 增加用户
	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
	// Wrong Signature
	checkInvoke(t, stub, "TopLevelUpdate", []string{"xxx.google.com", "2.2.2.2", "A", "86400", "admin", "MGUCMC7XpQpEfC4kh0KS7h9kvpTb97kwV38NEChHZo5NviGtt2CK2nSR0+EqKEY82X6gIxAKsJEHh3DlvARyhhG+N6lKNErJ5Fv8a6y0HX2fRbOTARjJAnuZfS32pnuAqu9yvP8g=="})
}

// TestExample04_DeleteSimpleDomain 测试用户不存在
func TestExample05_UpdateTLDomain_No_USER(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("simpleDomainQuest Test", scc)
	// 增加用户
	checkInvoke(t, stub, "UserRegister", []string{"admin1", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
	// Wrong Signature
	checkInvoke(t, stub, "TopLevelUpdate", []string{"xxx.google.com", "2.2.2.2", "A", "86400", "admin", "MGUCMC7XpQpEfC4kh0KS7h9kvpTb97kwV38NEChHZo5NviGtt2CK2nSR0+EqKEY82X6gIxAKsJEHh3DlvARyhhG+N6lKNErJ5Fv8a6y0HX2fRbOTARjJAnuZfS32pnuAqu9yvP8g=="})
}

// TestExample04_DeleteSimpleDomain 测试用户不一致
func TestExample05_UpdateTLDomain_Not_Same_User(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("simpleDomainQuest Test", scc)
	// 增加用户
	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
	checkInvoke(t, stub, "UserRegister", []string{"jim", "-----BEGIN CERTIFICATE-----\nMIICCzCCAZGgAwIBAgIQAOpb0QCV/y0qdDtDHZEE7zAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDczODU2WhcNMjUwNTA4MDcz\nODU2WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5k\nJNUDBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL\n10LhxzijgaEwgZ4wHQYDVR0OBBYEFGEEKfoi8WRktgpNQ+5ZW1yWej0SMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBRhBCn6IvFkZLYKTUPuWVtclno9EjAKBggqhkjOPQQDAgNoADBlAjAcdM3n\nsALhS5ksNd9h/XVXNFrNcrR22OKq81YLh3OU2GdWzAzqt8XU6UJM/UpudWECMQDt\nU/WJhQvaVAMr8XUrxjKdUoNThMh3J/zEAp3CZyS2vFfJa8cJDzV8j3s8a//8eVk=\n-----END CERTIFICATE-----\n"})

	checkInvoke(t, stub, "TopLevelUpdate", []string{"xxx.google.com", "2.2.2.2", "A", "86400", "admin", "MGUCMGSC7XpQpEfC4kh0KS7h9kvpTb97kwV38NEChHZo5NviGtt2CK2nSR0+EqKEY82X6gIxAKsJEHh3DlvARyhhG+N6lKNErJ5Fv8a6y0HX2fRbOTARjJAnuZfS32pnuAqu9yvP8g=="})
	checkInvoke(t, stub, "TopLevelUpdate", []string{"xxx.google.com", "2.2.2.2", "A", "86400", "jim", "MGUCMQCx+47gF+6RoUBFpL5b7Q7VRFxcQV1Svgv/s18XUEmN/DRPdA7EldzPo+44rjsjPoECMBlHvDn4CsVk4aeU5yi2YLHLd3MptQEcEYa0XERGzsCU8r11L+ALcxZD9pJxS9R6iw=="})
}

/**
*
*  用户管理注册
*
 */

// User Test
// TestExample02_Register 测试用户注册 正常
func TestExample02_Register(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)

	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})

}

// TestExample02_Register_Repeat 测试用户注册 重复注册
func TestExample02_Register_Repeat(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)

	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
}

// TestExample02_Register_Repeat 测试用户注册 证书不合法
func TestExample02_Register_Wrong_CERT(t *testing.T) {
	scc := new(SimpleChaincode)
	stub := shim.NewMockStub("ex10", scc)

	checkInvoke(t, stub, "UserRegister", []string{"admin", "-----BEGIN CERTFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"})
}
