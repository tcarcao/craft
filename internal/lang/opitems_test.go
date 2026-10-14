package lang

import "testing"

func TestSplitOperationItems(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{"CreateApiKey, CreateUsagePlanKey", []string{"CreateApiKey", "CreateUsagePlanKey"}},
		{"postAdvert, putAdvert, deleteAdvert", []string{"postAdvert", "putAdvert", "deleteAdvert"}},
		{"GET /nearest/paths/{latitude},{longitude}", []string{"GET /nearest/paths/{latitude},{longitude}"}},
		{`ADDR "https://a.b/x, y", SQL flat`, []string{`ADDR "https://a.b/x, y"`, "SQL flat"}},
		{"GET /a/{x, y}", []string{"GET /a/{x, y}"}},
		{"a, , b", []string{"a", "", "b"}},
		{"a, ", []string{"a", ""}},
		{"createSchedule", []string{"createSchedule"}},
	}
	for _, c := range cases {
		var got []string
		for _, it := range SplitOperationItems(c.body) {
			got = append(got, it.Text)
			if it.Text != "" && c.body[it.Start:it.Start+len(it.Text)] != it.Text {
				t.Errorf("%q: item %q has wrong Start %d", c.body, it.Text, it.Start)
			}
		}
		if len(got) != len(c.want) {
			t.Errorf("%q: got %q, want %q", c.body, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: got %q, want %q", c.body, got, c.want)
			}
		}
	}
}

func TestParseOperationItem(t *testing.T) {
	for _, c := range []struct{ in, verb, payload string }{
		{"POST /v1/x", "POST", "/v1/x"},
		{"createSchedule", "", "createSchedule"},
		{"POSTMAN x", "", "POSTMAN x"},
	} {
		v, p := ParseOperationItem(c.in)
		if v != c.verb || p != c.payload {
			t.Errorf("%q: got (%q, %q), want (%q, %q)", c.in, v, p, c.verb, c.payload)
		}
	}
}

func TestParseOperationItem_Egress(t *testing.T) {
	for _, c := range []struct{ in, verb, payload string }{
		{`ADDR "https://a.b/c"`, "ADDR", "https://a.b/c"},
		{"ADDR api.example.com", "ADDR", "api.example.com"},
		{"SQL investment flat", "SQL", "investment flat"},
	} {
		if v, p := ParseOperationItem(c.in); v != c.verb || p != c.payload {
			t.Errorf("%q: got (%q, %q)", c.in, v, p)
		}
	}
}
