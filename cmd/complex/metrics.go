package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

var metrics sync.Map

func Add(metric string, value int64) int64 {
	prev, ok := metrics.LoadOrStore(metric, value)
	curr := prev.(int64)
	if ok {
		curr += value
		metrics.Store(metric, curr)
	}

	return curr
}

func Inc(metric string) int64 {
	return Add(metric, 1)
}

func Range(fn func(key string, value int64) bool) {
	metrics.Range(func(key, value interface{}) bool {
		return fn(key.(string), value.(int64))
	})
}

func Snapshot() map[string]int64 {
	snapshot := make(map[string]int64)

	Range(func(key string, value int64) bool {
		snapshot[key] = value

		return true
	})

	return snapshot
}

var SnapshotHTTPEndpoint http.HandlerFunc = func(w http.ResponseWriter, _ *http.Request) {
	payload, err := json.MarshalIndent(Snapshot(), "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}
}
