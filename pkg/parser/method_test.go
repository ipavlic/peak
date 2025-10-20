package parser

import (
	"testing"
)

func TestFindGenericMethodDefinitions(t *testing.T) {
	input := `
public class SObjectCollection {
    private List<SObject> records;

    public <K> Map<K, List<SObject>> groupBy(String apiFieldName) {
        Map<K, List<SObject>> grouped = new Map<K, List<SObject>>();
        for (SObject rec : this.records) {
            K key = (K) rec.get(apiFieldName);
            if (!grouped.containsKey(key)) {
                grouped.put(key, new List<SObject>());
            }
            grouped.get(key).add(rec);
        }
        return grouped;
    }

    public <K, V> Map<K, V> transform(String keyField, String valueField) {
        Map<K, V> result = new Map<K, V>();
        return result;
    }
}
`

	p := NewParser(input)
	methods, err := p.FindGenericMethodDefinitions("SObjectCollection")

	if err != nil {
		t.Fatalf("Error finding generic methods: %v", err)
	}

	if len(methods) != 2 {
		t.Fatalf("Expected 2 generic methods, got %d", len(methods))
	}

	// Check groupBy method
	groupBy, exists := methods["SObjectCollection.groupBy"]
	if !exists {
		t.Fatal("Expected to find SObjectCollection.groupBy")
	}

	if groupBy.MethodName != "groupBy" {
		t.Errorf("Expected method name 'groupBy', got '%s'", groupBy.MethodName)
	}

	if len(groupBy.TypeParams) != 1 || groupBy.TypeParams[0] != "K" {
		t.Errorf("Expected type params [K], got %v", groupBy.TypeParams)
	}

	// Check transform method
	transform, exists := methods["SObjectCollection.transform"]
	if !exists {
		t.Fatal("Expected to find SObjectCollection.transform")
	}

	if len(transform.TypeParams) != 2 {
		t.Errorf("Expected 2 type params, got %d", len(transform.TypeParams))
	}

	if transform.TypeParams[0] != "K" || transform.TypeParams[1] != "V" {
		t.Errorf("Expected type params [K, V], got %v", transform.TypeParams)
	}
}
