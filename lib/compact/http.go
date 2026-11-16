package compact

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPAPI calls the server's HTTP API (/api/<version>/<function>) with an
// API key.
type HTTPAPI struct {
	BaseURL string // e.g. http://localhost:9001
	APIKey  string
	Client  *http.Client
	version string
}

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// apiVersion asks the server for its current API version once.
func (h *HTTPAPI) apiVersion() (string, error) {
	if h.version != "" {
		return h.version, nil
	}
	resp, err := h.client().Get(strings.TrimSuffix(h.BaseURL, "/") + "/api/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := redirected(resp); err != nil {
		return "", err
	}
	var v struct {
		CurrentVersion string `json:"currentVersion"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil || v.CurrentVersion == "" {
		return "", fmt.Errorf("%s/api/ answered no API version (HTTP %d); is it an Etherpad server?", h.BaseURL, resp.StatusCode)
	}
	h.version = v.CurrentVersion
	return h.version, nil
}

func (h *HTTPAPI) client() *http.Client {
	if h.Client == nil {
		h.Client = &http.Client{
			Timeout: 10 * time.Minute, // compacting a big pad takes a while
			// Redirects are not followed: the API key goes in a header,
			// and a redirect could carry it to another host. A server
			// that redirects (http to https, say) is reported, so --url
			// can name the address it redirects to.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return h.Client
}

// redirected reports a redirect answer as an error naming its target.
func redirected(resp *http.Response) error {
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return fmt.Errorf("the server redirects (HTTP %d to %s); give that address with --url", resp.StatusCode, resp.Header.Get("Location"))
	}
	return nil
}

// call POSTs a function (the key in the header, not in a URL that may end
// up in logs) and decodes its data into out.
func (h *HTTPAPI) call(function string, params url.Values, out any) error {
	version, err := h.apiVersion()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(h.BaseURL, "/")+"/api/"+version+"/"+function, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("apikey", h.APIKey)
	resp, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := redirected(resp); err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	var r apiResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if r.Code != 0 {
		if r.Code == 4 {
			return fmt.Errorf("%s (check the API key)", r.Message)
		}
		return fmt.Errorf("%s", r.Message)
	}
	if out != nil {
		return json.Unmarshal(r.Data, out)
	}
	return nil
}

func (h *HTTPAPI) ListAllPads() ([]string, error) {
	var d struct {
		PadIDs []string `json:"padIDs"`
	}
	err := h.call("listAllPads", url.Values{}, &d)
	return d.PadIDs, err
}

func (h *HTTPAPI) GetLastEdited(padID string) (int64, error) {
	var d struct {
		LastEdited int64 `json:"lastEdited"`
	}
	err := h.call("getLastEdited", url.Values{"padID": {padID}}, &d)
	return d.LastEdited, err
}

func (h *HTTPAPI) GetRevisionsCount(padID string) (int, error) {
	var d struct {
		Revisions int `json:"revisions"`
	}
	err := h.call("getRevisionsCount", url.Values{"padID": {padID}}, &d)
	return d.Revisions, err
}

func (h *HTTPAPI) CompactPad(padID string, keep *int) error {
	params := url.Values{"padID": {padID}}
	if keep != nil {
		params.Set("keepRevisions", strconv.Itoa(*keep))
	}
	return h.call("compactPad", params, nil)
}
