//go:build !no_spoof_tls_fingerprint

package http

import (
	"context"
	"io"
	"maps"
	"net/http"
	"net/url"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"libdb.so/dissent/internal/tls"
)

func NewClient(token string) *api.Client {
	httpClient := httputil.NewClientWithDriver(wrapClient(tls.NewClient(tls_client.WithCookieJar(tls_client.NewCookieJar()))))
	client := api.NewCustomClient(token, httpClient)
	client.UserAgent = BrowserUserAgent()
	return client
}

type client struct {
	tls_client.HttpClient
}

var _ httpdriver.Client = (*client)(nil)

func wrapClient(c tls_client.HttpClient) httpdriver.Client {
	return &client{HttpClient: c}
}

func (c *client) NewRequest(ctx context.Context, method, url string) (httpdriver.Request, error) {
	req, err := fhttp.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	return (*request)(req), nil
}

func (c *client) Do(req httpdriver.Request) (httpdriver.Response, error) {
	resp, err := c.HttpClient.Do((*fhttp.Request)(req.(*request)))
	if err != nil {
		return nil, err
	}
	return (*response)(resp), nil
}

type request fhttp.Request

var _ httpdriver.Request = (*request)(nil)

func (r *request) GetMethod() string {
	return r.Method
}

func (r *request) GetPath() string {
	return r.URL.Path
}

func (r *request) GetContext() context.Context {
	return (*fhttp.Request)(r).Context()
}

func (r *request) AddHeader(header http.Header) {
	maps.Copy(r.Header, fhttp.Header(header))
}

func (r *request) AddQuery(values url.Values) {
	query := r.URL.Query()
	for key, value := range values {
		query[key] = append(query[key], value...)
	}
	r.URL.RawQuery = query.Encode()
}

func (r *request) WithBody(body io.ReadCloser) {
	r.Body = body
}

type response fhttp.Response

var _ httpdriver.Response = (*response)(nil)

func (r *response) GetStatus() int {
	return r.StatusCode
}

func (r *response) GetHeader() http.Header {
	return http.Header(r.Header)
}

func (r *response) GetBody() io.ReadCloser {
	return r.Body
}
