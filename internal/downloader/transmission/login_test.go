package transmission

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"jproxy-go/internal/runtime"
)

func TestClient_logsInWithVersionField_whenSessionResponseIsValid(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method    string `json:"method"`
			Arguments struct {
				Fields []string `json:"fields"`
			} `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Method != "session-get" || len(request.Arguments.Fields) != 1 || request.Arguments.Fields[0] != "version" {
			t.Fatal("unexpected session-get request")
		}
		_, _ = io.WriteString(w, `{"result":"success","arguments":{"version":"4.0.6"}}`)
	}))
	defer upstream.Close()
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password"}})

	// When
	err := client.Login(context.Background())

	// Then
	if err != nil {
		t.Fatal("valid session-get response was rejected")
	}
}

func TestClient_rejectsMalformedSessionArguments_whenVersionIsInvalid(t *testing.T) {
	for _, arguments := range []string{`"invalid"`, `null`, `[]`, `{}`, `{"version":" "}`, `{"version":7}`} {
		t.Run(arguments, func(t *testing.T) {
			// Given
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{"result":"success","arguments":`+arguments+`}`)
			}))
			defer upstream.Close()

			// When
			err := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password"}}).Login(context.Background())

			// Then
			if !errors.Is(err, ErrMalformedResponse) {
				t.Fatal("malformed session-get response was accepted")
			}
		})
	}
}
