package dns

import (
	"context"
	"net"
	"sync"
	"time"
)

type Resolver struct {
	lookup  func(ctx context.Context, host string) ([]net.IP, error)
	timeout time.Duration

	ips map[string][]net.IP
	*sync.RWMutex
}

type Dial func(ctx context.Context, network string, host string) (net.Conn, error)

var (
	Dialer = &net.Dialer{
		Timeout:   time.Second * 15,
		KeepAlive: time.Second * 30,
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network string, host string) (net.Conn, error) {
				return (&net.Dialer{
					Timeout: time.Second * 15,
				}).DialContext(ctx, "udp", "1.1.1.1:53")
			},
		},
	}
)

func lookup(ctx context.Context, host string) ([]net.IP, error) {
	adr, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, len(adr))
	for idx := range adr {
		ips[idx] = adr[idx].IP
	}

	return ips, nil
}

func New() *Resolver {
	tick := time.NewTicker(time.Minute)

	resolver := &Resolver{
		lookup:  lookup,
		timeout: time.Second * 15,

		ips:     make(map[string][]net.IP, 50),
		RWMutex: new(sync.RWMutex),
	}

	go func() {
		for {
			<-tick.C
			resolver.Refresh()
		}
	}()

	return resolver
}

func (resolver *Resolver) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	ips, err := resolver.lookup(ctx, host)
	if err != nil {
		return nil, err
	}

	resolver.Lock()
	resolver.ips[host] = ips
	resolver.Unlock()

	return ips, nil
}

func (resolver *Resolver) Fetch(ctx context.Context, host string) ([]net.IP, error) {
	resolver.RLock()
	ips, ok := resolver.ips[host]
	resolver.RUnlock()

	if ok {
		return ips, nil
	}

	return resolver.Lookup(ctx, host)
}

func (resolver *Resolver) Refresh() {

	resolver.RLock()
	hosts := make([]string, len(resolver.ips))
	for host := range resolver.ips {
		hosts = append(hosts, host)
	}
	resolver.RUnlock()

	for _, host := range hosts {
		ctx, can := context.WithTimeout(context.Background(), resolver.timeout)
		resolver.Lookup(ctx, host)
		can()
	}
}

func (resolver *Resolver) Dial() Dial {
	
	return func(ctx context.Context, network string, host string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(host)
		if err != nil {
			return nil, err
		}

		ips, err := resolver.Fetch(ctx, host)
		if err != nil {
			return nil, err
		}

		var conn net.Conn
		for idx := range ips {
			conn, err = Dialer.DialContext(ctx, network, net.JoinHostPort(ips[idx].String(), port))
			if err == nil {
				break
			}
		}
		return conn, err
	}
}
