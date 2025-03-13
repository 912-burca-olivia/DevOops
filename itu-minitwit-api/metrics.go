package main

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	SuccessfulRequests *prometheus.CounterVec
}