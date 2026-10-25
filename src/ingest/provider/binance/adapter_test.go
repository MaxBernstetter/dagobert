package binance

import (
	"errors"
	"net/http"
	"testing"
)

func TestRetryable(t *testing.T) {
	a := &Adapter{}
	cases := []struct {
		status int
		err    error
		want   bool
	}{
		{http.StatusTooManyRequests, errors.New("429"), true},
		{http.StatusTeapot, errors.New("418"), true},
		{http.StatusInternalServerError, errors.New("500"), true},
		{http.StatusBadGateway, errors.New("502"), true},
		{http.StatusBadRequest, errors.New("400"), false},
		{http.StatusNotFound, errors.New("404"), false},
		{0, errors.New("network"), true},
		{http.StatusOK, errors.New("parse"), true},
		{http.StatusOK, nil, false},
	}
	for _, tc := range cases {
		got := a.Retryable(tc.status, tc.err)
		if got != tc.want {
			t.Fatalf("Retryable(%d, %v)=%v want %v", tc.status, tc.err, got, tc.want)
		}
	}
}

func TestParseKlines(t *testing.T) {
	body := []byte(`[[1000,"1","2","0.5","1.5","10",1999]]`)
	rows, err := parseKlines(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len=%d", len(rows))
	}
	c := rows[0]
	if c.OpenTime != 1000 || c.CloseTime != 1999 || c.Open != "1" || c.High != "2" || c.Low != "0.5" || c.Close != "1.5" || c.Volume != "10" {
		t.Fatalf("%+v", c)
	}
}

func TestNew_requiresBaseURL(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_optionsBaseURLOnly(t *testing.T) {
	a, err := New(Options{BaseURL: "https://data-api.binance.vision"})
	if err != nil {
		t.Fatal(err)
	}
	if a.MaxPageHours() != 1000 || a.RequestWeight() != 2 {
		t.Fatalf("policy hours=%d weight=%d", a.MaxPageHours(), a.RequestWeight())
	}
	if a.ID() != "binance" {
		t.Fatalf("id=%s", a.ID())
	}
}
