package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoverDevolve500EmJSON(t *testing.T) {
	quebrado := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]int
		m["x"] = 1 // panic: escrita em map nil
	})

	rec := httptest.NewRecorder()
	Recover(quebrado).ServeHTTP(rec, httptest.NewRequest("GET", "/standings", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status esperado 500, veio %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] == "" {
		t.Errorf("esperado {\"error\": ...}, veio %q", rec.Body.String())
	}
}

func TestRequestLoggerGuardaStatus(t *testing.T) {
	casos := map[string]http.HandlerFunc{
		"WriteHeader explícito": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) },
		"só Write (200)":        func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) },
	}
	esperado := map[string]int{"WriteHeader explícito": http.StatusTeapot, "só Write (200)": http.StatusOK}

	for nome, h := range casos {
		var capturado int
		spy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h(w, r)
			capturado = w.(*statusRecorder).status
		})
		RequestLogger(spy).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
		if capturado != esperado[nome] {
			t.Errorf("%s: status capturado %d, esperado %d", nome, capturado, esperado[nome])
		}
	}
}
