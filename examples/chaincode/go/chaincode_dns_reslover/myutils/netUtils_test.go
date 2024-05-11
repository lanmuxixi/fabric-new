package myutils

import (
	"fmt"
	"net"
	"testing"
)

func TestLookup(t *testing.T) {
	dns := "tans.fun"
	ip, err := net.LookupIP(dns)

	if err != nil {
		fmt.Print("reslove error")
	}
	fmt.Println(ip)
}

func TestParseDomainName(t *testing.T) {
	lookup, err := SendQuest("example.com", "127.0.0.1", "30054")
	if err != nil {
		return
	}
	fmt.Println(lookup)
}

func TestSendUpdate(t *testing.T) {
	type args struct {
		domain    string
		ip        string
		dnsServer string
		dnsPort   string
		zone      string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test insert",
			args: args{
				zone:      "com",
				domain:    "abc.google.com",
				ip:        "1.1.1.3",
				dnsServer: "127.0.0.1",
				dnsPort:   "30054",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SendUpdate(tt.args.zone, tt.args.domain, tt.args.ip, tt.args.dnsServer, tt.args.dnsPort)
		})
	}
}

func TestSendQuest(t *testing.T) {
	type args struct {
		domain    string
		dnsServer string
		dnsPort   string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "test query",
			args: args{
				domain:    "abc.google.com",
				dnsServer: "127.0.0.1",
				dnsPort:   "30054",
			},
			want:    []string{"1.1.1.2", "1.1.1.3"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SendQuest(tt.args.domain, tt.args.dnsServer, tt.args.dnsPort)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendQuest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) == 0 {
				t.Errorf("Result is empty")
			}
			fmt.Printf("%s result is %v", tt.args.domain, got)
		})
	}
}
