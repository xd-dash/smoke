package xdroute

// TXTRecord is the provider-neutral DNS record projection of a Route.
// Provider adapters translate this value into their own API types.
type TXTRecord struct {
	Name    string
	Content string
}

func (r Route) Record(zone string) (TXTRecord, error) {
	name, err := r.Identity().DNSName(zone)
	if err != nil {
		return TXTRecord{}, err
	}
	content, err := r.TXT()
	if err != nil {
		return TXTRecord{}, err
	}
	return TXTRecord{Name: name, Content: content}, nil
}
