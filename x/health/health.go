package health

import (
	"encoding/json"
	"net/http"
)

type Checker interface {
	HealthCheck() error
}

func HttpHandler(checker map[string]Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var healthy bool = true
		result := map[string]bool{}
		for k, v := range checker {
			result[k] = true
			if v.HealthCheck() != nil {
				healthy = false
				result[k] = false
			}
		}
		rs, _ := json.Marshal(map[string]interface{}{"healthy": healthy, "result": result})
		w.Header().Set("Content-Type", "application/json")
		if healthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(rs)
	}
}

func HttpLivenessHanlder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
