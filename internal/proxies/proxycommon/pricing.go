package proxycommon

import "strings"

// CatalogEntry is the minimum a vendor catalog entry exposes for
// LookupPricing.
type CatalogEntry[P any] interface {
	Prefix() string
	Price() P
}

// LookupPricing returns the first entry whose Prefix is a prefix of id,
// plus its pricing. ok=false if no match.
func LookupPricing[E CatalogEntry[P], P any](catalog []E, id string) (P, bool) {
	var zero P
	for _, e := range catalog {
		if strings.HasPrefix(id, e.Prefix()) {
			return e.Price(), true
		}
	}
	return zero, false
}
