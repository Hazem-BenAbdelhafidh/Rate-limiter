package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Request struct {
	count       int
	lastRequest time.Time
}

func newRequest(count int) *Request {
	return &Request{
		count:       count - 1,
		lastRequest: time.Now(),
	}
}

type RateLimiter struct {
	numOfReq    int
	timeRange   time.Duration
	requestsMap map[string]*Request
	sync.Mutex
}

func newRateLimiter(numOfReq int, timeRange time.Duration) *RateLimiter {
	return &RateLimiter{
		numOfReq:    numOfReq,
		timeRange:   timeRange,
		requestsMap: make(map[string]*Request),
	}
}

func (rl *RateLimiter) check(userIP string) error {
	rl.Lock()
	defer rl.Unlock()
	req, exists := rl.requestsMap[userIP]
	timeElapsed := time.Since(req.lastRequest)
	if !exists || req == nil || timeElapsed > rl.timeRange {
		request := newRequest(rl.numOfReq)
		rl.requestsMap[userIP] = request
		return nil
	}

	if timeElapsed <= rl.timeRange && req.count > 0 {
		req.count--
		req.lastRequest = time.Now()
	}

	if req.count <= 0 && timeElapsed <= rl.timeRange {
		return errors.New("Too many requests, please try again later")
	}
	return nil
}

func (s *Server) getIpMiddleware(next http.Handler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var userIP string
		if len(r.Header.Get("CF-Connecting-IP")) > 1 {
			userIP = r.Header.Get("CF-Connecting-IP")
			log.Println(net.ParseIP(userIP))
		} else if len(r.Header.Get("X-Forwarded-For")) > 1 {
			userIP = r.Header.Get("X-Forwarded-For")
			log.Println(net.ParseIP(userIP))
		} else if len(r.Header.Get("X-Real-IP")) > 1 {
			userIP = r.Header.Get("X-Real-IP")
			log.Println(net.ParseIP(userIP))
		} else {
			userIP = r.RemoteAddr
			if strings.Contains(userIP, ":") {
				log.Println(net.ParseIP(strings.Split(userIP, ":")[0]))
				userIP = strings.Split(userIP, ":")[0]
			} else {
				log.Println(net.ParseIP(userIP))
			}
		}

		err := s.rateLimiter.check(userIP)
		if err != nil {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
			return
		}

		ctx := context.WithValue(r.Context(), "ip", userIP)
		next.ServeHTTP(w, r.WithContext(ctx))
	}

}

func handleLimited(w http.ResponseWriter, r *http.Request) {
	log.Println("limited hit")
}

func handleUnlimited(w http.ResponseWriter, r *http.Request) {
	log.Println("unlimited hit ")
}

type Server struct {
	rateLimiter *RateLimiter
	port        int
}

func NewServer(port int, rateLimiter *RateLimiter) *Server {
	return &Server{
		port:        port,
		rateLimiter: rateLimiter,
	}
}

func (s *Server) start() {
	addr := fmt.Sprintf(":%d", s.port)
	mux := http.NewServeMux()
	mux.HandleFunc("/limited", s.getIpMiddleware(http.HandlerFunc(handleLimited)))
	mux.HandleFunc("/unlimited", handleUnlimited)
	log.Println("Server Listening on port :", s.port)
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		log.Println("error while starting the server : ", err.Error())
	}
}

func clearExpired(rateLimiter *RateLimiter) {
	rateLimiter.Lock()
	requestsMap := rateLimiter.requestsMap
	for {
		time.Sleep(1 * time.Minute)
		for key, value := range requestsMap {
			if time.Since(value.lastRequest) > 1*time.Hour {
				log.Println("deleting ip address : ", key, "from late limiter")
				delete(requestsMap, key)
			}
		}
		rateLimiter.Unlock()
	}
}

func main() {
	rateLimiter := newRateLimiter(10, 10*time.Second)
	go clearExpired(rateLimiter)
	server := NewServer(8000, rateLimiter)
	server.start()
}
