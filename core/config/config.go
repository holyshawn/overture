package config

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/holyshawn/overture/core/cache"
	"github.com/janeczku/go-dnsmasq/hostsfile"
)

var Config *configType

type EDNSClientSubnetType struct {
	Policy   string
	CustomIP string
}

type DNSUpstream struct {
	Name             string
	Address          string
	Protocol         string
	Timeout          int
	EDNSClientSubnet EDNSClientSubnetType
}

type configType struct {
	BindAddress        string `json:"BindAddress"`
	PrimaryDNS         []*DNSUpstream
	AlternativeDNS     []*DNSUpstream
	RedirectIPv6Record bool
	IPNetworkFile      string
	DomainFile         string
	DomainBase64Decode bool
	HostsFile          string
	MinimumTTL         int
	CacheSize          int

	DomainList            []string
	IPNetworkList         []*net.IPNet
	Hosts                 *hosts.Hostsfile
	ExternalIP            string
	ReservedIPNetworkList []*net.IPNet
	CachePool             *cache.Cache
}

func parseJson(path string) *configType {

	f, err := os.Open(path)
	if err != nil {
		log.Fatal("Open config file failed: ", err)
		os.Exit(1)
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal("Read config file failed: ", err)
		os.Exit(1)
	}

	j := new(configType)
	err = json.Unmarshal(b, j)
	if err != nil {
		log.Fatal("Json syntex error: ", err)
		os.Exit(1)
	}

	log.Debug(string(b))

	return j
}

func NewConfig(path string) *configType {

	return parseJson(path)

}
