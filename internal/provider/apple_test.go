package provider

import "testing"

func TestParseApple(t *testing.T) {
	people, err := parseApple([]byte(`[{"name":"Alex","location":"London, England","status":"5 minutes ago"},{"name":"Sam","location":"No location found"}]`))
	if err != nil || len(people) != 2 {
		t.Fatalf("%+v %v", people, err)
	}
	if people[0].LocationText != "London, England" || people[0].Status != "5 minutes ago" || people[0].UpdatedAt != nil || people[1].Status != "location unavailable" || people[1].LocationText != "" {
		t.Fatalf("%+v", people)
	}
	for _, data := range []string{"null", "bad", `{}`, `[{"name":""}]`} {
		if _, err := parseApple([]byte(data)); err == nil {
			t.Errorf("accepted %s", data)
		}
	}
	people, err = parseApple([]byte(`[]`))
	if err != nil || people == nil || len(people) != 0 {
		t.Fatalf("%+v %v", people, err)
	}
}
