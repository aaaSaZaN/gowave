package contracts

import "testing"

func TestLookupReturnsAllMostSpecificMatches(t *testing.T) {
	original := Endpoints
	t.Cleanup(func() { Endpoints = original })
	Endpoints = []Endpoint{
		{Key: "generic", Method: "GET", Path: "/api/:resource/:id"},
		{Key: "first", Method: "GET", Path: "/api/users/:id"},
		{Key: "second", Method: "GET", Path: "/api/users/{uuid}"},
		{Key: "less-specific", Method: "GET", Path: "/api/{resource}/{id}"},
		{Key: "literal", Method: "GET", Path: "/api/users/stats"},
	}
	matches, found := Lookup("GET", "/api/users/123")
	if !found || len(matches) != 2 || matches[0].Key != "first" || matches[1].Key != "second" {
		t.Fatalf("parameter matches: %+v, found %v", matches, found)
	}
	matches, found = Lookup("GET", "/api/users/stats")
	if !found || len(matches) != 1 || matches[0].Key != "literal" {
		t.Fatalf("literal match: %+v, found %v", matches, found)
	}
	if matches, found := Lookup("POST", "/api/users/123"); found || len(matches) != 0 {
		t.Fatalf("unexpected matches: %+v", matches)
	}
}
