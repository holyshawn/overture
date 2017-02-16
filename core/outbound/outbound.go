// Copyright (c) 2016 holyshawn. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package outbound

import (
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/holyshawn/overture/core/cache"
	"github.com/holyshawn/overture/core/config"
	"github.com/miekg/dns"
)

type outbound struct {
	ResponseMessage    *dns.Msg
	QuestionMessage    *dns.Msg

	DNSUpstream        *config.DNSUpstream
	EDNSClientSubnetIP string

	minimumTTL         int
	inboundIP          string
}

func newOutbound(q *dns.Msg, u *config.DNSUpstream, inboundIP string) *outbound {

	o := &outbound{
		QuestionMessage: q,
		DNSUpstream:     u,
		minimumTTL:      config.Config.MinimumTTL,
		inboundIP:       inboundIP,
	}

	o.EDNSClientSubnetIP = o.getEDNSClientSubnetIP()

	return o
}

func (o *outbound) exchangeFromRemote(IsCache bool, isLog bool) {

	if o.exchangeFromCache(isLog) {
		return
	}

	setEDNSClientSubnet(o.QuestionMessage, o.EDNSClientSubnetIP)

	c := new(dns.Client)
	c.Net = o.DNSUpstream.Protocol
	c.Timeout = time.Duration(o.DNSUpstream.Timeout) * time.Second

	temp, _, err := c.Exchange(o.QuestionMessage, o.DNSUpstream.Address)
	if err != nil {
		if err == dns.ErrTruncated {
			log.Warn("Maybe your primary dns server does not support edns client subnet")
			return
		}
	}
	if temp == nil {
		log.Debug(o.DNSUpstream.Name + " Fail: Response message is nil, maybe timeout, please check your query or dns configuration")
		return
	}

	o.ResponseMessage = temp

	if o.minimumTTL > 0 {
		setMinimumTTL(o.ResponseMessage, uint32(o.minimumTTL))
	}

	if IsCache {
		config.Config.CachePool.InsertMessage(cache.Key(o.QuestionMessage.Question[0], o.EDNSClientSubnetIP), o.ResponseMessage)
	}

	if isLog {
		o.logAnswer(false)
	}
}

func (o *outbound) exchangeFromLocal() bool {

	raw_name := o.QuestionMessage.Question[0].Name

	if o.exchangeFromHosts(raw_name) || o.exchangeFromIP(raw_name) {
		return true
	}

	return false
}

func (o *outbound) exchangeFromCache(isLog bool) bool {

	if config.Config.CacheSize == 0 {
		return false
	}

	m := config.Config.CachePool.Hit(cache.Key(o.QuestionMessage.Question[0], o.EDNSClientSubnetIP), o.QuestionMessage.Id)
	if m != nil {
		log.Debug(o.DNSUpstream.Name + " Hit: " + cache.Key(o.QuestionMessage.Question[0], o.EDNSClientSubnetIP))
		o.ResponseMessage = m
		if isLog {
			o.logAnswer(false)
		}
		return true
	}

	return false
}

func (o *outbound) exchangeFromHosts(raw_name string) bool {

	if config.Config.Hosts == nil {
		return false
	}

	name := raw_name[:len(raw_name)-1]
	ipl, err := config.Config.Hosts.FindHosts(name)

	if err == nil && len(ipl) > 0 {
		for _, ip := range ipl {
			if o.QuestionMessage.Question[0].Qtype == dns.TypeA {
				a, _ := dns.NewRR(raw_name + " IN A " + ip.String())
				o.createResponseMessageForLocal(a)
				return true
			}
			if o.QuestionMessage.Question[0].Qtype == dns.TypeAAAA {
				aaaa, _ := dns.NewRR(raw_name + " IN AAAA " + ip.String())
				o.createResponseMessageForLocal(aaaa)
				return true
			}
		}
	}

	return false
}

func (o *outbound) exchangeFromIP(raw_name string) bool {

	name := raw_name[:len(raw_name)-1]
	ip := net.ParseIP(name)
	if ip.To4() != nil && o.QuestionMessage.Question[0].Qtype == dns.TypeA {
		a, _ := dns.NewRR(raw_name + " IN A " + ip.String())
		o.createResponseMessageForLocal(a)
		return true
	}
	if ip.To16() != nil && o.QuestionMessage.Question[0].Qtype == dns.TypeAAAA {
		aaaa, _ := dns.NewRR(raw_name + " IN AAAA " + ip.String())
		o.createResponseMessageForLocal(aaaa)
		return true
	}

	return false
}

func (o *outbound) logAnswer(isLocal bool) {

	for _, a := range o.ResponseMessage.Answer {
		var name string
		if isLocal {
			name = "Local"
		} else {
			name = o.DNSUpstream.Name
		}
		log.Debug(name + " Answer: " + a.String())
	}
}

func (o *outbound) createResponseMessageForLocal(r dns.RR) {
	o.ResponseMessage = new(dns.Msg)
	o.ResponseMessage.Answer = append(o.ResponseMessage.Answer, r)
	o.ResponseMessage.SetReply(o.QuestionMessage)
	o.ResponseMessage.RecursionAvailable = true
}

func setMinimumTTL(m *dns.Msg, ttl uint32) {

	for _, a := range m.Answer {
		if a.Header().Ttl < ttl {
			a.Header().Ttl = ttl
		}
	}
}
