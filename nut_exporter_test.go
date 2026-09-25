package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"
)

var (
	binary = "nut_exporter"
)

const (
	address = "localhost:19100"
)

func TestMetricsHandler_ServeHTTP(t *testing.T) {
	// 1. Initialize your handler with any required state or dependencies
	handler := &upsNutMetricsHandler{

		handlers: make(map[string]*http.Handler),
	}

	initLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// 2. Create a mock HTTP request targeting your endpoint
	req := httptest.NewRequest(http.MethodGet, "/metrics?server=nut-lv.bonk.cc", nil)

	// 3. Create a ResponseRecorder to capture the HTTP response
	rr := httptest.NewRecorder()

	// 4. Directly invoke the ServeHTTP method on your handler
	handler.ServeHTTP(rr, req)

	// 5. Assertions: Check if the outcome matches your expectations

	// Check the HTTP status code
	expectedStatus := http.StatusOK
	if status := rr.Code; status != expectedStatus {
		t.Errorf("handler returned wrong status code: got %v want %v", status, expectedStatus)
	}

	// Check the response body
	expectedBody := "expected metrics output here" // Change this to your actual expected output
	if rr.Body.String() != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expectedBody)
	}

	// Optional: Check HTTP response headers if your handler sets them
	// expectedContentType := "text/plain; version=0.0.4"
	// if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
	// 	t.Errorf("handler returned wrong content type: got %v want %v", ctype, expectedContentType)
	// }
}

func TestSuccessfulLaunch(t *testing.T) {
	if _, err := exec.LookPath(binary); err != nil {
		//	if _, err := os.Stat(binary); err != nil {
		t.Error(err)
		return
	}

	exporter := exec.Command(binary, "--web.listen-address", address)
	test := func(pid int) error {
		if err := queryExporter(address); err != nil {
			return err
		}
		return nil
	}

	if err := runCommandAndTests(exporter, address, test); err != nil {
		t.Error(err)
	}
}

func queryExporter(address string) error {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", address))
	if err != nil {
		return err
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	if want, have := http.StatusOK, resp.StatusCode; want != have {
		return fmt.Errorf("want /metrics status code %d, have %d. Body:\n%s", want, have, b)
	}
	return nil
}

func runCommandAndTests(cmd *exec.Cmd, address string, fn func(pid int) error) error {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %s", err)
	}
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 10; i++ {
		if err := queryExporter(address); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
		if cmd.Process == nil || i == 9 {
			return fmt.Errorf("can't start command")
		}
	}

	errc := make(chan error)
	go func(pid int) {
		errc <- fn(pid)
	}(cmd.Process.Pid)

	err := <-errc
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
	return err
}
