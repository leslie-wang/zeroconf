package zeroconf

import (
	"fmt"
	"net"
	"sync"
)

// ServiceRecord contains the basic description of a service, which contains instance name, service type & domain
type ServiceRecord struct {
	Instance string `json:"name"`   // Instance name (e.g. "My web page")
	Service  string `json:"type"`   // Service name (e.g. _http._tcp.)
	Domain   string `json:"domain"` // If blank, assumes "local"

	// private variable populated on ServiceRecord creation
	serviceName         string
	serviceInstanceName string
	serviceTypeName     string
}

// ServiceName returns a complete service name (e.g. _foobar._tcp.local.), which is composed
// of a service name (also referred as service type) and a domain.
func (s *ServiceRecord) ServiceName() string {
	return s.serviceName
}

// ServiceInstanceName returns a complete service instance name (e.g. MyDemo\ Service._foobar._tcp.local.),
// which is composed from service instance name, service name and a domain.
func (s *ServiceRecord) ServiceInstanceName() string {
	return s.serviceInstanceName
}

// ServiceTypeName returns the complete identifier for a DNS-SD query.
func (s *ServiceRecord) ServiceTypeName() string {
	return s.serviceTypeName
}

// NewServiceRecord constructs a ServiceRecord.
func NewServiceRecord(instance, service, domain string) *ServiceRecord {
	s := &ServiceRecord{
		Instance:    instance,
		Service:     service,
		Domain:      domain,
		serviceName: fmt.Sprintf("%s.%s.", trimDot(service), trimDot(domain)),
	}

	// Cache service instance name
	if instance != "" {
		s.serviceInstanceName = fmt.Sprintf("%s.%s", trimDot(s.Instance), s.ServiceName())
	}

	// Cache service type name domain
	typeNameDomain := "local"
	if len(s.Domain) > 0 {
		typeNameDomain = trimDot(s.Domain)
	}
	s.serviceTypeName = fmt.Sprintf("_services._dns-sd._udp.%s.", typeNameDomain)

	return s
}

// LookupParams contains configurable properties to create a service discovery request
type LookupParams struct {
	ServiceRecord
	Entries chan<- *ServiceEntry // Entries Channel

	stopProbing chan struct{}
	once        sync.Once
}

// NewLookupParams constructs a LookupParams.
func NewLookupParams(instance, service, domain string, entries chan<- *ServiceEntry) *LookupParams {
	return &LookupParams{
		ServiceRecord: *NewServiceRecord(instance, service, domain),
		Entries:       entries,

		stopProbing: make(chan struct{}),
	}
}

// Notify subscriber that no more entries will arrive. Mostly caused
// by an expired context.
func (l *LookupParams) done() {
	close(l.Entries)
}

func (l *LookupParams) disableProbing() {
	l.once.Do(func() { close(l.stopProbing) })
}

// CallbackHook defines the different hook points for callback event
type CallbackHook int

const (
	// NetworkV4 hook is triggered after receiving network packet
	NetworkV4 CallbackHook = iota
	// NetworkV6 hook is triggered after receiving network packet
	NetworkV6
	// Unpack hook is triggered after unpack packet
	Unpack
	// ResponseUnicast is triggered after sending unicast reply
	ResponseUnicast
	// ResponseMulticast is triggered after sending multicast reply
	ResponseMulticast
)

// Callback for event callback purpose
type Callback func(CallbackHook, error, []interface{})

// ServiceEntry represents a browse/lookup result for client API.
// It is also used to configure service registration (server API), which is
// used to answer multicast queries.
type ServiceEntry struct {
	ServiceRecord
	HostName   string   `json:"hostname"` // Host machine DNS name
	Port       int      `json:"port"`     // Service Port
	Text       []string `json:"text"`     // Service info served as a TXT record
	TTL        uint32   `json:"ttl"`      // TTL of the service record
	AddrIPv4   []net.IP `json:"-"`        // Host machine IPv4 address
	AddrIPv6   []net.IP `json:"-"`        // Host machine IPv6 address
	CacheFlush bool     `json:"-"`        // Cache flush of the service record
	Callback   Callback `json:"-"`        // Callback function to allow caller to intercept internal event
}

// NewServiceEntry constructs a ServiceEntry.
func NewServiceEntry(instance, service, domain string) *ServiceEntry {
	return &ServiceEntry{
		ServiceRecord: *NewServiceRecord(instance, service, domain),
	}
}

func (se *ServiceEntry) callback(h CallbackHook, e error, args ...interface{}) {
	if se.Callback != nil {
		se.Callback(h, e, args)
	}
}
