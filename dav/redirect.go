package dav

import (
	"io"
	"net/http"
)

const maxRedirects = 10

type redirectFollower struct {
	client *http.Client
}

func newRedirectFollower() *redirectFollower {
	return &redirectFollower{
		client: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (r *redirectFollower) Do(req *http.Request) (*http.Response, error) {
	for i := 0; ; i++ {
		resp, err := r.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < 300 || resp.StatusCode >= 400 || i >= maxRedirects {
			return resp, nil
		}
		location := resp.Header.Get("Location")
		if location == "" {
			return resp, nil
		}

		target, err := req.URL.Parse(location)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		resp.Body.Close()

		var body io.Reader
		if req.GetBody != nil {
			if body, err = req.GetBody(); err != nil {
				return nil, err
			}
		}

		next, err := http.NewRequestWithContext(
			req.Context(), req.Method, target.String(), body)
		if err != nil {
			return nil, err
		}
		next.Header = req.Header.Clone()
		next.ContentLength = req.ContentLength
		next.GetBody = req.GetBody
		if next.URL.Host != req.URL.Host {
			next.Header.Del("Authorization")
		}
		req = next
	}
}
