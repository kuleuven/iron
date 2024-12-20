package msg

import (
	"reflect"
	"testing"
)

func TestAdd(t *testing.T) {
	query := QueryRequest{}

	query.Conditions.Add(1, "a")
	query.Conditions.Add(2, "b")
	query.Selects.Add(1, 1)
	query.KeyVals.Add("a", "value")

	expected1 := ISKeyVal{Length: 2, Keys: []int{1, 2}, Values: []string{"a", "b"}}

	msg := "Expected: %v, got: %v"

	if !reflect.DeepEqual(query.Conditions, expected1) {
		t.Errorf(msg, expected1, query.Conditions)
	}

	expected2 := IIKeyVal{Length: 1, Keys: []int{1}, Values: []int{1}}

	if !reflect.DeepEqual(query.Selects, expected2) {
		t.Errorf(msg, expected2, query.Selects)
	}

	expected3 := SSKeyVal{Length: 1, Keys: []string{"a"}, Values: []string{"value"}}

	if !reflect.DeepEqual(query.KeyVals, expected3) {
		t.Errorf(msg, expected3, query.KeyVals)
	}
}
