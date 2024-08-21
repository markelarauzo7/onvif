package networking

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/beevik/etree"
	"github.com/icholy/digest"
	"github.com/juju/errors"
)

// SendSoap send soap message
func SendSoap(httpClient *http.Client, endpoint, message string) (*http.Response, error) {
	resp, err := httpClient.Post(endpoint, "application/soap+xml; charset=utf-8", bytes.NewBufferString(message))
	if err != nil {
		return resp, errors.Annotate(err, "Post")
	}

	// if resp.StatusCode is 4xx,5xx, return error
	if resp.StatusCode >= 400 && resp.StatusCode < 600 {
		return resp, errors.Errorf("Server error: %d: %s", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

func SendSoapWithDigest(httpClient *http.Client, endpoint, message, username, password string) (*http.Response, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(message); err != nil {
		return nil, err
	}

	e := doc.FindElement("./Envelope/Header/Security")
	if e != nil {
		bodyTag := doc.Root().SelectElement("Header")
		bodyTag.RemoveChild(e)
		data, err := doc.WriteToString()
		if err != nil {
			return nil, err
		}
		message = data
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(message))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return resp, errors.Annotate(err, "Post with digest")
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}

	wwwAuth := resp.Header.Get("WWW-Authenticate")
	chal, err := digest.ParseChallenge(wwwAuth)
	if err != nil {
		return resp, fmt.Errorf("fail to parse challenge: %w", err)
	}

	cred, err := digest.Digest(chal, digest.Options{
		Method:   "POST",
		URI:      req.URL.RequestURI(),
		Username: username,
		Password: password,
	})

	if err != nil {
		return resp, fmt.Errorf("fail to build digest: %w", err)
	}

	req.Header.Add("Authorization", cred.String())
	req.Body = io.NopCloser((bytes.NewBufferString(message)))
	resp, err = httpClient.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "Post with digest")
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 600 {
		return resp, errors.Errorf("Post with digest error: %d: %s", resp.StatusCode, resp.Status)
	}

	return resp, nil
}
