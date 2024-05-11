package myutils

import (
	"fmt"
	"github.com/TanYongF/dns"
)

const (
	TTL           = 86400
	RECORD_TYPE_A = "A"
	RECORD_CLASS  = "IN"
)

// SendQuest 构造 DNS 查询命令，查询记录
func SendQuest(domain string, dnsServer string, dnsPort string) ([]dns.A, error) {
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, fmt.Sprintf("%s:%s", dnsServer, dnsPort))
	if err != nil {
		return nil, err
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("dns query failed")
	}

	var records []dns.A
	for _, a := range r.Answer {
		if t, ok := a.(*dns.A); ok {
			records = append(records, *t)

		}
	}
	return records, nil
}

// SendUpdate 构造 DNS 更新命令，更新记录
func SendUpdate(zone string, domain string, ip string, dnsServer string, dnsPort string) (bool, error) {
	var m dns.Msg

	m.SetUpdate(dns.Fqdn(zone))
	var newRRs []dns.RR
	record := fmt.Sprintf("%s %d %s %s %s", dns.Fqdn(domain), TTL, RECORD_CLASS, RECORD_TYPE_A, ip)
	rr, _ := dns.NewRR(record)
	newRRs = append(newRRs, rr)
	m.Insert(newRRs)
	var client dns.Client
	res1, _, err := client.Exchange(&m, fmt.Sprintf("%s:%s", dnsServer, dnsPort))
	if err != nil {
		return false, err
	}
	if res1.Rcode != dns.RcodeSuccess {
		return false, fmt.Errorf("dns update failed")
	}
	return true, nil
}

// SendRemoveRRset 构造 DNS 删除命令，删除记录（只删除完全匹配的记录）
func SendRemoveRRset(zone string, domain string, ip string, dnsServer string, dnsPort string) (bool, error) {
	var m dns.Msg

	m.SetUpdate(dns.Fqdn(zone))
	var rrs []dns.RR
	record := fmt.Sprintf("%s %d %s %s %s", dns.Fqdn(domain), TTL, RECORD_CLASS, RECORD_TYPE_A, ip)
	rr, _ := dns.NewRR(record)
	rrs = append(rrs, rr)
	m.RemoveRRset(rrs)
	var client dns.Client
	res1, _, err := client.Exchange(&m, fmt.Sprintf("%s:%s", dnsServer, dnsPort))
	if err != nil {
		return false, err
	}
	if res1.Rcode != dns.RcodeSuccess {
		return false, fmt.Errorf("dns update failed")
	}
	return true, nil
}

// SendRemoveName 构造 DNS 删除命令，删除记录（删除所有给定域名的记录）
func SendRemoveName(zone string, domain string, dnsServer string, dnsPort string) (bool, error) {
	var m dns.Msg

	m.SetUpdate(dns.Fqdn(zone))
	var rrs []dns.RR
	record := fmt.Sprintf("%s %d %s %s %s", dns.Fqdn(domain), TTL, RECORD_CLASS, RECORD_TYPE_A, "1.1.1.1")
	rr, _ := dns.NewRR(record)
	rrs = append(rrs, rr)
	m.RemoveName(rrs)
	var client dns.Client
	res1, _, err := client.Exchange(&m, fmt.Sprintf("%s:%s", dnsServer, dnsPort))
	if err != nil {
		return false, err
	}
	if res1.Rcode != dns.RcodeSuccess {
		return false, fmt.Errorf("dns update failed")
	}
	return true, nil
}
