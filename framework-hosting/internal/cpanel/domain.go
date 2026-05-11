package cpanel

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type Domains struct {
	MainDomain    string   `json:"main_domain"`
	AddonDomains  []string `json:"addon_domains"`
	ParkedDomains []string `json:"parked_domains"`
	SubDomains    []string `json:"sub_domains"`
}

func (c *Client) ListDomains() (*Domains, error) {
	resp, err := c.Call("DomainInfo", "list_domains", url.Values{})
	if err != nil {
		return nil, err
	}
	var d Domains
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		return nil, fmt.Errorf("parse list_domains data: %w", err)
	}
	return &d, nil
}
