package arca

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// doSOAP hace un POST SOAP 1.1 y devuelve el body crudo. El parseo lo hace el
// caller (WSAA / WSFEv1) según la operación.
func (c *Client) doSOAP(ctx context.Context, url, soapAction string, envelope []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(envelope))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", soapAction)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("arca soap request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return body, fmt.Errorf("arca soap http %d", resp.StatusCode)
	}
	return body, nil
}
