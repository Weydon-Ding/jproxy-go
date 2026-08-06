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

func TestClient_renamesTorrentAndFile_whenRequestsAreValid(t *testing.T) {
	// Given
	var requests []receivedRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, decodeReceivedRequest(t, r.Body))
		if len(requests) == 1 {
			w.Header().Set("X-Transmission-Session-Id", "session")
			w.WriteHeader(http.StatusConflict)
			return
		}
		if requests[len(requests)-1].Method == "torrent-get" {
			_, _ = io.WriteString(w, `{"result":"success","arguments":{"torrents":[{"name":"old-root","files":[{"name":"dir/old.mkv"}]}]}}`)
			return
		}
		if requests[len(requests)-1].Arguments.Path == "old-root" {
			_, _ = io.WriteString(w, `{"result":"success","arguments":{"path":"old-root","name":"new-root","id":7}}`)
			return
		}
		_, _ = io.WriteString(w, `{"result":"success","arguments":{"path":"dir/old.mkv","name":"new.mkv","id":7}}`)
	}))
	defer upstream.Close()
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password", TransmissionRevision: 1}})

	// When
	err := client.Rename(context.Background(), "hash", "new-root")
	if err == nil {
		err = client.RenameFile(context.Background(), "hash", "dir/old.mkv", "dir/new.mkv")
	}

	// Then
	if err != nil || len(requests) != 4 {
		t.Fatal("unexpected rename result")
	}
	assertRenameArguments(t, requests[2], "old-root", "new-root")
	assertRenameArguments(t, requests[3], "dir/old.mkv", "new.mkv")
}

func TestClient_rejectsInvalidFileRename_whenPathEscapesSibling(t *testing.T) {
	// Given
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: "http://example.test", TransmissionUsername: "user", TransmissionPassword: "password"}})

	// When / Then
	for _, paths := range [][2]string{{"", "file"}, {"dir/file", ""}, {"dir/file", "other/file"}, {"dir/file", "dir/.."}, {"dir/file", `dir\\file`}, {"dir/file", "dir/a\x00"}} {
		if err := client.RenameFile(context.Background(), "hash", paths[0], paths[1]); !errors.Is(err, ErrInvalidPath) {
			t.Fatal("invalid file path was accepted")
		}
	}
}

func TestClient_marksUnknownAndMalformedTorrentResponses_whenLookupResponseIsIncomplete(t *testing.T) {
	for _, body := range []string{
		`{"result":"success","arguments":{"torrents":[]}}`,
		`{"result":"success","arguments":{}}`,
		`{"result":"success","arguments":{"torrents":null}}`,
		`{"result":"success","arguments":{"torrents":[{},{}]}}`,
		`{"result":"success","arguments":{"torrents":[{"name":" "}]}}`,
		`{"result":"success","arguments":{"torrents":[{"name":"root"}]}}`,
		`{"result":"success","arguments":{"torrents":[{"name":"root","files":null}]}}`,
		`{"result":"success","arguments":{"torrents":[{"name":"root","files":[{}]}]}}`,
	} {
		t.Run(body, func(t *testing.T) {
			// Given
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, body)
			}))
			defer upstream.Close()

			// When
			_, err := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password"}}).Files(context.Background(), "hash")

			// Then
			want := ErrMalformedResponse
			if body == `{"result":"success","arguments":{"torrents":[]}}` {
				want = ErrUnknownTorrent
			}
			if !errors.Is(err, want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestClient_rejectsMalformedRenameAcknowledgements_whenResponseDoesNotConfirmRequest(t *testing.T) {
	for _, arguments := range []string{
		`{}`,
		`"invalid"`,
		`{"path":"old-root","name":"new-root"}`,
		`{"path":"wrong","name":"new-root","id":7}`,
		`{"path":"old-root","name":"wrong","id":7}`,
		`{"path":"old-root","name":"new-root","id":7.5}`,
	} {
		t.Run(arguments, func(t *testing.T) {
			// Given
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if decodeReceivedRequest(t, r.Body).Method == "torrent-get" {
					_, _ = io.WriteString(w, `{"result":"success","arguments":{"torrents":[{"name":"old-root","files":[]}]}}`)
					return
				}
				_, _ = io.WriteString(w, `{"result":"success","arguments":`+arguments+`}`)
			}))
			defer upstream.Close()

			// When
			err := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password"}}).Rename(context.Background(), "hash", "new-root")

			// Then
			if !errors.Is(err, ErrMalformedResponse) {
				t.Fatal("malformed rename acknowledgement was accepted")
			}
		})
	}
}

type receivedRequest struct {
	Method    string `json:"method"`
	Arguments struct {
		Path string `json:"path"`
		Name string `json:"name"`
	} `json:"arguments"`
}

func decodeReceivedRequest(t *testing.T, body io.Reader) receivedRequest {
	t.Helper()
	var request receivedRequest
	if err := json.NewDecoder(body).Decode(&request); err != nil {
		t.Fatal(err)
	}
	return request
}

func assertRenameArguments(t *testing.T, request receivedRequest, path, name string) {
	t.Helper()
	if request.Method != "torrent-rename-path" || request.Arguments.Path != path || request.Arguments.Name != name {
		t.Fatal("unexpected torrent rename request")
	}
}
