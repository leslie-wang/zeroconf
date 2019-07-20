package zeroconf

import (
	"context"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
)

var (
	mdnsName    = "test--xxxxxxxxxxxx"
	mdnsService = "test--xxxx.tcp"
	mdnsDomain  = "local."
	mdnsPort    = 8888
)

func startMDNS(ctx context.Context, port int, name, service, domain string, cb Callback) {
	// 5353 is default mdns port
	entry := NewServiceEntry(name, service, domain)
	entry.Port = port
	entry.Text = []string{"txtv=0", "lo=1", "la=2"}
	entry.Callback = cb

	server, err := RegisterServiceEntry(entry, nil)
	if err != nil {
		panic(errors.Wrap(err, "while registering mdns service"))
	}
	defer server.Shutdown()
	log.Printf("Published service: %s, type: %s, domain: %s", name, service, domain)

	<-ctx.Done()

	log.Printf("Shutting down.")

}

func TestBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go startMDNS(ctx, mdnsPort, mdnsName, mdnsService, mdnsDomain, nil)

	time.Sleep(time.Second)

	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("Expected create resolver success, but got %v", err)
	}
	entries := make(chan *ServiceEntry)
	expectedResult := []*ServiceEntry{}
	go func(results <-chan *ServiceEntry) {
		s := <-results
		expectedResult = append(expectedResult, s)
	}(entries)

	if err := resolver.Browse(ctx, mdnsService, mdnsDomain, entries); err != nil {
		t.Fatalf("Expected browse success, but got %v", err)
	}
	<-ctx.Done()

	if len(expectedResult) != 1 {
		t.Fatalf("Expected number of service entries is 1, but got %d", len(expectedResult))
	}
	if expectedResult[0].Domain != mdnsDomain {
		t.Fatalf("Expected domain is %s, but got %s", mdnsDomain, expectedResult[0].Domain)
	}
	if expectedResult[0].Service != mdnsService {
		t.Fatalf("Expected service is %s, but got %s", mdnsService, expectedResult[0].Service)
	}
	if expectedResult[0].Instance != mdnsName {
		t.Fatalf("Expected instance is %s, but got %s", mdnsName, expectedResult[0].Instance)
	}
	if expectedResult[0].Port != mdnsPort {
		t.Fatalf("Expected port is %d, but got %d", mdnsPort, expectedResult[0].Port)
	}

	// wait 1 second for mdns server to cool down
	time.Sleep(time.Second)
}

func TestNoRegister(t *testing.T) {
	// wait 5 second until last cache is expired
	time.Sleep(5 * time.Second)

	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("Expected create resolver success, but got %v", err)
	}

	// before register, mdns resolve shuold not have any entry
	entries := make(chan *ServiceEntry)
	go func(results <-chan *ServiceEntry) {
		s := <-results
		if s != nil {
			t.Fatalf("Expected empty service entries but got %v", *s)
		}
	}(entries)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := resolver.Browse(ctx, mdnsService, mdnsDomain, entries); err != nil {
		t.Fatalf("Expected browse success, but got %v", err)
	}
	<-ctx.Done()
	cancel()
}

func TestCallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pairs := map[dns.Question]dns.Msg{}
	cb := func(h CallbackHook, e error, a []interface{}) {
		switch h {
		case NetworkV4:
			if !strings.Contains(e.Error(), "closed network connection") {
				t.Fatalf("receive packet from v4 interface got error: %v", e)
			}
		case NetworkV6:
			if !strings.Contains(e.Error(), "closed network connection") {
				t.Fatalf("receive packet from v6 interface got error: %v", e)
			}
		case Unpack:
			if e != nil {
				t.Fatalf("receive unpack message: %v\n", e)
			}
		case ResponseUnicast:
			if e != nil {
				t.Fatalf("receive unicast response: %v\n", e)
			}
			newResp := a[1].(dns.Msg)
			newResp.Id = 0
			resp, ok := pairs[a[0].(dns.Question)]
			if ok {
				// besides id, everything else should be equal
				if !reflect.DeepEqual(resp, newResp) {
					t.Fatalf("got diff response: %v ---- %v\n", resp, a[1])
				}
			} else {
				pairs[a[0].(dns.Question)] = newResp
			}
		case ResponseMulticast:
			if e != nil {
				t.Fatalf("receive multicast reponse: %v\n", e)
			}
			t.Fatalf("should not receive multicast reponse")
		}
	}

	go startMDNS(ctx, mdnsPort, mdnsName, mdnsService, mdnsDomain, cb)

	time.Sleep(time.Second)

	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("Expected create resolver success, but got %v", err)
	}
	entries := make(chan *ServiceEntry)
	go func(results <-chan *ServiceEntry) {
		<-results
	}(entries)

	if err := resolver.Browse(ctx, mdnsService, mdnsDomain, entries); err != nil {
		t.Fatalf("Expected browse success, but got %v", err)
	}
	<-ctx.Done()

	if len(pairs) != 1 {
		t.Fatalf("Expected same reply for the request, but got %v", pairs)
	}

	// wait 1 second for mdns server to cool down
	time.Sleep(time.Second)
}

func TestCallbackWrongName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cb := func(h CallbackHook, e error, a []interface{}) {
		switch h {
		case NetworkV4:
			if !strings.Contains(e.Error(), "closed network connection") {
				t.Fatalf("receive packet from v4 interface got error: %v", e)
			}
		case NetworkV6:
			if !strings.Contains(e.Error(), "closed network connection") {
				t.Fatalf("receive packet from v6 interface got error: %v", e)
			}
		case Unpack:
			if e != nil {
				t.Fatalf("receive unpack message: %v\n", e)
			}
		case ResponseUnicast:
			if e != nil {
				t.Fatalf("receive unicast response: %v\n", e)
			}
			t.Fatalf("should not receive unicast reponse")
		case ResponseMulticast:
			if e != nil {
				t.Fatalf("receive multicast reponse: %v\n", e)
			}
			t.Fatalf("should not receive multicast reponse")
		}
	}

	go startMDNS(ctx, mdnsPort, mdnsName, mdnsService, mdnsDomain, cb)

	time.Sleep(time.Second)

	resolver, err := NewResolver(nil)
	if err != nil {
		t.Fatalf("Expected create resolver success, but got %v", err)
	}

	for k, v := range map[string]string {"invalid": mdnsDomain, mdnsService: "invalid", "all_invalid": "invalid"} {
		entries := make(chan *ServiceEntry)
		go func(results <-chan *ServiceEntry) {
			<-results
		}(entries)

		go func() {
			if err := resolver.Browse(ctx, k, v, entries); err != nil {
				t.Fatalf("Expected browse success, but got %v", err)
			}
		}()
	}

	<-ctx.Done()

	// wait 1 second for mdns server to cool down
	time.Sleep(time.Second)
}
