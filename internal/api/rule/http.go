package rule

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func decodeJSON(r *http.Request, target any) error {
	d := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple json values")
	}
	return nil
}
func writeError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"request failed"}`))
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
