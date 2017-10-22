package httpTest

import (
	"testing"
	"net/http"
)

func handler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("This is an example server.\n"))
}

func TestTLS(t *testing.T) {
	http.HandleFunc("/", handler)
	err := http.ListenAndServeTLS(":6767", "tests/test.crt", "tests/test.key", nil)
	if err != nil {
		t.Errorf("Could not create listener: %s", err.Error())
	}
}
