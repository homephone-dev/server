//go:build e2e

// Package e2e_test brings up docker-compose.yml + docker-compose.e2e.yml,
// seeds fixtures via the REST API, drives the pjsua containers, and asserts
// call outcomes via GET /devices/{id}/calls. Run via:
//
//	docker compose -f docker-compose.yml -f docker-compose.e2e.yml up -d --build
//	go test -tags e2e ./test/e2e/...
//	docker compose -f docker-compose.yml -f docker-compose.e2e.yml down -v
//
// See docs/testing.md for the current state of this harness and known gaps.
package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

const baseURL = "http://localhost:8080/api/v1"

// Container names as assigned by `docker compose -p homephone-dev-server
// -f docker-compose.yml -f docker-compose.e2e.yml up`: the default project
// name is the compose-file directory's basename. If you run the stack
// under a different project name (COMPOSE_PROJECT_NAME or -p), override
// these via the SIP_DEVICE_CONTAINER / SIP_TRUNK_CONTAINER env vars.
func sipDeviceContainer() string {
	if v := os.Getenv("SIP_DEVICE_CONTAINER"); v != "" {
		return v
	}
	return "homephone-dev-server-sip-device-1"
}

func sipTrunkContainer() string {
	if v := os.Getenv("SIP_TRUNK_CONTAINER"); v != "" {
		return v
	}
	return "homephone-dev-server-sip-trunk-mock-1"
}

// asteriskHost is the SIP signaling target the SIPp UAC containers dial;
// they reach Asterisk over the shared compose network by service name.
const asteriskHost = "asterisk"

func apiToken(t *testing.T) string {
	t.Helper()
	tok := os.Getenv("API_TOKEN")
	if tok == "" {
		t.Fatal("API_TOKEN env var must be set to run e2e tests")
	}
	return tok
}

func apiRequest(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken(t))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func waitForAPI(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp := apiRequest(t, http.MethodGet, "/devices", nil)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("backend API did not become ready in time")
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func seedDevice(t *testing.T) string {
	t.Helper()
	resp := apiRequest(t, http.MethodPost, "/devices", map[string]string{
		"name": "e2e-device", "sipEndpointId": "device-1", "timezone": "UTC",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed device: status %d", resp.StatusCode)
	}
	var d struct{ ID string }
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatal(err)
	}
	return d.ID
}

func TestSmoke_APIReachable(t *testing.T) {
	waitForAPI(t)
}

// seedDeviceNamed seeds a device with a unique name (per-test, so scenario
// tests run in the same package don't collide) but a fixed sipEndpointId --
// there's only one "device-1" endpoint configured in asterisk/etc/asterisk/
// pjsip.conf, so every scenario test shares it and must clean up its own
// contacts/call-log rows before/after running.
func seedDeviceNamed(t *testing.T, name string) string {
	t.Helper()
	resp := apiRequest(t, http.MethodPost, "/devices", map[string]string{
		"name": name, "sipEndpointId": "device-1", "timezone": "UTC",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed device: status %d", resp.StatusCode)
	}
	var d struct{ ID string }
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatal(err)
	}
	return d.ID
}

// findDeviceBySIPEndpoint returns the id of an existing device row for the
// given sipEndpointId, if one exists (devices aren't deleted between test
// runs, and only one device may exist per SIP endpoint at a time in the
// backend's routing logic, so scenario tests reuse a single shared device
// row rather than creating a fresh one -- see deviceForEndpoint).
func findDeviceBySIPEndpoint(t *testing.T, sipEndpointID string) (id string, ok bool) {
	t.Helper()
	resp := apiRequest(t, http.MethodGet, "/devices", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list devices: status %d", resp.StatusCode)
	}
	var devices []struct {
		ID            string `json:"id"`
		SIPEndpointID string `json:"sipEndpointId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatal(err)
	}
	for _, d := range devices {
		if d.SIPEndpointID == sipEndpointID {
			return d.ID, true
		}
	}
	return "", false
}

// deviceForEndpoint returns the device id for sipEndpointId "device-1",
// creating it if it doesn't already exist. Scenario tests share this one
// device row (matching the one SIP endpoint configured in
// asterisk/etc/asterisk/pjsip.conf) and are responsible for cleaning up
// their own contacts/call-log rows.
func deviceForEndpoint(t *testing.T, sipEndpointID string) string {
	t.Helper()
	if id, ok := findDeviceBySIPEndpoint(t, sipEndpointID); ok {
		return id
	}
	return seedDeviceNamed(t, "e2e-"+sipEndpointID)
}

// addContact adds an allowlist contact (a number allowed to reach/be
// reached by this device) and returns its id.
func addContact(t *testing.T, deviceID, number string) string {
	t.Helper()
	resp := apiRequest(t, http.MethodPost, "/devices/"+deviceID+"/contacts", map[string]string{
		"label": "e2e", "number": number,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add contact: status %d", resp.StatusCode)
	}
	var c struct{ ID string }
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func deleteContact(t *testing.T, contactID string) {
	t.Helper()
	resp := apiRequest(t, http.MethodDelete, "/contacts/"+contactID, nil)
	resp.Body.Close()
}

// purgeCallsForNumber deletes any prior call-log rows for this device/
// number pair, so a fresh test run's polling doesn't match a stale
// leftover row from an earlier run.
func purgeCallsForNumber(t *testing.T, deviceID, number string) {
	t.Helper()
	resp := apiRequest(t, http.MethodDelete, "/devices/"+deviceID+"/calls?number="+number, nil)
	resp.Body.Close()
}

type callRecord struct {
	ID           string  `json:"id"`
	Direction    string  `json:"direction"`
	RemoteNumber string  `json:"remoteNumber"`
	Outcome      string  `json:"outcome"`
	Reason       *string `json:"reason"`
}

// waitForCallOutcome polls GET /devices/{id}/calls until a call log row
// for remoteNumber reaches a terminal outcome ("connected" or "blocked"),
// or the deadline passes. It returns the matching row.
func waitForCallOutcome(t *testing.T, deviceID, remoteNumber string) callRecord {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp := apiRequest(t, http.MethodGet, "/devices/"+deviceID+"/calls", nil)
		var calls []callRecord
		if err := json.NewDecoder(resp.Body).Decode(&calls); err != nil {
			resp.Body.Close()
			t.Fatal(err)
		}
		resp.Body.Close()
		for _, c := range calls {
			if c.RemoteNumber == remoteNumber && (c.Outcome == "connected" || c.Outcome == "blocked") {
				return c
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("no terminal call log outcome for remoteNumber %q within deadline", remoteNumber)
	return callRecord{}
}
