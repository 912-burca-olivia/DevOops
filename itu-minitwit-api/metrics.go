package main

import "github.com/prometheus/client_golang/prometheus"



type Metrics struct {
	SuccessfulRequests *prometheus.CounterVec
	MessagesSent *prometheus.CounterVec
	UnfollowRequests *prometheus.CounterVec
	FollowRequests *prometheus.CounterVec
	BadRequests *prometheus.CounterVec
}

func InitMetrics() *Metrics {
	m := &Metrics{
		SuccessfulRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "successful_request",
				Help: "Total number of successful (2xx) HTTP requests",
			},
			[]string{"path"},
		),
		MessagesSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "successful_message",
				Help: "Total number of successfully sent messages",
			},
			[]string{"path"},
		),
		UnfollowRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "successful_unfollows",
				Help: "Total number of successfully sent unfollow requests",
			},
			[]string{"path"},
		),
		FollowRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "successful_follows",
				Help: "Total number of successfully sent follow requests",
			},
			[]string{"path"},
		),
		BadRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "unsuccessful_request",
				Help: "Total number of unsuccessful (4xx) HTTP requests",
			},
			[]string{"path"},
		),
	}

	prometheus.MustRegister(m.SuccessfulRequests)
	prometheus.MustRegister(m.BadRequests)
	prometheus.MustRegister(m.FollowRequests)
	prometheus.MustRegister(m.UnfollowRequests)
	prometheus.MustRegister(m.MessagesSent)

	return m
}
