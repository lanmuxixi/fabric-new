package myutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestCheckValidDomain(t *testing.T) {
	type args struct {
		domain string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"TestCheckValidDomain", args{"http://www.google.com"}, true},
		{"TestCheckValidDomain", args{"https://www.google.com"}, true},
		{"TestCheckValidDomain", args{"http://www.google.com:8080"}, true},
		{"TestCheckValidDomain", args{"http://www.google.com:8080/abc"}, true},
		{"TestCheckValidDomain", args{"http://www.google.com:8080/abc?def=123"}, true},
		{"TestCheckValidDomain", args{"https://www.google.com:8080/abc?def=123#456"}, true},
		{"TestCheckValidDomain", args{"http://www.google.com:8080/abc?def=123#456/xyz/abc"}, true},
		{"TestCheckValidDomain", args{"www.google.com"}, false},
		{"TestCheckValidDomain", args{"com"}, true},
		{"TestCheckValidDomain", args{""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckValidDomain(tt.args.domain); got != tt.want {
				t.Errorf("CheckValidDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckValidIp(t *testing.T) {
	type args struct {
		ip string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"TestCheckValidIp", args{"222.222.222.222"}, true},
		{"TestCheckValidIp", args{"1.1.1.1.1"}, false},
		{"TestCheckValidIp", args{"1.1.1"}, false},
		{"TestCheckValidIp", args{"1.1.1."}, false},
		{"TestCheckValidIp", args{"1.1.1.256"}, false},
		{"TestCheckValidIp", args{"255.255.255.255"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckValidIp(tt.args.ip); got != tt.want {
				t.Errorf("CheckValidIp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTopLevelDomain(t *testing.T) {
	type args struct {
		domain string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"TestGetTopLevelDomain", args{"http://www.google.com"}, "com", false},
		{"TestGetTopLevelDomain", args{"https://www.google.com"}, "com", false},
		{"TestGetTopLevelDomain", args{"http://www.google.com:8080"}, "com", false},
		{"TestGetTopLevelDomain", args{"http://www.google.com:8080/abc"}, "com", false},
		{"TestGetTopLevelDomain", args{"http://www.google.com:8080/abc?def=123"}, "com", false},
		{"TestGetTopLevelDomain", args{"https://www.google.com:8080/abc?def=123#456"}, "com", false},
		{"TestGetTopLevelDomain", args{"com"}, "com", false},
		{"TestGetTopLevelDomain", args{"www.baidu.com"}, "com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTopLevelDomain(tt.args.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTopLevelDomain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetTopLevelDomain() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildResponse(t *testing.T) {
	type args struct {
		status  bool
		message string
		data    map[string]interface{}
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "TestBuildResponse success",
			args: args{
				status:  true,
				message: "Success",
				data:    map[string]interface{}{"key": "value"},
			},
			want: func() []byte {
				response := Response{
					Code: Success,
					Msg:  "Success",
					Data: map[string]interface{}{"key": "value"},
				}
				responseJson, _ := json.Marshal(response)
				return responseJson
			}(),
		},
		{
			name: "TestBuildResponse failed",
			args: args{
				status:  false,
				message: "Failed",
				data:    map[string]interface{}{"key": "value"},
			},
			want: func() []byte {
				response := Response{
					Code: Failed,
					Msg:  "Failed",
					Data: map[string]interface{}{"key": "value"},
				}
				responseJson, _ := json.Marshal(response)
				return responseJson
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildResponse(tt.args.status, tt.args.message, tt.args.data); !bytes.Equal(got, tt.want) {
				t.Errorf("BuildResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSign(t *testing.T) {
	var privateKey, cert, privateKey2, cert2 []byte
	privateKey = []byte("-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDA6p2gap9n0U96P2rp+\nzudeJkw8VHGmMoakz6Um4pVi5auu7UyOYlqLu8XfoRQylm+gBwYFK4EEACKhZANi\nAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77oTl/\nLg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzEMSin\nf4U=\n-----END PRIVATE KEY-----")
	privateKey2 = []byte("-----BEGIN PRIVATE KEY-----\nMIG/AgEAMBAGByqGSM49AgEGBSuBBAAiBIGnMIGkAgEBBDDEzpnX/6bJHiAyX3YM\nsnjHAgflkru6J629fEXvXp9R3gvRoUyTVya275zul+u7irOgBwYFK4EEACKhZANi\nAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5kJNUD\nBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL10Lh\nxzg=\n-----END PRIVATE KEY-----")
	cert = []byte("-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----")
	cert2 = []byte("-----BEGIN CERTIFICATE-----\nMIICCzCCAZGgAwIBAgIQAOpb0QCV/y0qdDtDHZEE7zAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDczODU2WhcNMjUwNTA4MDcz\nODU2WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAATCRfmQst/g22wAuSpRI9SOeeIiSHm6yFS/++d1FKdPC9I1VF5U2qjzvm5k\nJNUDBr7QSHqIcrtnuiZB+4xfVR5wIkir7mGx8kDq6yqUatZJhyI1mBvszrPGMWdL\n10LhxzijgaEwgZ4wHQYDVR0OBBYEFGEEKfoi8WRktgpNQ+5ZW1yWej0SMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBRhBCn6IvFkZLYKTUPuWVtclno9EjAKBggqhkjOPQQDAgNoADBlAjAcdM3n\nsALhS5ksNd9h/XVXNFrNcrR22OKq81YLh3OU2GdWzAzqt8XU6UJM/UpudWECMQDt\nU/WJhQvaVAMr8XUrxjKdUoNThMh3J/zEAp3CZyS2vFfJa8cJDzV8j3s8a//8eVk=\n-----END CERTIFICATE-----")
	res := Sign(privateKey, "xxx.google.com")
	res2 := Sign(privateKey2, "xxx.google.com")
	fmt.Println(Sign(privateKey2, "xxx.google.com"))
	verify := Verify(cert, "xxx.google.com", res)
	ok := Verify(cert2, "xxx.google.com", res2)
	fmt.Println("verify2: ", ok)
	fmt.Printf("encrypt: %s\n", res)
	fmt.Printf("Verify: %v\n", verify)
}

func TestVerifyCertificate(t *testing.T) {
	type args struct {
		certStr []byte
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "TestVerifyCertificate",
			args: args{
				[]byte("-----BEGIN CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"),
			},
			want: true,
		},
		{
			name: "TestVerifyCertificate2",
			args: args{
				[]byte("-----BEGI CERTIFICATE-----\nMIICCjCCAZGgAwIBAgIQAIvYoCKznwvKPCDx8ZyvtTAKBggqhkjOPQQDAjAXMRUw\nEwYDVQQDDAx3d3cudGFucy5mdW4wHhcNMjQwNTA4MDYxNzU1WhcNMjUwNTA4MDYx\nNzU1WjAXMRUwEwYDVQQDDAx3d3cudGFucy5mdW4wdjAQBgcqhkjOPQIBBgUrgQQA\nIgNiAASJ8KkFtmJVeUu30qei2lV/6ouCvDmu3+2IQDvxQz+b4+uEfd1jvBE2mH77\noTl/Lg9WGMrAOD64EdXQOlqK7UL53XESimQur9UFHJEW4IMq48ZYIdIKX9I3dMzE\nMSinf4WjgaEwgZ4wHQYDVR0OBBYEFJLmcANsF95a7AsSY5C6LyT7/fLZMA4GA1Ud\nDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MDsGA1UdJQQ0MDIGCCsGAQUFBwMC\nBggrBgEFBQcDAQYIKwYBBQUHAwMGCCsGAQUFBwMEBggrBgEFBQcDCDAfBgNVHSME\nGDAWgBSS5nADbBfeWuwLEmOQui8k+/3y2TAKBggqhkjOPQQDAgNnADBkAjAHF4X5\nrhX8g1ZQAmz9pheV5wFFpjZOfM2jS9SVDbjNEw9vOOki8DAM/ripZMuOiT8CME6G\nbykxYmJJb3Rf3O2YKEqYmgOPkL3f0stA36cWNzp4C2PJqaQU2/ic16ZM1c63mg==\n-----END CERTIFICATE-----"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifyCertificate(tt.args.certStr); got != tt.want {
				t.Errorf("VerifyCertificate() = %v, want %v", got, tt.want)
			}
		})
	}
}
