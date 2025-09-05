package httpx_test

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/bluescreen10/httpx"
)

func TestSimpleForm(t *testing.T) {
	body := bytes.NewReader([]byte("email=ab@c.com&name=test&age=40&missing=1234"))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	type user struct {
		Email    string `form:"email"`
		FullName string `form:"name"`
		Age      int    `form:"age"`
		Dummy    int
	}

	u := user{}

	err := httpx.ParseBody(r, &u)
	if err != nil {
		t.Fatal(err)
	}

	if u.Email != "ab@c.com" || u.FullName != "test" || u.Age != 40 {
		t.Fatal("error parsing form")
	}
}

func TestSimpleJSON(t *testing.T) {
	body := bytes.NewReader([]byte("{ \"email\": \"ab@c.com\", \"name\": \"test\", \"age\":40}"))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/json")

	type user struct {
		Email    string `json:"email"`
		FullName string `json:"name"`
	}

	u := user{}

	err := httpx.ParseBody(r, &u)
	if err != nil {
		t.Fatal(err)
	}

	if u.Email != "ab@c.com" || u.FullName != "test" {
		t.Fatal("error parsing json")
	}
}

func TestSimpleXML(t *testing.T) {
	body := bytes.NewReader([]byte("<user><email>ab@c.com</email><name>test</name></user>"))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/xml")

	type user struct {
		Email    string `xml:"email"`
		FullName string `xml:"name"`
	}

	u := user{}

	err := httpx.ParseBody(r, &u)
	if err != nil {
		t.Fatal(err)
	}

	if u.Email != "ab@c.com" || u.FullName != "test" {
		t.Fatal("error parsing xml")
	}
}
