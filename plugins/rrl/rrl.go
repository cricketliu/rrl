package rrl

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/rrl/plugins/rrl/cache"

	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

// RRL performs response rate limiting
type RRL struct {
	Next  plugin.Handler
	Zones []string

	window int

	ipv4PrefixLength int
	ipv6PrefixLength int

	responsesPerSecond int
	nodataPerSecond    int
	nxdomainsPerSecond int
	referralsPerSecond int
	errorsPerSecond    int

	maxTableSize int

	table *cache.Cache
}

// ResponseAccount holds accounting for a category of response
type ResponseAccount struct {
	allowance int
	lastCheck time.Time
	balance   int
}

// Theses constants are categories of response types
const (
	rTypeResponse = 0
	rTypeNodata   = 1
	rTypeNxdomain = 2
	rTypeReferral = 3
	rTypeError    = 4
)

// responseType returns the RRL response type for a response
func responseType(m dns.Msg) byte {
	if len(m.Answer) > 0 {
		return rTypeResponse
	} else if m.Rcode == dns.RcodeNameError {
		return rTypeNxdomain
	} else if m.Rcode == dns.RcodeSuccess {
		// todo: determine if this is actually a referral, assume nodata for now
		return rTypeNodata
	} else {
		return rTypeError
	}
}

// allowanceForRtype returns the per second allowance for the given rtype
func (rrl *RRL) allowanceForRtype(rtype uint8) int {
	switch rtype {
	case rTypeResponse:
		return rrl.responsesPerSecond
	case rTypeNodata:
		return rrl.nodataPerSecond
	case rTypeNxdomain:
		return rrl.nxdomainsPerSecond
	case rTypeReferral:
		return rrl.referralsPerSecond
	case rTypeError:
		return rrl.errorsPerSecond
	}
	return -1
}

// initTable creates a new cache table and sets the cache eviction function
func (rrl *RRL) initTable() {
	rrl.table = cache.New(rrl.maxTableSize)
	// This eviction function returns true if the allowance is >= max value (window)
	rrl.table.SetEvict(func(el *interface{}) bool {
		ra, ok := (*el).(ResponseAccount)
		if !ok {
			return true
		}
		return ra.allowance*int(time.Now().Sub(ra.lastCheck).Seconds()) >= rrl.window
	})
}

// responseToToken returns a token string for the given inputs
func (rrl *RRL) responseToToken(rtype uint8, qtype uint16, name, remoteAddr string) string {
	var qname string

	if rtype != rTypeError {
		qname = name
	}

	qtypeStr := strconv.FormatUint(uint64(qtype), 10)
	rtypestr := strconv.FormatUint(uint64(rtype), 10)

	prefix := rrl.addrPrefix(remoteAddr)

	return strings.Join([]string{prefix, rtypestr, qtypeStr, qname}, "/")
}

// debit will decrement an existing response account in the rrl table by one and recalculate the current balance,
// or if the response account does not exist, it will add it.
func (rrl *RRL) debit(allowance int, t string) (int, error) {
	result := rrl.table.UpdateAdd(t,
		// the 'update' function debits the account and returns the new balance
		func(el *interface{}) interface{} {
			ra := (*el).(*ResponseAccount)
			if ra == nil {
				return nil
			}
			now := time.Now()
			ra.balance += allowance*int(now.Sub(ra.lastCheck).Seconds()) - 1
			if ra.balance >= rrl.window {
				// balance can't exceed window
				ra.balance = rrl.window - 1
			} else if min := -1 * rrl.window * allowance; ra.balance < min {
				// balance can't be more negative than window * allowance
				ra.balance = min
			}
			ra.lastCheck = now
			return ra.balance
		},
		// the 'add' function returns a new ResponseAccount for the response type
		func() interface{} {
			ra := &ResponseAccount{
				allowance: allowance,
				lastCheck: time.Now(),
				balance:   rrl.window - 1,
			}
			return ra
		})

	if result == nil {
		return 0, nil
	}
	if err, ok := result.(error); ok {
		return 0, err
	}
	if balance, ok := result.(int); ok {
		return balance, nil
	}
	return 0, errors.New("unexpected result type")
}

// addrPrefix returns the address prefix of the net.Addr style address string (e.g. 1.2.3.4:1234 or [1:2::3:4]:1234)
func (rrl *RRL) addrPrefix(addr string) string {
	i := strings.LastIndex(addr, ":")
	ip := net.ParseIP(addr[:i])
	if ip.To4() != nil {
		ip = ip.Mask(net.CIDRMask(rrl.ipv4PrefixLength, 32))
		return ip.String()
	}
	ip = net.ParseIP(addr[1 : i-1]) // strip brackets from ipv6 e.g. [2001:db8::1]
	ip = ip.Mask(net.CIDRMask(rrl.ipv6PrefixLength, 128))

	return ip.String()
}
