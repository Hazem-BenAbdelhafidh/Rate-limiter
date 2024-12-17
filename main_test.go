package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const errMsg string = "Error while hitting the route /%s : %v"
const route string = "%s/%s"

func TestNewRequest(t *testing.T) {
	req := newRequest(10)
	if req.count != 9 {
		t.Errorf("Expected count to be 9 but got %d", req.count)
	}

	if req.lastRequest.Sub(time.Now()) > time.Second*2 {
		t.Errorf("last request is not correct")
	}

}

func TestNewRateLimiter(t *testing.T) {
	rl := newRateLimiter(10, 10*time.Second)
	if rl.numOfReq != 10 {
		t.Errorf("Expected numOfReq to be 10 but got %d", rl.numOfReq)
	}

	if rl.timeRange != 10*time.Second {
		t.Errorf("Expected timeRange to be 10 seconds but got %f seconds", rl.timeRange.Seconds())
	}
}

func TestNewServer(t *testing.T) {
	rl := newRateLimiter(10, 10*time.Second)
	server := NewServer(8080, rl)

	if server.port != 8080 {
		t.Errorf("Expected port to be 8080 seconds but got %d", server.port)
	}
}

func TestHandleLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(handleLimited))
	url := server.URL
	resp, err := http.Get(fmt.Sprintf(route, url, "limited"))
	if err != nil {
		t.Errorf(errMsg, "/limited", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK")
	}

	for i := 0; i < 11; i++ {
		resp, err := http.Get(fmt.Sprintf(route, url, "limited"))
		if err != nil {
			t.Errorf(errMsg, "/limited", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK but got status : %d", resp.StatusCode)
		}
	}
}

func TestHandleUnlimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(handleUnlimited))
	url := server.URL
	resp, err := http.Get(fmt.Sprintf(route, url, "unlimited"))
	if err != nil {
		t.Errorf(errMsg, "/unlimited", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK")
	}

	for i := 0; i < 10; i++ {
		resp, err := http.Get(fmt.Sprintf(route, url, "unlimited"))
		if err != nil {
			t.Errorf(errMsg, "/unlimited", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK but got status : %d", resp.StatusCode)
		}

	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		rl      RateLimiter
		request *Request
		err     error
	}{
		{
			rl:      *newRateLimiter(10, 10*time.Second),
			request: newRequest(10),
			err:     nil,
		},
		{
			rl:      *newRateLimiter(10, 10*time.Second),
			request: newRequest(1),
			err:     errors.New("Too many requests, please try again later"),
		},
	}

	for index, test := range tests {
		test.rl.requestsMap["127.0.0.1"] = test.request
		err := test.rl.check("127.0.0.1")
		if err != nil && err.Error() != test.err.Error() {
			t.Errorf("test %d failed expected error : %v but got error : %v", index, test.err, err)
		}
	}

}
