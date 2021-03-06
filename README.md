# rrl

## Name

*rrl* - provides response rate limiting to help mitigate DNS amplification attacks.

## Description

The *rrl* plugin tracks response rates per category of response. The category of a given response consists of the following:

* Prefix of the client IP (per the  ipv4/6-prefix-length)
* Requested name (qname) excluding response type of error (see response type below)
* Requested type (qtype) excluding response type of error (see response type below)
* Response type (each corresponding to the configurable per-second allowances)
  * response - for positive responses that contain answers
  * nodata - for NODATA responses
  * nxdomain - for NXDOMAIN responses
  * referrals - for referrals or delegations
  * error - for all DNS errors (except NXDOMAIN)

To better protect against attacks using invalid requests, requested name and type are not categorized separately for error type requests. In other words, all error responses are limited collectively per client, regardless of qname or qtype.

Each category has an account balance which is credited at a rate of the configured *per-second* allowance for that response type, and debited for each time a response in that catgegory would be sent to a client.  When an account balance is negative, responses in the category are dropped until the balance goes non-negative.  Account balances cannot be more positive than *window*, and cannot be more negative than *window* * *per-second* allowance.


This implmentation intends to replicate the behavior of BIND 9 response rate limiting feature.

## Syntax

```
rrl [ZONES...] {
    window SECONDS
    ipv4-prefix-length LENGTH
    ipv6-prefix-length LENGTH
    responses-per-second ALLOWANCE
    nodata-per-second ALLOWANCE
    nxdomains-per-second ALLOWANCE
    referrals-per-second ALLOWANCE
    errors-per-second ALLOWANCE
    max-table-size SIZE
}
```

* `window SECONDS` - defines a rolling window in SECONDS during which response rates are tracked. Default 15

* `ipv4-prefix-length LENGTH` - the prefix LENGTH in bits to use for identifying a ipv4 client. Default 24

* `ipv6-prefix-length LENGTH` - the prefix LENGTH in bits to use for identifying a ipv6 client. Default 56

* `responses-per-second ALLOWANCE` - the number of positive responses allowed per second. Default 0

* `nodata-per-second ALLOWANCE` - the number of empty (NODATA) responses allowed per second. Defaults to responses-per-second.

* `nxdomains-per-second ALLOWANCE` - the number of negative (NXDOMAIN) responses allowed per second. Defaults to responses-per-second.

* `referrals-per-second ALLOWANCE` - the number of negative (NXDOMAIN) responses allowed per second. Defaults to responses-per-second.

* `errors-per-second ALLOWANCE` - the number of error responses allowed per second (excluding NXDOMAIN). Defaults to responses-per-second.

* `max-table-size SIZE` - the maximum number of responses to be tracked at one time. When exceeded, rrl stops rate limiting new responses.


## Examples

Example 1

~~~ corefile

. {
  rrl . {
    responses-per-second 10
  }
}

~~~

## Bugs

## Additional References

[A Quick Introduction to Response Rate Limiting](https://kb.isc.org/docs/aa-01000)

[This Plugin's Design Spec](./README-DEV.md)
