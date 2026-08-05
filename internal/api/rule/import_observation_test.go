package rule

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jproxy-go/internal/store/sqlite"
)

type importObservation struct {
	partReads            int
	filesystemOperations int
}

func (o *importObservation) ObservePartRead() { o.partReads++ }

func (o *importObservation) ObserveFilesystemOperation() { o.filesystemOperations++ }

func TestHandler_importConsumesMultipartPartWithoutFilesystemOperations(t *testing.T) {
	// Given
	store, err := sqlite.Open(context.Background(), t.TempDir()+"/import.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	observation := &importObservation{}
	handler := NewHandler(Options{
		Store: store, Domain: "sonarr", MultipartObserver: observation,
	})
	boundary := "observation"
	body := "--" + boundary + "\r\nContent-Disposition: form-data; name=\"file\"; filename=\"rules.json\"\r\nContent-Type: application/json\r\n\r\n[{\"id\":\"rule\",\"token\":\"title\",\"regex\":\".*\",\"replacement\":\"x\",\"example\":\"x\"}]\r\n--" + boundary + "--\r\n"
	request := httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/import", strings.NewReader(body))
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	response := httptest.NewRecorder()

	// When
	handler.ServeHTTP(response, request)

	// Then
	if response.Code != http.StatusOK || observation.partReads != 1 || observation.filesystemOperations != 0 || bytes.Contains(response.Body.Bytes(), []byte("rules.json")) {
		t.Fatalf("status=%d part_reads=%d filesystem_operations=%d", response.Code, observation.partReads, observation.filesystemOperations)
	}
}
